import { elements, formatBytes, formatRate, formatTime } from './core.js';

export function drawChart(history) {
  const canvas = elements.trafficChart;
  const rect = canvas.getBoundingClientRect();
  const ratio = Math.min(window.devicePixelRatio || 1, 2);
  canvas.width = Math.max(1, Math.floor(rect.width * ratio));
  canvas.height = Math.max(1, Math.floor(rect.height * ratio));
  const context = canvas.getContext('2d');
  context.scale(ratio, ratio);
  const width = rect.width;
  const height = rect.height;
  context.clearRect(0, 0, width, height);

  elements.chartEmpty.hidden = history.length >= 2;
  if (history.length < 2) return;

  const padding = { top: 10, right: 48, bottom: 24, left: 48 };
  const plotWidth = Math.max(1, width - padding.left - padding.right);
  const plotHeight = Math.max(1, height - padding.top - padding.bottom);
  const maxMessages = Math.max(1, ...history.map((point) => point.messages_ps || 0));
  const maxBytes = Math.max(1, ...history.map((point) => point.bytes_ps || 0));

  context.strokeStyle = '#2b302c';
  context.lineWidth = 1;
  context.font = '10px Cascadia Code, Consolas, monospace';
  context.fillStyle = '#707971';
  context.textBaseline = 'middle';
  for (let index = 0; index <= 4; index += 1) {
    const y = padding.top + (plotHeight * index) / 4;
    context.beginPath();
    context.moveTo(padding.left, y + 0.5);
    context.lineTo(width - padding.right, y + 0.5);
    context.stroke();
    const fraction = 1 - index / 4;
    context.textAlign = 'right';
    context.fillText(formatRate(maxMessages * fraction).replace('/s', ''), padding.left - 8, y);
    context.textAlign = 'left';
    context.fillText(formatBytes(maxBytes * fraction), width - padding.right + 8, y);
  }

  drawSeries(context, history, 'messages_ps', maxMessages, '#57c785', padding, plotWidth, plotHeight);
  drawSeries(context, history, 'bytes_ps', maxBytes, '#56b8c7', padding, plotWidth, plotHeight);
  context.textAlign = 'left';
  context.fillStyle = '#707971';
  context.fillText(formatTime(history[0].at), padding.left, height - 8);
  context.textAlign = 'right';
  context.fillText(formatTime(history[history.length - 1].at), width - padding.right, height - 8);
}

function drawSeries(context, history, key, maxValue, color, padding, width, height) {
  context.beginPath();
  for (let index = 0; index < history.length; index += 1) {
    const x = padding.left + (width * index) / Math.max(1, history.length - 1);
    const y = padding.top + height - ((history[index][key] || 0) / maxValue) * height;
    if (index === 0) context.moveTo(x, y);
    else context.lineTo(x, y);
  }
  context.strokeStyle = color;
  context.lineWidth = 2;
  context.lineJoin = 'round';
  context.lineCap = 'round';
  context.stroke();
}
