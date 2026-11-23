#!/usr/bin/env node
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function parseArgs(argv) {
  let root = path.resolve(__dirname, '..');
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--root' && i + 1 < argv.length) {
      root = path.resolve(argv[++i]);
    } else {
      throw new Error(`unknown or incomplete argument: ${argv[i]}`);
    }
  }
  return root;
}

function scriptSource(exec) {
  if (typeof exec === 'string') return exec;
  if (Array.isArray(exec) && exec.every((line) => typeof line === 'string')) {
    return exec.join('\n');
  }
  throw new Error('unsupported script exec format (expected string or string array)');
}

function checkCollection(root) {
  const file = path.join(root, 'docs', 'api', 'ecosnap.postman_collection.json');
  let raw;
  try {
    raw = fs.readFileSync(file, 'utf8');
  } catch {
    throw new Error('docs/api/ecosnap.postman_collection.json: collection file is missing or unreadable');
  }
  if (!raw.trim()) throw new Error('docs/api/ecosnap.postman_collection.json: collection file is empty');

  let collection;
  try {
    collection = JSON.parse(raw);
  } catch {
    throw new Error('docs/api/ecosnap.postman_collection.json: invalid JSON');
  }
  if (!collection || typeof collection !== 'object' || Array.isArray(collection)) {
    throw new Error('docs/api/ecosnap.postman_collection.json: expected a collection object');
  }

  let scriptCount = 0;
  function visitEvents(events, context) {
    if (events === undefined) return;
    if (!Array.isArray(events)) throw new Error(`${context}: event must be an array`);
    for (let index = 0; index < events.length; index += 1) {
      const event = events[index];
      const type = event && typeof event.listen === 'string' ? event.listen : `event[${index}]`;
      const label = `${context} (${type})`;
      if (!event || typeof event !== 'object' || !event.script || typeof event.script !== 'object') {
        throw new Error(`${label}: missing script object`);
      }
      const script = event.script;
      if (script.type !== undefined && script.type !== 'text/javascript') {
        throw new Error(`${label}: unsupported script type '${script.type}'`);
      }
      let source;
      try {
        source = scriptSource(script.exec);
      } catch (error) {
        throw new Error(`${label}: ${error.message}`);
      }
      try {
        new Function(source);
      } catch {
        throw new Error(`${label}: syntax error`);
      }
      scriptCount += 1;
    }
  }

  function visitItems(items, parent) {
    if (items === undefined) return;
    if (!Array.isArray(items)) throw new Error(`${parent}: item must be an array`);
    for (let index = 0; index < items.length; index += 1) {
      const item = items[index];
      if (!item || typeof item !== 'object') throw new Error(`${parent}/item[${index}]: expected an item object`);
      const name = typeof item.name === 'string' && item.name.length ? item.name : `item[${index}]`;
      const context = `${parent}/${name}`;
      visitEvents(item.event, context);
      visitItems(item.item, context);
    }
  }

  visitEvents(collection.event, 'collection');
  visitItems(collection.item, 'collection');
  if (scriptCount === 0) throw new Error('docs/api/ecosnap.postman_collection.json: no event scripts found');
  return scriptCount;
}

try {
  const count = checkCollection(parseArgs(process.argv.slice(2)));
  console.log(`Postman scripts: ${count} syntax checks passed`);
} catch (error) {
  console.error(`Postman script validation failed: ${error.message}`);
  process.exitCode = 1;
}
