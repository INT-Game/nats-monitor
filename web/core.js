export const elements = Object.fromEntries(
  [...document.querySelectorAll('[id]')].map((element) => [element.id, element]),
);

const numberFormatter = new Intl.NumberFormat('zh-CN');

export function formatNumber(value, maximumFractionDigits = 0) {
  if (maximumFractionDigits === 0) return numberFormatter.format(value || 0);
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits }).format(value || 0);
}

export function formatRate(value) {
  if (!value) return '0 /s';
  if (value >= 1_000_000) return `${formatNumber(value / 1_000_000, 1)}M/s`;
  if (value >= 1_000) return `${formatNumber(value / 1_000, 1)}k/s`;
  return `${formatNumber(value, 1)}/s`;
}

export function formatBytes(value, suffix = '') {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = Number(value) || 0;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  const digits = size >= 100 || unit === 0 ? 0 : 1;
  return `${formatNumber(size, digits)} ${units[unit]}${suffix}`;
}

export function formatTime(value) {
  if (!value || value.startsWith('0001-')) return '--';
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(new Date(value));
}

export function formatDateTime(value) {
  if (!value || value.startsWith('0001-')) return '--';
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(new Date(value));
}

export function setText(id, value) {
  elements[id].textContent = value;
}

export function appendCell(row, value, className = '') {
  const cell = document.createElement('td');
  cell.textContent = value;
  if (className) cell.className = className;
  row.append(cell);
  return cell;
}

export function appendEmpty(parent, message) {
  const empty = document.createElement('p');
  empty.className = 'empty-state';
  empty.textContent = message;
  parent.append(empty);
}
