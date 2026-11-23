'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { runChecked } = require('./run-full-flow.cjs');

async function withServer(handler, callback) {
  const server = http.createServer(handler);
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const { port } = server.address();
    await callback(`http://127.0.0.1:${port}`);
  } finally {
    await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
}

function collection(testScript) {
  return {
    info: { name: 'runner test', schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json' },
    item: [{ name: 'probe', request: { method: 'GET', url: '{{base_url}}/probe' }, event: [{ listen: 'test', script: { type: 'text/javascript', exec: testScript } }] }],
    variable: []
  };
}

const environment = { base_url: '', user_token: 'ci-user', mitra_token: 'ci-mitra' };

test('accepts a Newman request whose assertion passes', async () => {
  await withServer((_request, response) => { response.writeHead(200); response.end('ok'); }, async baseUrl => {
    const result = await runChecked(collection(["pm.test('status is 200', () => pm.response.to.have.status(200));"]), { baseUrl, environment, phase: 'pass' });
    assert.equal(result.requests, 1);
    assert.equal(result.assertions, 1);
  });
});
test('rejects a response with no assertion', async () => {
  let requests = 0;
  await withServer((_request, response) => { requests++; response.writeHead(200); response.end('ok'); }, async baseUrl => {
    const unchecked = collection([]);
    unchecked.item[0].event = [];
    await assert.rejects(runChecked(unchecked, { baseUrl, environment, phase: 'unchecked' }), error => {
      assert.equal(error.kind, 'unchecked');
      return true;
    });
    assert.equal(requests, 1);
  });
});


test('rejects an assertion failure without exposing assertion values', async () => {
  let requests = 0;
  await withServer((_request, response) => { requests++; response.writeHead(200); response.end('private-response'); }, async baseUrl => {
    await assert.rejects(runChecked(collection(["pm.test('private-assertion', () => pm.response.to.have.status(201));"]), { baseUrl, environment, phase: 'actual-assertion' }), error => {
      assert.equal(error.kind, 'assertion');
      assert.doesNotMatch(error.message, /private-response|private-assertion/);
      return true;
    });
    assert.equal(requests, 1);
  });
});

test('rejects silently skipped requests with no HTTP request', async () => {
  let requests = 0;
  await withServer((_request, response) => { requests++; response.end('ok'); }, async baseUrl => {
    const skipped = collection([]);
    skipped.item[0].event = [{ listen: 'prerequest', script: { type: 'text/javascript', exec: ['pm.execution.skipRequest();'] } }];
    await assert.rejects(runChecked(skipped, { baseUrl, environment, phase: 'skipped' }), error => {
      assert.equal(error.message, 'skipped: incomplete run: request count did not match plan');
      return true;
    });
    assert.equal(requests, 0);
  });
});

test('rejects request-script exceptions without exposing their message', async () => {
  await withServer((_request, response) => { response.end('ok'); }, async baseUrl => {
    await assert.rejects(runChecked(collection(['throw new Error("private script error");']), { baseUrl, environment, phase: 'safe-script-phase' }), error => {
      assert.equal(error.kind, 'script');
      assert.doesNotMatch(error.message, /private script error/);
      return true;
    });
  });
});

test('rejects a non-loopback target before opening a connection', async () => {
  let connected = false;
  const server = http.createServer((_request, response) => { connected = true; response.end('ok'); });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  try {
    const { port } = server.address();
    await assert.rejects(runChecked(collection(["pm.test('ok', () => pm.response.to.have.status(200));"]), { baseUrl: `http://192.0.2.1:${port}`, environment, phase: 'target' }), /loopback/);
    assert.equal(connected, false);
  } finally {
    await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
});
test('CLI exits nonzero with a controlled assertion failure after the real HTTP response', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'ecosnap-newman-cli-'));
  let requests = 0;
  try {
    await fs.mkdir(path.join(root, 'docs', 'api'), { recursive: true });
    await fs.copyFile(
      path.resolve(__dirname, '../../docs/api/ecosnap.postman_collection.json'),
      path.join(root, 'docs', 'api', 'ecosnap.postman_collection.json')
    );
    await withServer((_request, response) => {
      requests++;
      response.writeHead(418, { 'Content-Type': 'application/json' });
      response.end('private-cli-response-body');
    }, async baseUrl => {
      const child = spawn(process.execPath, [
        path.join(__dirname, 'run-full-flow.cjs'),
        '--base-url', baseUrl,
        '--root', root
      ], { stdio: ['ignore', 'pipe', 'pipe'] });
      let stdout = '';
      let stderr = '';
      child.stdout.setEncoding('utf8').on('data', chunk => { stdout += chunk; });
      child.stderr.setEncoding('utf8').on('data', chunk => { stderr += chunk; });
      const exit = await new Promise((resolve, reject) => {
        child.once('error', reject);
        child.once('close', (code, signal) => resolve({ code, signal }));
      });
      assert.equal(exit.code, 1);
      assert.equal(exit.signal, null);
      assert.match(stderr, /NEWMAN_FULL_FLOW_FAIL phase=registration category=assertion/);
      assert.doesNotMatch(`${stdout}${stderr}`, /private-cli-response-body/);
      assert.equal(requests, 1);
    });
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});
