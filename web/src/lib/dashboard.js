/** @typedef {'ok' | 'warn' | 'danger'} Tone */
/** @typedef {'available' | 'credit_available' | 'limited' | 'blocked' | 'unknown'} Availability */

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
 * @param {Availability | string | undefined} availability
 * @param {number} remainingWeekly
 * @param {boolean} hasWeeklyWindow
 * @returns {Tone}
 */
export function toneFor(availability, remainingWeekly, hasWeeklyWindow = false) {
  if (availability === 'blocked' || availability === 'limited') return 'danger';
  if (availability === 'credit_available' || availability === 'unknown') return 'warn';
  if (!hasWeeklyWindow) return 'warn';
  const remaining = clampPercent(remainingWeekly);
  if (remaining === 0) return 'danger';
  if (remaining <= 20) return 'warn';
  return 'ok';
}

/**
 * @param {Availability | string | undefined} availability
 * @param {Tone} tone
 */
export function labelFor(availability, tone) {
  if (availability === 'blocked') return '已阻止';
  if (availability === 'limited') return '额度受限';
  if (availability === 'credit_available') return '余额可用';
  if (availability === 'unknown') return '待同步';
  if (tone === 'warn') return '接近用尽';
  return '可用';
}

/**
 * @param {Array<{ availability?: Availability | string }>} dashboardAccounts
 */
export function availableCount(dashboardAccounts) {
  return dashboardAccounts.filter((item) => item.availability === 'available' || item.availability === 'credit_available').length;
}

/**
 * @param {Array<{ availability?: Availability | string }>} dashboardAccounts
 */
export function suggestedSwitchCount(dashboardAccounts) {
  return dashboardAccounts.filter((item) => item.availability === 'limited' || item.availability === 'blocked' || item.availability === 'unknown').length;
}

/**
 * @param {Set<string>} selectedKeys
 * @param {Array<{ id?: string; profile_name?: string; nickname?: string }>} accounts
 */
export function pruneSelection(selectedKeys, accounts) {
  const valid = new Set(accounts.map(accountRecordKey));
  return new Set([...selectedKeys].filter((key) => valid.has(key)));
}
