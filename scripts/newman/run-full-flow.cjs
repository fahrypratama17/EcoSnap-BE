'use strict';

const fs = require('node:fs');
const path = require('node:path');
const newman = require('newman');

const REGISTRATION = '01-register (sekali per akun Firebase)';
const HELPERS = '90 Helpers (manual, jangan di bulk run)';
const PAGINATION = '12 Pagination cursor (butuh data >= 2 baris)';
const REGISTRATION_TOKENS = { user_token: 'ci-register-user', mitra_token: 'ci-register-mitra' };
const MAIN_TOKENS = { user_token: 'ci-user', mitra_token: 'ci-mitra' };
const REQUEST_TIMEOUT_MS = 15000;
const SCRIPT_TIMEOUT_MS = 10000;

function assertLoopbackUrl(value) {
  let url;
  try { url = new URL(value); } catch { throw new Error('base URL must be a valid loopback HTTP URL'); }
  const hostname = url.hostname.toLowerCase().replace(/^\[|\]$/g, '');
  const isLoopback = hostname === 'localhost' || hostname === '::1' || /^127(?:\.\d{1,3}){3}$/.test(hostname);
  if (url.protocol !== 'http:' || !isLoopback || url.username || url.password) {
    throw new Error('base URL must target loopback over HTTP');
  }
  return url.toString().replace(/\/$/, '');
}

function countRequests(items) {
  let count = 0;
  for (const item of items || []) {
    if (item.request) count++;
    if (item.item) count += countRequests(item.item);
  }
  return count;
}

function requireNamedFolder(items, name) {
  const matches = (items || []).filter(item => item.name === name && Array.isArray(item.item));
  if (matches.length !== 1) throw new Error(`collection must contain exactly one folder named "${name}"`);
  return matches[0];
}

function cloneCollection(collection, items) {
  return { ...structuredClone(collection), item: structuredClone(items) };
}

function planPhases(collection) {
  const top = collection.item || [];
  const account = requireNamedFolder(top, '01 Akun & Profil');
  const registration = requireNamedFolder(account.item, REGISTRATION);
  const helpers = requireNamedFolder(top, HELPERS);
  const pagination = requireNamedFolder(top, PAGINATION);
  const registrationRequests = countRequests(registration.item);
  if (registrationRequests < 1) throw new Error('registration folder contains no requests');
  const mainItems = top.filter(item => item !== helpers && item !== pagination).map(item => {
    if (item !== account) return structuredClone(item);
    return { ...structuredClone(item), item: structuredClone(item.item.filter(child => child !== registration)) };
  }).filter(item => item !== account || item.item.length > 0);
  const phase2Requests = countRequests(mainItems);
  const phase3Items = [...mainItems, structuredClone(pagination)];
  const phase3Requests = countRequests(phase3Items);
  if (phase2Requests < 1 || phase3Requests <= phase2Requests) throw new Error('collection main phases have unexpected request counts');
  return [
    { name: 'registration', collection: cloneCollection(collection, [structuredClone(registration)]), tokens: REGISTRATION_TOKENS, planned: registrationRequests },
    { name: 'main', collection: cloneCollection(collection, mainItems), tokens: MAIN_TOKENS, planned: phase2Requests },
    { name: 'main-fresh-copy-with-pagination', collection: cloneCollection(collection, phase3Items), tokens: MAIN_TOKENS, planned: phase3Requests }
  ];
}

function runChecked(collection, { baseUrl, environment, phase }) {
  let safeBaseUrl;
  try { safeBaseUrl = assertLoopbackUrl(baseUrl); }
  catch (error) { return Promise.reject(error); }
  const planned = countRequests(collection.item);
  if (!planned) return Promise.reject(new Error(`${phase}: incomplete plan has no request leaves`));
  let observed = 0;
  let requestFailures = false;
  let missingResponse = false;
  let exceptionSeen = false;
  return new Promise((resolve, reject) => {
    const fail = (message, kind = 'execution') => {
      const error = new Error(`${phase}: ${message}`);
      error.kind = kind;
      error.phase = phase;
      reject(error);
    };
    try {
      const runner = newman.run({
        collection,
        environment: { values: Object.entries({ ...environment, base_url: safeBaseUrl }).map(([key, value]) => ({ key, value, enabled: true })) },
        reporters: [],
        timeoutRequest: REQUEST_TIMEOUT_MS,
        timeoutScript: SCRIPT_TIMEOUT_MS,
        ignoreRedirects: true,
        bail: true
      }, (error, summary) => {
        if (error || !summary || !summary.run) return fail('Newman execution failed');
        const run = summary.run;
        const requestStats = run.stats && run.stats.requests || {};
        const assertionStats = run.stats && run.stats.assertions || {};
        const actual = requestStats.total || 0;
        const assertionTotal = assertionStats.total || 0;
        if (requestStats.failed > 0 || requestFailures) return fail('request failure', 'request');
        if (assertionStats.failed > 0) return fail('assertion failure', 'assertion');
        if (exceptionSeen) return fail('script or runner exception', 'script');
        if (missingResponse) return fail('request completed without a response', 'request');
        if (run.failures && run.failures.length) return fail('execution failure', 'execution');
        if (actual !== planned || observed !== planned) return fail('incomplete run: request count did not match plan');
        const unchecked = (run.executions || []).some(execution =>
          execution.response && !(execution.assertions || []).some(assertion => !assertion.skipped)
        );
        if (unchecked) return fail('response had no non-skipped assertion', 'unchecked');
        resolve({ requests: actual, assertions: assertionTotal });
      });
      runner.on('request', (error, args) => {
        observed++;
        if (error) requestFailures = true;
        if (!args || !args.response) missingResponse = true;
      });
      runner.on('exception', () => { exceptionSeen = true; });
    } catch {
      fail('Newman execution failed');
    }
  });
}

function parseArgs(argv) {
  let baseUrl;
  let root = path.resolve(__dirname, '../..');
  for (let index = 0; index < argv.length; index++) {
    const arg = argv[index];
    if (arg === '--base-url' && argv[index + 1]) baseUrl = argv[++index];
    else if (arg === '--root' && argv[index + 1]) root = path.resolve(argv[++index]);
    else throw new Error('usage: node scripts/newman/run-full-flow.cjs --base-url http://127.0.0.1:<port> [--root repo]');
  }
  if (!baseUrl) throw new Error('usage: node scripts/newman/run-full-flow.cjs --base-url http://127.0.0.1:<port> [--root repo]');
  return { baseUrl: assertLoopbackUrl(baseUrl), root };
}

async function main(argv = process.argv.slice(2)) {
  const { baseUrl, root } = parseArgs(argv);
  const collectionPath = path.join(root, 'docs', 'api', 'ecosnap.postman_collection.json');
  let source;
  try { source = JSON.parse(fs.readFileSync(collectionPath, 'utf8')); }
  catch { throw new Error('unable to read Postman collection'); }
  const phases = planPhases(source);
  const totals = [];
  for (const phase of phases) {
    const result = await runChecked(phase.collection, { baseUrl, environment: phase.tokens, phase: phase.name });
    totals.push({ name: phase.name, ...result });
    process.stdout.write(`NEWMAN_PHASE_PASS ${phase.name} requests=${result.requests} assertions=${result.assertions}\n`);
  }
  const requests = totals.reduce((sum, phase) => sum + phase.requests, 0);
  const assertions = totals.reduce((sum, phase) => sum + phase.assertions, 0);
  process.stdout.write(`NEWMAN_FULL_FLOW_PASS phases=${totals.length} requests=${requests} assertions=${assertions}\n`);
  return totals;
}

if (require.main === module) {
  main().catch(error => {
    const message = error && /^usage:/.test(error.message)
      ? error.message
      : `NEWMAN_FULL_FLOW_FAIL phase=${error && error.phase || 'setup'} category=${error && error.kind || 'setup'}`;
    process.stderr.write(`${message}\n`);
    process.exitCode = 1;
  });
}

module.exports = { runChecked, assertLoopbackUrl, planPhases };
