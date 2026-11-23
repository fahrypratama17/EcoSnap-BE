'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const test = require('node:test');

const cli = path.join(__dirname, 'check-postman-scripts.cjs');

function fixture(t, collection) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'postman-script-gate-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const directory = path.join(root, 'docs', 'api');
  fs.mkdirSync(directory, { recursive: true });
  if (collection !== undefined) {
    fs.writeFileSync(path.join(directory, 'ecosnap.postman_collection.json'), collection);
  }
  return root;
}

function run(root) {
  return spawnSync(process.execPath, [cli, '--root', root], {
    encoding: 'utf8',
    env: { PATH: process.env.PATH, SystemRoot: process.env.SystemRoot },
  });
}

test('validates collection, folder, and request scripts including string exec and return', (t) => {
  const root = fixture(t, JSON.stringify({
    event: [{ listen: 'prerequest', script: { exec: ['const a = 1;', 'pm.variables.set("a", a); return;'] } }],
    item: [{ name: 'Folder', event: [{ listen: 'test', script: { type: 'text/javascript', exec: 'pm.test("ok", () => {});' } }],
      item: [{ name: 'Request', event: [{ listen: 'test', script: { exec: 'pm.response.to.have.status(200);' } }] }] }],
  }));
  const result = run(root);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /3 syntax checks passed/);
});

test('rejects malformed nested script without exposing source text', (t) => {
  const secret = 'doNotEchoSensitiveValue';
  const root = fixture(t, JSON.stringify({ item: [{ name: 'Folder', item: [{ name: 'Broken request', event: [
    { listen: 'test', script: { exec: `const ${secret} = 1; const value = ;` } },
  ] }] }] }));
  const result = run(root);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Folder\/Broken request \(test\): syntax error/);
  assert.doesNotMatch(result.stderr, new RegExp(secret));
});

test('rejects missing collection file', (t) => {
  const result = run(fixture(t));
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /collection file is missing or unreadable/);
});

test('rejects malformed collection JSON without echoing its contents', (t) => {
  const secret = 'doNotEchoJsonToken';
  const result = run(fixture(t, `{ "token": "${secret}", broken }`));
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /invalid JSON/);
  assert.doesNotMatch(result.stderr, new RegExp(secret));
});
