/** @typedef {'ok' | 'warn' | 'danger'} Tone */

/**
 * UI lifecycle key for one concrete account record.
 * Prefer the database id so deleting and re-adding the same profile does not
 * inherit stale selection/refreshing state from the deleted record.
 * @param {{ id?: string; profile_name?: string; nickname?: string }} account
 */
export function accountRecordKey(account) {
  return account.id || account.profile_name || account.nickname || '';
}

/**
 * Usage records are keyed by backend account identity, currently profile_name
 * with nickname fallback. Keep this separate from the UI record key.
 * @param {{ id?: string; profile_name?: string; nickname?: string }} account
 */
export function accountUsageKey(account) {
  return account.profile_name || account.nickname || account.id || '';
}

/**
 * @param {number} value
 */
export function clampPercent(value) {
  return Math.max(0, Math.min(100, Math.round(Number(value) || 0)));
}

/**
 * @param {number} remaining5h
 * @param {number} remainingWeekly
 * @returns {Tone}
 */
export function toneFor(remaining5h, remainingWeekly) {
  const lowestRemaining = Math.min(clampPercent(remaining5h), clampPercent(remainingWeekly));
  if (lowestRemaining === 0) return 'danger';
  if (lowestRemaining <= 20) return 'warn';
  return 'ok';
}

/**
 * @param {Tone} tone
 */
export function labelFor(tone) {
  if (tone === 'danger') return '冷却中';
  if (tone === 'warn') return '接近用尽';
  return '可用';
}

/**
 * @param {Array<{ tone: Tone }>} dashboardAccounts
 */
export function availableCount(dashboardAccounts) {
  return dashboardAccounts.filter((item) => item.tone === 'ok').length;
}

/**
 * @param {Array<{ tone: Tone }>} dashboardAccounts
 */
export function suggestedSwitchCount(dashboardAccounts) {
  return dashboardAccounts.filter((item) => item.tone !== 'ok').length;
}

/**
 * @param {Set<string>} selectedKeys
 * @param {Array<{ id?: string; profile_name?: string; nickname?: string }>} accounts
 */
export function pruneSelection(selectedKeys, accounts) {
  const valid = new Set(accounts.map(accountRecordKey));
  return new Set([...selectedKeys].filter((key) => valid.has(key)));
}
