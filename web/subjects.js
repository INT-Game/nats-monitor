import { appendCell, elements, formatNumber, setText } from './core.js';

const subjectTypes = {
  custom: { label: '业务', order: 0 },
  inbox: { label: '临时 Inbox', order: 1 },
  system: { label: '系统内部', order: 2 },
};

const internalPrefixes = ['$SYS.', '$JS.', '$KV.', '$O.', '$SRV.'];

export function classifySubject(subject) {
  const value = String(subject || '');
  if (value === '_INBOX' || value.startsWith('_INBOX.')) return 'inbox';
  if (internalPrefixes.some((prefix) => value === prefix.slice(0, -1) || value.startsWith(prefix))) return 'system';
  return 'custom';
}

function compareText(left, right) {
  const a = String(left || '');
  const b = String(right || '');
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

function compareSubjects(left, right) {
  const leftType = classifySubject(left.subject);
  const rightType = classifySubject(right.subject);
  return subjectTypes[leftType].order - subjectTypes[rightType].order
    || compareText(left.subject, right.subject)
    || compareText(left.account, right.account)
    || compareText(left.qgroup, right.qgroup)
    || Number(left.cid || 0) - Number(right.cid || 0)
    || compareText(left.sid, right.sid);
}

export function renderSubjects(subscriptions) {
  const list = [...(subscriptions.subscriptions_list || [])].sort(compareSubjects);
  const selectedAccount = elements.subjectAccount.value;
  const accounts = [...new Set(list.map((item) => item.account).filter(Boolean))].sort(compareText);
  const existing = [...elements.subjectAccount.options].slice(1).map((option) => option.value);
  if (existing.join('\0') !== accounts.join('\0')) {
    elements.subjectAccount.replaceChildren(new Option('全部账号', ''));
    for (const account of accounts) elements.subjectAccount.add(new Option(account, account));
    elements.subjectAccount.value = accounts.includes(selectedAccount) ? selectedAccount : '';
  }

  const query = elements.subjectSearch.value.trim().toLowerCase();
  const account = elements.subjectAccount.value;
  const selectedType = elements.subjectType.value;
  const counts = { custom: 0, inbox: 0, system: 0 };
  for (const item of list) counts[classifySubject(item.subject)] += 1;
  const filtered = list.filter((item) => {
    const type = classifySubject(item.subject);
    const matchesType = !selectedType || type === selectedType;
    const matchesAccount = !account || item.account === account;
    const matchesQuery = !query || [item.subject, item.qgroup, item.account, item.cid, item.sid]
      .some((value) => String(value || '').toLowerCase().includes(query));
    return matchesType && matchesAccount && matchesQuery;
  });
  const queues = new Set(list.map((item) => item.qgroup).filter(Boolean));

  setText('subsCurrent', formatNumber(subscriptions.num_subscriptions));
  setText('subsLoaded', formatNumber(list.length));
  setText('subsQueues', formatNumber(queues.size));
  setText('subsHitRate', `${formatNumber((subscriptions.cache_hit_rate || 0) * 100, 1)}%`);
  setText('subsMatches', formatNumber(subscriptions.num_matches));
  setText('subsFanout', `${formatNumber(subscriptions.avg_fanout, 1)} / ${formatNumber(subscriptions.max_fanout)}`);
  const total = subscriptions.total || list.length;
  setText(
    'subjectResultMeta',
    `显示 ${formatNumber(filtered.length)} 条 · 业务 ${formatNumber(counts.custom)} · Inbox ${formatNumber(counts.inbox)} · 系统 ${formatNumber(counts.system)} · 端点共 ${formatNumber(total)} 条`,
  );

  elements.subjectsBody.replaceChildren();
  elements.subjectsEmpty.hidden = filtered.length > 0;
  for (const item of filtered) {
    const type = classifySubject(item.subject);
    const row = document.createElement('tr');
    appendCell(row, item.subject || '--', 'subject-name');
    const typeCell = appendCell(row, '');
    const badge = document.createElement('span');
    badge.className = `subject-type-badge is-${type}`;
    badge.textContent = subjectTypes[type].label;
    typeCell.append(badge);
    appendCell(row, item.account || '--');
    appendCell(row, item.qgroup || '--');
    appendCell(row, String(item.cid ?? '--'));
    appendCell(row, item.sid || '--');
    appendCell(row, formatNumber(item.msgs));
    appendCell(row, item.max ? formatNumber(item.max) : '--');
    elements.subjectsBody.append(row);
  }
}
