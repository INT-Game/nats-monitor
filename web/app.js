import { appendCell, appendEmpty, elements, formatBytes, formatDateTime, formatNumber, formatRate, formatTime, setText } from './core.js';
import { drawChart } from './chart.js';
import { renderSubjects } from './subjects.js';

const state = {
  snapshot: null,
  refreshTimer: null,
  refreshing: false,
};

const viewNames = ['overview', 'connections', 'slowconsumers', 'subjects', 'accounts', 'topology', 'jetstream'];

async function loadSnapshot() {
  if (state.refreshing) return;
  state.refreshing = true;
  elements.refreshButton.disabled = true;
  elements.refreshButton.textContent = '刷新中';
  try {
    const response = await fetch('/api/snapshot', { cache: 'no-store' });
    if (response.status === 401) {
      window.location.assign('/login');
      return;
    }
    if (!response.ok) throw new Error(`监控服务返回 ${response.status}`);
    state.snapshot = await response.json();
    render(state.snapshot);
  } catch (error) {
    showClientError(error.message);
  } finally {
    state.refreshing = false;
    elements.refreshButton.disabled = false;
    elements.refreshButton.textContent = '立即刷新';
  }
}

function showClientError(message) {
  elements.errorBanner.hidden = false;
  setText('errorText', message);
  setHealth(false, '页面连接失败');
}

function setHealth(healthy, label) {
  elements.healthBadge.className = `health-badge ${healthy ? '' : 'is-offline'}`.trim();
  setText('healthText', label);
}

function render(data) {
  setText('targetLabel', data.target || '--');
  renderTargetPresets(data.targets || []);
  setText('viewerName', data.viewer || '--');
  setHealth(data.healthy, data.healthy ? '运行正常' : '服务离线');
  elements.errorBanner.hidden = data.healthy;
  if (!data.healthy) setText('errorText', data.error || 'NATS 监控端点无响应');
  setText('lastUpdated', data.last_success_at && !data.last_success_at.startsWith('0001-')
    ? `最后成功采样 ${formatTime(data.last_success_at)}`
    : '尚无成功采样');

  const server = data.server || {};
  const rates = data.rates || {};
  const subscriptions = data.subscriptions || {};
  const pendingConnections = (data.connections || []).filter((connection) => connection.pending_bytes > 0);
  setText('metricConnections', formatNumber(server.connections));
  setText('metricConnectionsMeta', `累计 ${formatNumber(server.total_connections)}`);
  setText('metricMessages', formatRate(rates.messages_per_second));
  setText('metricBytes', formatBytes(rates.bytes_per_second, '/s'));
  setText('metricSubscriptions', formatNumber(server.subscriptions || subscriptions.num_subscriptions));
  setText('metricFanout', `平均扇出 ${formatNumber(subscriptions.avg_fanout, 1)}`);
  setText('metricSlowConsumers', formatNumber(server.slow_consumers));
  setText('metricSlowState', `当前积压 ${formatNumber(pendingConnections.length)}`);
  elements.metricSlowConsumers.closest('.metric').classList.remove('is-warning');

  renderHealth(data);
  renderServerDetails(server);
  renderWarnings(data);
  renderConnections(data.connections || []);
  renderSlowConsumers(data);
  renderSubjects(subscriptions);
  renderAccounts(data.accounts || {}, data.account_stats || []);
  renderTopology(data);
  renderJetStream(data.jetstream || {});
  requestAnimationFrame(() => drawChart(data.history || []));
}

function renderServerDetails(server) {
  setText('detailServerName', server.server_name || '--');
  setText('detailServerID', server.server_id || '--');
  setText('detailServerBind', `${server.host || '--'}:${server.port || '--'}`);
  setText('detailGoRuntime', server.go || '--');
  setText('detailCores', `${formatNumber(server.cores)} / ${formatNumber(server.gomaxprocs)}`);
  setText('detailMaxConnections', formatNumber(server.max_connections));
  setText('detailMaxPayload', formatBytes(server.max_payload));
  setText('detailAuthRequired', server.auth_required ? 'Required' : 'Not required');
}

function setStatus(id, label, tone = '') {
  const element = elements[id];
  element.className = tone ? `is-${tone}` : '';
  element.replaceChildren();
  const dot = document.createElement('span');
  dot.className = 'mini-dot';
  element.append(dot, document.createTextNode(label));
}

function gatewayCount(gateways) {
  const outbound = Object.keys(gateways?.outbound_gateways || {}).length;
  const inbound = Object.values(gateways?.inbound_gateways || {}).reduce((total, items) => total + items.length, 0);
  return { outbound, inbound, total: outbound + inbound };
}

function renderHealth(data) {
  const server = data.server || {};
  const gateways = gatewayCount(data.gateways);
  const topologyTotal = (data.routes || []).length + gateways.total + (data.leaf_nodes?.leafs || []).length;
  setStatus('statusServer', data.healthy ? '可用' : '离线', data.healthy ? '' : 'offline');
  setStatus('statusData', data.last_success_at && !data.last_success_at.startsWith('0001-') ? '已同步' : '无数据', data.healthy ? '' : 'muted');
  setStatus('statusTopology', `${topologyTotal} 条连接`, data.healthy ? '' : 'muted');
  const pendingCount = (data.connections || []).filter((connection) => connection.pending_bytes > 0).length;
  setStatus('statusSlow', pendingCount ? `${pendingCount} 条积压` : '无积压', pendingCount ? 'warning' : '');
  setText('serverVersion', server.version || '--');
  setText('serverUptime', server.uptime || '--');
  setText('serverCPU', `${formatNumber(server.cpu, 1)}%`);
  setText('serverMemory', formatBytes(server.mem));
}

function renderWarnings(data) {
  const warnings = [...(data.warnings || [])];
  if (!data.healthy && data.error) warnings.unshift(data.error);
  setText('warningCount', String(warnings.length));
  elements.warningList.replaceChildren();
  if (!warnings.length) {
    appendEmpty(elements.warningList, '暂无异常');
    return;
  }
  for (const warning of warnings) {
    const item = document.createElement('div');
    item.className = 'warning-item';
    const time = document.createElement('time');
    time.textContent = formatTime(data.last_attempt_at);
    const message = document.createElement('p');
    message.textContent = warning;
    item.append(time, message);
    elements.warningList.append(item);
  }
}

function renderConnections(connections) {
  const query = elements.connectionSearch.value.trim().toLowerCase();
  const filtered = connections.filter((connection) =>
    [connection.name, connection.ip, connection.account, connection.authorized_user, connection.lang, connection.version]
      .some((value) => String(value || '').toLowerCase().includes(query)),
  );
  elements.connectionsBody.replaceChildren();
  elements.connectionsEmpty.hidden = filtered.length > 0;
  for (const connection of filtered) {
    const row = document.createElement('tr');
    appendCell(row, connection.name || `CID ${connection.cid}`, 'client-name');
    appendCell(row, `${connection.ip || '--'}:${connection.port || '--'}`);
    appendCell(row, [connection.account, connection.authorized_user].filter(Boolean).join(' / ') || '--');
    appendCell(row, [connection.lang, connection.version].filter(Boolean).join(' ') || connection.type || '--');
    appendCell(row, formatNumber(connection.subscriptions));
    appendCell(row, formatBytes(connection.pending_bytes), connection.pending_bytes > 1024 * 1024 ? 'pending-warning' : '');
    appendCell(row, `${formatNumber(connection.in_msgs)} / ${formatBytes(connection.in_bytes)}`);
    appendCell(row, `${formatNumber(connection.out_msgs)} / ${formatBytes(connection.out_bytes)}`);
    appendCell(row, connection.rtt || '--');
    appendCell(row, connection.uptime || '--');
    elements.connectionsBody.append(row);
  }
}

function renderSlowConsumers(data) {
  const server = data.server || {};
  const stats = server.slow_consumer_stats || {};
  const pending = [...(data.connections || [])]
    .filter((connection) => connection.pending_bytes > 0)
    .sort((left, right) => right.pending_bytes - left.pending_bytes || left.cid - right.cid);
  const recent = [...(data.recent_slow_consumers || [])]
    .sort((left, right) => new Date(right.stop) - new Date(left.stop));

  setText('slowTotal', formatNumber(server.slow_consumers));
  setText('slowClients', formatNumber(stats.clients));
  setText('slowRoutes', formatNumber(stats.routes));
  setText('slowGateways', formatNumber(stats.gateways));
  setText('slowLeafs', formatNumber(stats.leafs));
  setText('slowPending', formatNumber(pending.length));
  setText('slowScanMeta', `检查最近 ${formatNumber(data.closed_connections_scanned)} / ${formatNumber(data.closed_connections_total)} 条关闭连接`);

  elements.slowPendingBody.replaceChildren();
  elements.slowPendingEmpty.hidden = pending.length > 0;
  for (const connection of pending) {
    const subjects = (connection.subscriptions_list_detail || []).map((item) => item.subject).filter(Boolean);
    const visibleSubjects = subjects.slice(0, 3).join(', ');
    const subjectLabel = subjects.length > 3 ? `${visibleSubjects} +${subjects.length - 3}` : visibleSubjects || '--';
    const row = document.createElement('tr');
    appendCell(row, connection.name || `CID ${connection.cid}`, 'client-name');
    appendCell(row, `${connection.ip || '--'}:${connection.port || '--'}`);
    appendCell(row, [connection.account, connection.authorized_user].filter(Boolean).join(' / ') || '--');
    appendCell(row, formatBytes(connection.pending_bytes), 'pending-warning');
    appendCell(row, subjectLabel, 'subject-list-cell');
    appendCell(row, formatNumber(connection.subscriptions));
    appendCell(row, `${formatNumber(connection.out_msgs)} / ${formatBytes(connection.out_bytes)}`);
    appendCell(row, connection.rtt || '--');
    appendCell(row, connection.uptime || '--');
    elements.slowPendingBody.append(row);
  }

  elements.slowRecentBody.replaceChildren();
  elements.slowRecentEmpty.hidden = recent.length > 0;
  for (const connection of recent) {
    const row = document.createElement('tr');
    appendCell(row, formatDateTime(connection.stop));
    appendCell(row, connection.name || `CID ${connection.cid}`, 'client-name');
    appendCell(row, `${connection.ip || '--'}:${connection.port || '--'}`);
    appendCell(row, [connection.account, connection.authorized_user].filter(Boolean).join(' / ') || '--');
    appendCell(row, connection.reason || 'Slow Consumer', 'pending-warning');
    appendCell(row, formatBytes(connection.pending_bytes));
    appendCell(row, `${formatNumber(connection.out_msgs)} / ${formatBytes(connection.out_bytes)}`);
    appendCell(row, formatNumber(connection.subscriptions));
    appendCell(row, connection.uptime || '--');
    elements.slowRecentBody.append(row);
  }
}

function renderAccounts(accounts, stats) {
  const statsByAccount = new Map(stats.map((item) => [item.acc, item]));
  const names = [...new Set([...(accounts.accounts || []), ...stats.map((item) => item.acc)])].filter(Boolean).sort();
  setText('systemAccount', `系统账号 ${accounts.system_account || '--'}`);
  elements.accountsBody.replaceChildren();
  elements.accountsEmpty.hidden = names.length > 0;
  for (const name of names) {
    const item = statsByAccount.get(name) || { acc: name, sent: {}, received: {} };
    const row = document.createElement('tr');
    appendCell(row, item.name || item.acc, 'client-name');
    appendCell(row, formatNumber(item.conns));
    appendCell(row, formatNumber(item.total_conns));
    appendCell(row, formatNumber(item.num_subscriptions));
    appendCell(row, formatNumber(item.sent?.msgs));
    appendCell(row, formatBytes(item.sent?.bytes));
    appendCell(row, formatNumber(item.received?.msgs));
    appendCell(row, formatBytes(item.received?.bytes));
    appendCell(row, formatNumber(item.leafnodes));
    appendCell(row, formatNumber(item.slow_consumers), item.slow_consumers ? 'pending-warning' : '');
    elements.accountsBody.append(row);
  }
}

function renderTopology(data) {
  const routes = data.routes || [];
  const gateways = data.gateways || {};
  const gatewayTotals = gatewayCount(gateways);
  const leafs = data.leaf_nodes?.leafs || [];
  setText('topologyRoutes', formatNumber(routes.length));
  setText('topologyOutbound', formatNumber(gatewayTotals.outbound));
  setText('topologyInbound', formatNumber(gatewayTotals.inbound));
  setText('topologyLeafs', formatNumber(leafs.length));

  elements.routesBody.replaceChildren();
  elements.routesEmpty.hidden = routes.length > 0;
  for (const route of routes) {
    const row = document.createElement('tr');
    appendCell(row, route.remote_name || route.remote_id || `RID ${route.rid}`, 'client-name');
    appendCell(row, `${route.ip || '--'}:${route.port || '--'}`);
    appendCell(row, route.is_configured ? 'Configured' : route.did_solicit ? 'Solicited' : 'Inbound');
    appendCell(row, formatNumber(route.subscriptions));
    appendCell(row, formatBytes(route.pending_size), route.pending_size > 1024 * 1024 ? 'pending-warning' : '');
    appendCell(row, `${formatNumber(route.in_msgs)} / ${formatBytes(route.in_bytes)}`);
    appendCell(row, `${formatNumber(route.out_msgs)} / ${formatBytes(route.out_bytes)}`);
    appendCell(row, route.rtt || '--');
    appendCell(row, route.uptime || '--');
    elements.routesBody.append(row);
  }

  const gatewayRows = [];
  for (const [name, item] of Object.entries(gateways.outbound_gateways || {})) gatewayRows.push({ direction: 'Outbound', name, ...item });
  for (const [name, items] of Object.entries(gateways.inbound_gateways || {})) {
    for (const item of items) gatewayRows.push({ direction: 'Inbound', name, ...item });
  }
  elements.gatewaysBody.replaceChildren();
  elements.gatewaysEmpty.hidden = gatewayRows.length > 0;
  for (const item of gatewayRows) {
    const connection = item.connection || {};
    const row = document.createElement('tr');
    appendCell(row, item.direction);
    appendCell(row, item.name || '--', 'client-name');
    appendCell(row, connection.name || `CID ${connection.cid || '--'}`);
    appendCell(row, `${connection.ip || '--'}:${connection.port || '--'}`);
    appendCell(row, formatNumber(connection.subscriptions));
    appendCell(row, `${formatNumber(connection.in_msgs)} / ${formatBytes(connection.in_bytes)}`);
    appendCell(row, `${formatNumber(connection.out_msgs)} / ${formatBytes(connection.out_bytes)}`);
    appendCell(row, connection.uptime || '--');
    elements.gatewaysBody.append(row);
  }

  elements.leafsBody.replaceChildren();
  elements.leafsEmpty.hidden = leafs.length > 0;
  for (const leaf of leafs) {
    const row = document.createElement('tr');
    appendCell(row, leaf.name || `ID ${leaf.id}`, 'client-name');
    appendCell(row, `${leaf.ip || '--'}:${leaf.port || '--'}`);
    appendCell(row, leaf.account || '--');
    appendCell(row, leaf.is_spoke ? 'Spoke' : leaf.is_isolated ? 'Isolated' : 'Hub');
    appendCell(row, formatNumber(leaf.subscriptions));
    appendCell(row, `${formatNumber(leaf.in_msgs)} / ${formatBytes(leaf.in_bytes)}`);
    appendCell(row, `${formatNumber(leaf.out_msgs)} / ${formatBytes(leaf.out_bytes)}`);
    appendCell(row, leaf.rtt || '--');
    appendCell(row, leaf.compression || '--');
    elements.leafsBody.append(row);
  }
}

function renderJetStream(jetStream) {
  const enabled = !jetStream.disabled;
  elements.jetstreamState.textContent = enabled ? 'Enabled' : 'Disabled';
  elements.jetstreamState.classList.toggle('is-enabled', enabled);
  elements.jetstreamDisabled.hidden = enabled;
  setText('jsStreams', formatNumber(jetStream.streams));
  setText('jsConsumers', formatNumber(jetStream.consumers));
  setText('jsMessages', formatNumber(jetStream.messages));
  setText('jsBytes', formatBytes(jetStream.bytes));
  setText('jsUsage', `${formatBytes(jetStream.memory)} / ${formatBytes(jetStream.storage)}`);
  setText('jsAPI', `${formatNumber(jetStream.api?.errors)} / ${formatNumber(jetStream.api?.total)}`);
}

function selectView(name) {
  const selected = viewNames.includes(name) ? name : 'overview';
  for (const tab of document.querySelectorAll('.tab')) {
    const active = tab.dataset.view === selected;
    tab.classList.toggle('is-active', active);
    tab.setAttribute('aria-selected', String(active));
  }
  document.querySelector('.tab.is-active')?.scrollIntoView({ block: 'nearest', inline: 'center' });
  for (const view of document.querySelectorAll('.view')) {
    const active = view.id === `${selected}View`;
    view.classList.toggle('is-active', active);
    view.hidden = !active;
  }
  if (selected === 'overview' && state.snapshot) requestAnimationFrame(() => drawChart(state.snapshot.history || []));
}

function scheduleRefresh() {
  clearInterval(state.refreshTimer);
  const delay = Number(elements.refreshInterval.value);
  if (delay > 0) state.refreshTimer = setInterval(loadSnapshot, delay);
}

function closeTargetPreset() {
  elements.targetPresetButton.setAttribute('aria-expanded', 'false');
  elements.targetPresetList.hidden = true;
}

function setTargetPreset(value) {
  const target = (state.snapshot?.targets || []).find((item) => item.url === value);
  elements.targetPresetButton.dataset.value = target?.url || '';
  elements.targetPresetLabel.textContent = target?.name || '临时输入';
  for (const option of elements.targetPresetList.querySelectorAll('[role="option"]')) {
    option.setAttribute('aria-selected', String(option.dataset.value === elements.targetPresetButton.dataset.value));
  }
}

function openTargetPreset() {
  elements.targetPresetButton.setAttribute('aria-expanded', 'true');
  elements.targetPresetList.hidden = false;
}

function openTargetDialog() {
  const currentTarget = state.snapshot?.target || '';
  elements.targetInput.value = currentTarget;
  setTargetPreset(currentTarget);
  closeTargetPreset();
  elements.targetError.hidden = true;
  elements.targetDialog.showModal();
  requestAnimationFrame(() => elements.targetInput.select());
}

function renderTargetPresets(targets) {
  const signature = JSON.stringify(targets);
  if (elements.targetPreset.dataset.signature === signature) return;
  elements.targetPresetList.replaceChildren();
  for (const target of [{ name: '临时输入', url: '' }, ...targets]) {
    const option = document.createElement('button');
    option.type = 'button';
    option.className = 'target-picker-option';
    option.dataset.value = target.url;
    option.setAttribute('role', 'option');
    option.setAttribute('aria-selected', 'false');

    const dot = document.createElement('span');
    dot.className = 'target-picker-dot';
    const copy = document.createElement('span');
    copy.className = 'target-picker-copy';
    const name = document.createElement('span');
    name.className = 'target-picker-name';
    name.textContent = target.name;
    const url = document.createElement('span');
    url.className = 'target-picker-url';
    url.textContent = target.url || '手动填写监控地址';
    copy.append(name, url);
    option.append(dot, copy);
    elements.targetPresetList.append(option);
  }
  elements.targetPreset.dataset.signature = signature;
  setTargetPreset(elements.targetPresetButton.dataset.value || '');
}

async function switchTarget(event) {
  event.preventDefault();
  elements.targetError.hidden = true;
  elements.targetSubmitButton.disabled = true;
  elements.targetSubmitButton.textContent = '连接中';
  try {
    const response = await fetch('/api/target', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: elements.targetInput.value.trim() }),
    });
    if (response.status === 401) {
      window.location.assign('/login');
      return;
    }
    const result = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(result.error || `切换失败 (${response.status})`);
    elements.targetDialog.close();
    await loadSnapshot();
  } catch (error) {
    elements.targetError.textContent = error.message;
    elements.targetError.hidden = false;
  } finally {
    elements.targetSubmitButton.disabled = false;
    elements.targetSubmitButton.textContent = '连接并切换';
  }
}

for (const tab of document.querySelectorAll('.tab')) {
  tab.addEventListener('click', () => {
    const name = tab.dataset.view;
    history.replaceState(null, '', `#${name}`);
    selectView(name);
  });
}

elements.refreshButton.addEventListener('click', loadSnapshot);
elements.retryButton.addEventListener('click', loadSnapshot);
elements.refreshInterval.addEventListener('change', scheduleRefresh);
elements.connectionSearch.addEventListener('input', () => renderConnections(state.snapshot?.connections || []));
elements.subjectSearch.addEventListener('input', () => renderSubjects(state.snapshot?.subscriptions || {}));
elements.subjectAccount.addEventListener('change', () => renderSubjects(state.snapshot?.subscriptions || {}));
elements.subjectType.addEventListener('change', () => renderSubjects(state.snapshot?.subscriptions || {}));
elements.targetButton.addEventListener('click', openTargetDialog);
elements.targetForm.addEventListener('submit', switchTarget);
elements.targetPresetButton.addEventListener('click', () => {
  if (elements.targetPresetList.hidden) openTargetPreset();
  else closeTargetPreset();
});
elements.targetPresetButton.addEventListener('keydown', (event) => {
  if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return;
  event.preventDefault();
  openTargetPreset();
  const options = [...elements.targetPresetList.querySelectorAll('[role="option"]')];
  const selected = options.find((option) => option.getAttribute('aria-selected') === 'true');
  (selected || options[event.key === 'ArrowDown' ? 0 : options.length - 1])?.focus();
});
elements.targetPresetList.addEventListener('click', (event) => {
  const option = event.target.closest('[role="option"]');
  if (!option) return;
  setTargetPreset(option.dataset.value);
  if (option.dataset.value) elements.targetInput.value = option.dataset.value;
  closeTargetPreset();
  elements.targetPresetButton.focus();
});
elements.targetPresetList.addEventListener('keydown', (event) => {
  const options = [...elements.targetPresetList.querySelectorAll('[role="option"]')];
  const index = options.indexOf(document.activeElement);
  if (event.key === 'Escape') {
    event.preventDefault();
    closeTargetPreset();
    elements.targetPresetButton.focus();
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const offset = event.key === 'ArrowDown' ? 1 : -1;
    options[(index + offset + options.length) % options.length]?.focus();
  } else if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault();
    options[event.key === 'Home' ? 0 : options.length - 1]?.focus();
  }
});
elements.targetInput.addEventListener('input', () => {
  if (elements.targetInput.value !== elements.targetPresetButton.dataset.value) setTargetPreset('');
});
elements.targetCancelButton.addEventListener('click', () => elements.targetDialog.close());
elements.targetCloseButton.addEventListener('click', () => elements.targetDialog.close());
elements.targetDialog.addEventListener('close', closeTargetPreset);
document.addEventListener('click', (event) => {
  if (!elements.targetPreset.contains(event.target)) closeTargetPreset();
});
elements.queueHelpButton.addEventListener('click', () => elements.queueHelpDialog.showModal());
elements.queueHelpCloseButton.addEventListener('click', () => elements.queueHelpDialog.close());
elements.queueHelpDoneButton.addEventListener('click', () => elements.queueHelpDialog.close());
elements.fanoutHelpButton.addEventListener('click', () => elements.fanoutHelpDialog.showModal());
elements.fanoutHelpCloseButton.addEventListener('click', () => elements.fanoutHelpDialog.close());
elements.fanoutHelpDoneButton.addEventListener('click', () => elements.fanoutHelpDialog.close());
window.addEventListener('resize', () => {
  document.querySelector('.tab.is-active')?.scrollIntoView({ block: 'nearest', inline: 'center' });
  if (state.snapshot && !elements.overviewView.hidden) drawChart(state.snapshot.history || []);
});

selectView(location.hash.slice(1));
scheduleRefresh();
loadSnapshot();
