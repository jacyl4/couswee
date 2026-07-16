<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import {
    accountRecordKey,
    accountUsageKey,
    availableCount as countAvailable,
    clampPercent,
    labelFor,
    pruneSelection as pruneSelectedKeys,
    suggestedSwitchCount as countSuggestedSwitch,
    toneFor
  } from '$lib/dashboard.js';

  type Account = {
    id?: string;
    nickname: string;
    profile_name?: string;
    auth_path: string;
    login_method?: string;
    status?: string;
    subscription?: string;
    weekly_usage?: number;
    has_weekly_window?: boolean;
    availability?: Availability;
    plan_type?: string;
    rate_limit_allowed?: boolean;
    rate_limit_reached_type?: string;
    credits_available?: boolean;
    credits_unlimited?: boolean;
    credits_balance?: string;
    credits_approx_local_messages?: number;
    credits_approx_cloud_messages?: number;
    credits_overage_limit_reached?: boolean;
    spend_control_reached?: boolean;
    usage_source?: string;
    usage_last_refresh?: string;
    usage_stale?: boolean;
    usage_error?: string;
    auth_status?: string;
    auth_expired?: boolean;
    auth_expires_at?: string;
    auth_last_refresh?: string;
    auth_error?: string;
    active?: boolean;
    last_used_at?: string;
    last_switch_at?: string;
    last_switch?: string;
  };

  type LoginSession = {
    id: string;
    method: 'device' | string;
    status: string;
    authorization_url?: string;
    verification_url?: string;
    user_code?: string;
    device_code?: string;
    expires_at?: string;
    error?: string;
  };

  type UsageRecord = {
    account: string;
    weekly_usage?: number;
    weekly_remaining?: number;
    has_weekly_window?: boolean;
    availability?: Availability;
    plan_type?: string;
    rate_limit_allowed?: boolean;
    rate_limit_reached_type?: string;
    credits_available?: boolean;
    credits_unlimited?: boolean;
    credits_balance?: string;
    credits_approx_local_messages?: number;
    credits_approx_cloud_messages?: number;
    credits_overage_limit_reached?: boolean;
    spend_control_reached?: boolean;
    usage_basis?: string;
    unit?: string;
    source?: string;
    last_refresh?: string;
    stale?: boolean;
    error?: string;
  };

  type Tone = 'ok' | 'warn' | 'danger';
  type Availability = 'available' | 'credit_available' | 'limited' | 'blocked' | 'unknown';
  type RefreshScope = {
    accounts?: boolean;
    usage?: boolean;
  };

  type DashboardAccount = {
    key: string;
    account: Account;
    usage?: UsageRecord;
    remainingWeekly: number;
    hasWeeklyWindow: boolean;
    availability: Availability;
    tone: Tone;
    statusLabel: string;
    entitlementSummary: string;
    loginStatusLabel: string;
    authExpired: boolean;
    authStatusLabel: string;
    authStatusDetail: string;
    selected: boolean;
    refreshingUsage: boolean;
    usageStale: boolean;
    usageError: string;
  };

  const API = {
    accounts: '/api/accounts',
    switch: '/api/switch',
    usage: '/api/codex/usage',
    loginStart: '/api/codex/login/start',
    loginStatus: (id: string) => `/api/codex/login/${id}`,
    loginCancel: (id: string) => `/api/codex/login/${id}/cancel`,
    account: (id: string) => `/api/accounts/${id}`
  } as const;

  let accounts: Account[] = [];
  let usageRecords: UsageRecord[] = [];
  let loadingAccounts = true;
  let loadingUsage = true;
  let errorMessage = '';
  let noticeMessage = '';
  let switchingSelector = '';
  let refreshingUsageKeys = new Set<string>();
  let addingAccount = false;
  let manualImportOpen = false;
  let addForm = { nickname: '', profile_name: '', auth_path: '', subscription: '' };
  let activeLogin: LoginSession | null = null;
  let loginBusy = false;
  let selectedKeys = new Set<string>();
  let editingKey = '';
  let editForm = { nickname: '', subscription: '', status: '' };
  let timers: number[] = [];
  let loginPollTimer: number | undefined;

  $: usageByAccount = new Map(usageRecords.map((record) => [record.account, record]));
  $: dashboardAccounts = accounts.map((account) => toDashboardAccount(account, usageByAccount, selectedKeys, refreshingUsageKeys));
  $: selectedCount = selectedKeys.size;
  $: availableCount = countAvailable(dashboardAccounts);
  $: suggestedSwitchCount = countSuggestedSwitch(dashboardAccounts);
  $: lastSyncLabel = formatRelativeTime(latestRefresh(usageRecords));
  $: anyLoading = loadingAccounts || loadingUsage;

  onMount(() => {
    void refreshDashboard();
    timers = [
      window.setInterval(() => refreshDashboard({ accounts: true, usage: false }), 30_000),
      window.setInterval(() => refreshDashboard({ accounts: false, usage: true }), 60_000)
    ];
  });

  onDestroy(() => {
    timers.forEach((timer) => window.clearInterval(timer));
    if (loginPollTimer) window.clearInterval(loginPollTimer);
  });

  async function refreshDashboard(scope: RefreshScope = {}) {
    const refreshAccounts = scope.accounts ?? true;
    const refreshUsage = scope.usage ?? true;
    const jobs: Promise<void>[] = [];
    if (refreshAccounts) jobs.push(loadAccounts());
    if (refreshUsage) jobs.push(loadUsage());
    await Promise.all(jobs);
  }

  async function loadAccounts() {
    loadingAccounts = true;
    try {
      const response = await fetch(API.accounts, { headers: { Accept: 'application/json' } });
      if (!response.ok) throw new Error(`读取账号失败：HTTP ${response.status}`);
      accounts = await response.json();
      pruneSelection();
      errorMessage = '';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '读取账号失败';
    } finally {
      loadingAccounts = false;
    }
  }

  async function loadUsage() {
    loadingUsage = true;
    try {
      const response = await fetch(API.usage, { headers: { Accept: 'application/json' } });
      if (!response.ok) throw new Error(`读取 Codex 用量失败：HTTP ${response.status}`);
      usageRecords = await response.json();
      errorMessage = '';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '读取 Codex 用量失败';
    } finally {
      loadingUsage = false;
    }
  }

  async function switchAccount(account: Account) {
    const selector = accountKey(account);
    if (!selector || switchingSelector) return;
    switchingSelector = selector;
    markUsageRefreshing(selector, true);
    errorMessage = '';
    noticeMessage = '';
    try {
      const response = await fetch(API.switch, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify(account.profile_name ? { profile_name: account.profile_name } : { id: account.id })
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || `切换失败：HTTP ${response.status}`);
      }
      await refreshDashboard();
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '切换账号失败';
    } finally {
      switchingSelector = '';
      markUsageRefreshing(selector, false);
    }
  }

  function markUsageRefreshing(key: string, refreshing: boolean) {
    const next = new Set(refreshingUsageKeys);
    if (refreshing) next.add(key);
    else next.delete(key);
    refreshingUsageKeys = next;
  }

  function toDashboardAccount(
    account: Account,
    usageMap: Map<string, UsageRecord>,
    selectedSet: Set<string>,
    refreshingSet: Set<string>
  ): DashboardAccount {
    const usage = usageMap.get(accountUsageKey(account));
    const remainingWeekly = remainingWeeklyValue(account, usage);
    const hasWeeklyWindow = usage?.has_weekly_window ?? account.has_weekly_window ?? false;
    const authExpired = isAuthExpired(account);
    const authProblem = hasAuthProblem(account);
    const reportedAvailability = usage?.availability || account.availability || 'unknown';
    const availability: Availability = authProblem ? 'blocked' : reportedAvailability;
    const tone = toneFor(availability, remainingWeekly, hasWeeklyWindow);
    const key = accountKey(account);
    const usageError = visibleUsageError(account, usage?.error || account.usage_error || '');
    return {
      key,
      account,
      usage,
      remainingWeekly,
      hasWeeklyWindow,
      availability,
      tone,
      statusLabel: authExpired ? '认证过期' : labelFor(availability, tone),
      entitlementSummary: entitlementSummary(account, usage),
      loginStatusLabel: authExpired ? '需要重新登录' : loginStatusLabel(account.status),
      authExpired,
      authStatusLabel: authStatusLabel(account),
      authStatusDetail: authStatusDetail(account),
      selected: selectedSet.has(key),
      refreshingUsage: refreshingSet.has(key),
      usageStale: authProblem ? false : Boolean(usage?.stale ?? account.usage_stale),
      usageError
    };
  }

  function remainingWeeklyValue(account: Account, record: UsageRecord | undefined) {
    return clampPercent(record?.weekly_remaining ?? record?.weekly_usage ?? account.weekly_usage ?? 0);
  }

  function entitlementSummary(account: Account, record: UsageRecord | undefined) {
    const planType = record?.plan_type || account.plan_type || '';
    const unlimited = record?.credits_unlimited ?? account.credits_unlimited;
    const creditsAvailable = record?.credits_available ?? account.credits_available;
    const balance = record?.credits_balance ?? account.credits_balance;
    const items = planType ? [planType] : [];
    if (unlimited) items.push('credits 不限');
    else if (creditsAvailable) items.push(balance ? `credits ${balance}` : 'credits 可用');
    else if (balance) items.push(`credits ${balance}`);
    return items.join(' · ');
  }

  function loginStatusLabel(status = 'ready') {
    if (status === 'active') return '';
    if (status === 'login_pending') return '登录中';
    if (status === 'login_failed') return '登录失败';
    if (status === 'disabled') return '已停用';
    return '已登录';
  }

  function isAuthExpired(account: Account) {
    return Boolean(account.auth_expired || account.auth_status === 'expired');
  }

  function hasAuthProblem(account: Account) {
    return isAuthExpired(account) || account.auth_status === 'missing' || account.auth_status === 'invalid';
  }

  function authStatusLabel(account: Account) {
    if (isAuthExpired(account)) return '认证已过期';
    if (account.auth_status === 'missing') return '认证文件缺失';
    if (account.auth_status === 'invalid') return '认证文件异常';
    if (account.auth_status === 'ready') return '认证有效';
    return '';
  }

  function authStatusDetail(account: Account) {
    if (!account.auth_expires_at) return '';
    const date = new Date(account.auth_expires_at);
    if (Number.isNaN(date.getTime())) return '';
    const prefix = isAuthExpired(account) ? '过期于' : '有效至';
    return `${prefix} ${date.toLocaleString('zh-CN')}`;
  }

  function visibleUsageError(account: Account, message = '') {
    if (!message) return '';
    if (hasAuthProblem(account)) return '';
    if (isAuthRefreshError(message)) return '';
    return message;
  }

  function isAuthRefreshError(message = '') {
    const normalized = message.toLowerCase();
    return normalized.includes('refresh codex auth') || normalized.includes('token_expired');
  }

  function displayUsageError(message = '') {
    if (!message) return '';
    return message.length > 90 ? `${message.slice(0, 87)}...` : message;
  }

  function sessionStatusLabel(status = '') {
    const labels: Record<string, string> = {
      pending: '等待授权',
      waiting_user: '等待用户输入设备码',
      succeeded: '已登录',
      failed: '登录失败',
      expired: '已过期',
      cancelled: '已取消'
    };
    return labels[status] || status || '未开始';
  }

  function accountKey(account: Account) {
    return accountRecordKey(account);
  }

  function toggleSelected(account: Account) {
    const key = accountKey(account);
    const next = new Set(selectedKeys);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    selectedKeys = next;
  }

  function pruneSelection() {
    selectedKeys = pruneSelectedKeys(selectedKeys, accounts);
  }

  function openAddAccount() {
    addingAccount = !addingAccount;
    noticeMessage = '';
  }

  async function startCodexLogin() {
    await startLogin(API.loginStart, '登录已创建，请打开验证地址并输入用户 code。', true);
  }

  async function startLogin(url: string, notice: string, openAuthorization = false) {
    loginBusy = true;
    errorMessage = '';
    noticeMessage = '';
    try {
      const response = await fetch(url, { method: 'POST', headers: { Accept: 'application/json' } });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || `启动登录失败：HTTP ${response.status}`);
      }
      activeLogin = await response.json();
      const authorizationURL = activeLogin.verification_url || activeLogin.authorization_url;
      if (openAuthorization && authorizationURL?.startsWith('http')) {
        window.open(authorizationURL, '_blank', 'noopener,noreferrer');
      }
      noticeMessage = notice;
      startLoginPolling();
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '启动登录失败';
    } finally {
      loginBusy = false;
    }
  }

  function startLoginPolling() {
    if (loginPollTimer) window.clearInterval(loginPollTimer);
    if (!activeLogin) return;
    loginPollTimer = window.setInterval(pollLoginStatus, 2_000);
  }

  async function pollLoginStatus() {
    if (!activeLogin) return;
    try {
      const response = await fetch(API.loginStatus(activeLogin.id), { headers: { Accept: 'application/json' } });
      if (!response.ok) return;
      activeLogin = await response.json();
      if (['succeeded', 'failed', 'expired', 'cancelled'].includes(activeLogin.status)) {
        if (loginPollTimer) window.clearInterval(loginPollTimer);
        await refreshDashboard();
      }
    } catch {
      // poll failure is non-fatal; next tick may recover.
    }
  }

  async function cancelLogin() {
    if (!activeLogin) return;
    loginBusy = true;
    try {
      const response = await fetch(API.loginCancel(activeLogin.id), { method: 'POST', headers: { Accept: 'application/json' } });
      if (!response.ok) throw new Error(`取消登录失败：HTTP ${response.status}`);
      activeLogin = await response.json();
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '取消登录失败';
    } finally {
      loginBusy = false;
    }
  }

  async function submitAddAccount() {
    const nickname = addForm.nickname.trim();
    const profileName = addForm.profile_name.trim();
    const authPath = addForm.auth_path.trim();
    if (!nickname || !authPath) {
      noticeMessage = '请填写账号昵称和 auth 文件路径。';
      return;
    }
    errorMessage = '';
    noticeMessage = '';
    try {
      const response = await fetch(API.accounts, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({
          nickname,
          profile_name: profileName,
          auth_path: authPath,
          login_method: 'imported',
          status: 'ready',
          subscription: addForm.subscription.trim()
        })
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || `新增账号失败：HTTP ${response.status}`);
      }
      addForm = { nickname: '', profile_name: '', auth_path: '', subscription: '' };
      manualImportOpen = false;
      addingAccount = false;
      await refreshDashboard();
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '新增账号失败';
    }
  }

  function startEdit(account: Account) {
    editingKey = accountKey(account);
    editForm = {
      nickname: account.nickname,
      subscription: account.subscription || '',
      status: account.status || 'ready'
    };
  }

  function cancelEdit() {
    editingKey = '';
  }

  async function submitEdit(account: Account) {
    const id = account.id || account.nickname;
    errorMessage = '';
    noticeMessage = '';
    try {
      const response = await fetch(API.account(id), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify(editForm)
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || `保存账号失败：HTTP ${response.status}`);
      }
      editingKey = '';
      await refreshDashboard();
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '保存账号失败';
    }
  }

  async function deleteSelected() {
    if (selectedCount === 0) return;
    errorMessage = '';
    noticeMessage = '';
    const ids = [...selectedKeys];
    try {
      const response = await fetch(API.accounts, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ profile_names: ids, ids })
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || `删除账号失败：HTTP ${response.status}`);
      }
      selectedKeys = new Set<string>();
      await refreshDashboard();
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : '删除账号失败';
    }
  }

  function latestRefresh(records: UsageRecord[]) {
    const times = records
      .map((record) => Date.parse(record.last_refresh || ''))
      .filter((time) => Number.isFinite(time));
    if (times.length === 0) return undefined;
    return new Date(Math.max(...times));
  }

  function formatRelativeTime(date?: Date) {
    if (!date) return anyLoading ? '同步中' : '暂无数据';
    const diffMs = Date.now() - date.getTime();
    if (!Number.isFinite(diffMs) || diffMs < 30_000) return '刚刚';
    const minutes = Math.round(diffMs / 60_000);
    if (minutes < 60) return `${minutes} 分钟前`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours} 小时前`;
    return `${Math.round(hours / 24)} 天前`;
  }

  function formatLastSwitch(account: Account) {
    const raw = account.last_used_at || account.last_switch_at || account.last_switch || '';
    if (!raw) return account.active ? '刚刚' : '未知';
    const parsed = new Date(raw);
    if (Number.isNaN(parsed.getTime())) return raw;
    return formatRelativeTime(parsed);
  }

  function initialFor(nickname: string) {
    return (nickname.trim()[0] || '?').toUpperCase();
  }
</script>

<svelte:head>
  <title>couswee 账号切换监测</title>
</svelte:head>

<main class="dashboard-shell">
  <header class="dashboard-header">
    <div class="title-block">
      <h1>couswee 账号切换监测</h1>
      <p>个人多账号用量概览</p>
    </div>

    <div class="top-controls" aria-label="账号概览">
      <section class="summary-grid" aria-label="账号状态概览">
        <article class="summary-card tone-ok">
          <span class="summary-icon">♟</span>
          <div>
            <span>可用账号</span>
            <strong>{availableCount}</strong>
          </div>
        </article>
        <article class="summary-card tone-warn">
          <span class="summary-icon">↻</span>
          <div>
            <span>建议切换</span>
            <strong>{suggestedSwitchCount}</strong>
          </div>
        </article>
        <article class="summary-card tone-ok">
          <span class="summary-icon">◷</span>
          <div>
            <span>最后同步</span>
            <strong>{lastSyncLabel}</strong>
          </div>
        </article>
      </section>
    </div>
  </header>

  <section class="management-panel" aria-label="账号管理">
    <div class="management-toolbar">
      <span class="gear-icon">⚙</span>
      <strong>账号管理</strong>
      <div class="management-actions">
        <button class="manager-button add" type="button" on:click={openAddAccount}>
          <span>＋</span>
          新增账号
        </button>
        <button class="manager-button delete" type="button" disabled={selectedCount === 0} on:click={deleteSelected}>
          <span>♲</span>
          删除选中
        </button>
      </div>
      <span class="selected-count">已选择 {selectedCount} 项</span>
    </div>
  </section>

  {#if addingAccount}
    <section class="login-panel" aria-label="新增账号登录面板">
      <div class="login-methods">
        <button type="button" class="login-method" disabled={loginBusy} on:click={startCodexLogin}>
          <strong>Codex 登录</strong>
          <span>打开授权页并输入用户 code，完成后写入受管 profile。</span>
        </button>
        <button type="button" class="login-method muted" on:click={() => (manualImportOpen = !manualImportOpen)}>
          <strong>手动导入</strong>
          <span>兼容旧 auth 文件路径，不经过登录流程。</span>
        </button>
      </div>

      {#if activeLogin}
        <div class="login-session-card">
          <div>
            <span class="session-status">{sessionStatusLabel(activeLogin.status)}</span>
            <strong>Codex 登录会话</strong>
          </div>
          {#if activeLogin.verification_url || activeLogin.authorization_url}
            <p>
              验证地址：
              <a href={activeLogin.verification_url || activeLogin.authorization_url} target="_blank" rel="noreferrer">
                {activeLogin.verification_url || activeLogin.authorization_url}
              </a>
            </p>
          {/if}
          {#if activeLogin.user_code}
            <p>用户 code：<code>{activeLogin.user_code}</code></p>
          {/if}
          {#if activeLogin.expires_at}
            <p>过期时间：{new Date(activeLogin.expires_at).toLocaleString('zh-CN')}</p>
          {/if}
          {#if activeLogin.error}
            <p class="usage-error">{activeLogin.error}</p>
          {/if}
          <button type="button" disabled={loginBusy} on:click={cancelLogin}>取消登录</button>
        </div>
      {/if}

      {#if manualImportOpen}
        <form class="add-account-form" aria-label="手动导入账号" on:submit|preventDefault={submitAddAccount}>
          <input bind:value={addForm.nickname} type="text" placeholder="账号昵称" autocomplete="off" />
          <input bind:value={addForm.profile_name} type="text" placeholder="profile_name（留空则按昵称生成）" autocomplete="off" />
          <input bind:value={addForm.auth_path} type="text" placeholder="auth 文件路径，例如 ~/.codex-auth/main.json" autocomplete="off" />
          <input bind:value={addForm.subscription} type="text" placeholder="订阅/备注（可选）" autocomplete="off" />
          <button type="submit">保存账号</button>
        </form>
      {/if}
    </section>
  {/if}

  {#if errorMessage}
    <div class="message error" role="alert">{errorMessage}</div>
  {/if}

  {#if noticeMessage}
    <div class="message notice" role="status">{noticeMessage}</div>
  {/if}

  <section class="account-list" aria-label="Codex 账号列表">
    {#if loadingAccounts && accounts.length === 0}
      <div class="empty-card">正在加载账号数据…</div>
    {:else if accounts.length === 0}
      <div class="empty-card">还没有账号。请使用账号管理中的 Codex 登录或手动导入添加账号。</div>
    {:else}
      {#each dashboardAccounts as item (item.key)}
        <article class="account-card {item.tone}" class:selected={item.selected} class:active={item.account.active} aria-label={`${item.account.nickname} 账号卡片${item.selected ? '，已选中' : ''}`}>
          <button
            class="select-box"
            class:checked={item.selected}
            type="button"
            aria-label={`${item.selected ? '取消选择' : '选择'} ${item.account.nickname}`}
            aria-pressed={item.selected}
            title={item.selected ? '取消选择' : '选择账号'}
            on:click={() => toggleSelected(item.account)}
          ></button>

          <div class="identity-block">
            <div class="avatar {item.tone}" aria-hidden="true">{initialFor(item.account.nickname)}</div>
            <div class="identity-copy">
              <h2>{item.account.nickname}</h2>
              <span class="status-pill {item.tone}"><i></i>{item.loginStatusLabel ? `${item.statusLabel} · ${item.loginStatusLabel}` : item.statusLabel}</span>
              <p>{item.account.profile_name || '未命名 profile'} · 上次切换： {formatLastSwitch(item.account)}</p>
              {#if item.entitlementSummary}
                <p class="entitlement-state">{item.entitlementSummary}</p>
              {/if}
              {#if item.authStatusLabel}
                <p class="auth-state" class:danger={item.authExpired}>{item.authStatusLabel}{item.authStatusDetail ? `，${item.authStatusDetail}` : ''}</p>
              {/if}
            </div>
          </div>

          <div class="usage-block" aria-label={`${item.account.nickname} 剩余流量`}>
            {#if item.hasWeeklyWindow}
              <div class="limit-row">
                <div class="limit-heading">
                  <span class="limit-label">周额度</span>
                  <strong class="remaining {item.tone}">{item.remainingWeekly}% 可用</strong>
                </div>
                <div class="meter" aria-label={`weekly remaining ${item.remainingWeekly}%`}>
                  <span class={item.tone} style={`width: ${item.remainingWeekly}%`}></span>
                </div>
              </div>
            {/if}
            {#if item.refreshingUsage}
              <p class="usage-note">正在刷新用量…</p>
            {:else if item.usageError}
              <p class="usage-error" title={item.usageError}>{displayUsageError(item.usageError)}</p>
            {:else if item.usageStale}
              <p class="usage-note">数据可能已过期</p>
            {/if}
          </div>

          <div class="switch-actions">
            <button
              class="switch-button {item.tone}"
              class:current={item.account.active}
              type="button"
              disabled={item.account.active || switchingSelector === item.key}
              on:click={() => switchAccount(item.account)}
            >
              {item.account.active ? '当前使用' : switchingSelector === item.key ? '切换中…' : '切换'}
            </button>
            <button type="button" class="edit-button" on:click={() => startEdit(item.account)}>编辑</button>
          </div>

          {#if editingKey === item.key}
            <form class="edit-account-form" on:submit|preventDefault={() => submitEdit(item.account)}>
              <input bind:value={editForm.nickname} type="text" placeholder="账号昵称" autocomplete="off" />
              <input bind:value={editForm.subscription} type="text" placeholder="订阅/备注" autocomplete="off" />
              <select bind:value={editForm.status}>
                <option value="ready">已登录</option>
                <option value="active">已激活</option>
                <option value="login_pending">登录中</option>
                <option value="login_failed">登录失败</option>
                <option value="disabled">已停用</option>
              </select>
              <button type="submit">保存</button>
              <button type="button" on:click={cancelEdit}>取消</button>
            </form>
          {/if}
        </article>
      {/each}
    {/if}
  </section>

</main>
