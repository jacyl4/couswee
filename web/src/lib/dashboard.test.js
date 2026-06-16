import assert from 'node:assert/strict';
import test from 'node:test';

import {
  accountRecordKey,
  accountUsageKey,
  availableCount,
  pruneSelection,
  suggestedSwitchCount,
  toneFor
} from './dashboard.js';

test('toneFor treats zero weekly remaining as cooldown danger', () => {
  assert.equal(toneFor(100, 0), 'danger');
  assert.equal(toneFor(0, 100), 'danger');
  assert.equal(toneFor(45, 50), 'ok');
  assert.equal(toneFor(21, 90), 'ok');
  assert.equal(toneFor(20, 90), 'warn');
  assert.equal(toneFor(1, 90), 'warn');
  assert.equal(toneFor(9, 90), 'warn');
});

test('summary counts derive from dashboard tone', () => {
  const accounts = [{ tone: 'ok' }, { tone: 'warn' }, { tone: 'danger' }];
  assert.equal(availableCount(accounts), 1);
  assert.equal(suggestedSwitchCount(accounts), 2);
});

test('record key uses id while usage key preserves profile identity', () => {
  const account = { id: 'acc-new', profile_name: 'dev-main', nickname: 'Dev' };
  assert.equal(accountRecordKey(account), 'acc-new');
  assert.equal(accountUsageKey(account), 'dev-main');
});

test('pruneSelection does not carry deleted profile selection to re-added record', () => {
  const selected = new Set(['acc-old']);
  const readded = [{ id: 'acc-new', profile_name: 'dev-main', nickname: 'Dev' }];
  assert.deepEqual([...pruneSelection(selected, readded)], []);
});
