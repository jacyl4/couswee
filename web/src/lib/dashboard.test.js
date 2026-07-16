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

test('toneFor prioritizes backend availability and only falls back to the weekly window', () => {
  assert.equal(toneFor('blocked', 90, true), 'danger');
  assert.equal(toneFor('limited', 90, true), 'danger');
  assert.equal(toneFor('credit_available', 0, true), 'warn');
  assert.equal(toneFor('unknown', 90, false), 'warn');
  assert.equal(toneFor('available', 0, true), 'danger');
  assert.equal(toneFor('available', 20, true), 'warn');
  assert.equal(toneFor('available', 21, true), 'ok');
});

test('summary counts derive from backend availability', () => {
  const accounts = [{ availability: 'available' }, { availability: 'credit_available' }, { availability: 'limited' }, { availability: 'unknown' }];
  assert.equal(availableCount(accounts), 2);
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
