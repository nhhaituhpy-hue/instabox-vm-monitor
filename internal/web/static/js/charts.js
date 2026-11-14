const SVG_NS = 'http://www.w3.org/2000/svg';
let chartUid = 0;

function svgEl(tag, attrs = {}) {
  const el = document.createElementNS(SVG_NS, tag);
  for (const [k, v] of Object.entries(attrs)) {
    el.setAttribute(k, String(v));
  }
  return el;
}

export function fmtBytes(bytes) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let idx = 0;
  let val = bytes;
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024;
    idx++;
  }
  return `${val >= 10 || idx === 0 ? val.toFixed(idx === 0 ? 0 : 1) : val.toFixed(2)} ${units[idx]}`;
}

export function fmtRate(bps) {
  return `${fmtBytes(bps)}/s`;
}

export function fmtAxisRate(bps) {
  if (!Number.isFinite(bps) || bps <= 0) return '0';
  const units = ['B', 'K', 'M', 'G'];
  let idx = 0;
  let val = bps;
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024;
    idx++;
  }
  return `${val >= 10 || idx === 0 ? Math.round(val) : val.toFixed(1)}${units[idx]}`;
}

export function buildMiniAreaChart(values, color, maxVal = 100) {
  const w = 160;
  const h = 40;
  const padY = 4;
  const uid = ++chartUid;
  const svg = svgEl('svg', {
    viewBox: `0 0 ${w} ${h}`,
    class: 'mini-chart-svg',
    'aria-hidden': 'true',
    preserveAspectRatio: 'none'
  });

  const defs = svgEl('defs');
  const grad = svgEl('linearGradient', { id: `mc-grad-${uid}`, x1: '0', y1: '0', x2: '0', y2: '1' });
  grad.append(
    svgEl('stop', { offset: '0%', 'stop-color': color, 'stop-opacity': '0.32' }),
    svgEl('stop', { offset: '100%', 'stop-color': color, 'stop-opacity': '0.03' })
  );
  defs.append(grad);
  svg.append(defs);

  const valid = (values || []).filter((v) => v !== null && v !== undefined && Number.isFinite(v));
  if (valid.length === 0) {
    svg.append(svgEl('line', {
      x1: 0, y1: h - 3, x2: w, y2: h - 3,
      stroke: color, 'stroke-opacity': '0.28', 'stroke-width': '1.2', 'stroke-dasharray': '3 3'
    }));
    return svg;
  }

  svg.append(svgEl('line', {
    x1: 0, y1: h - 1, x2: w, y2: h - 1,
    stroke: color, 'stroke-opacity': '0.18', 'stroke-width': '1'
  }));

  const pts = valid.length === 1 ? [valid[0], valid[0]] : valid;
  const max = Math.max(maxVal, 1);
  const coords = pts.map((v, idx) => {
    const clamped = Math.max(0, Math.min(max, Number(v) || 0));
    const x = (idx / (pts.length - 1)) * w;
    const y = (h - padY) - (clamped / max) * (h - padY * 2);
    return [x, y];
  });

  const lineD = coords.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0].toFixed(2)},${p[1].toFixed(2)}`).join(' ');
  const areaD = `${lineD} L${w},${h} L0,${h} Z`;

  svg.append(
    svgEl('path', { d: areaD, fill: `url(#mc-grad-${uid})` }),
    svgEl('path', { d: lineD, fill: 'none', stroke: color, 'stroke-width': '2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' })
  );
  return svg;
}

export function renderDetailChart(container, config) {
  const {
    title,
    unit = '%',
    samples = [],
    series = [],
    fixedMax = null,
    valueFormatter = (v) => `${v.toFixed(1)}%`,
    axisFormatter = null,
    statPills = null,
    labels = { now: 'Now', avg: 'Avg', peak: 'Peak', empty: 'No data' },
    locale = 'en'
  } = config;

  container.textContent = '';
  const card = document.createElement('section');
  card.className = 'telemetry-card';

  const head = document.createElement('div');
  head.className = 'telemetry-card-head';

  const h4 = document.createElement('h4');
  h4.className = 'telemetry-card-title';
  h4.textContent = title;

  const statsRow = document.createElement('div');
  statsRow.className = 'telemetry-stats';

  let pillsData = statPills;
  if (!pillsData) {
    const primary = series[0];
    const primaryVals = samples
      .map((s) => primary.getValue(s))
      .filter((v) => v !== null && v !== undefined && Number.isFinite(v));
    const latestVal = primaryVals.length ? primaryVals[primaryVals.length - 1] : 0;
    const avgVal = primaryVals.length ? primaryVals.reduce((a, b) => a + b, 0) / primaryVals.length : 0;
    const peakVal = primaryVals.length ? Math.max(...primaryVals) : 0;
    pillsData = [
      [labels.now, valueFormatter(latestVal)],
      [labels.avg, valueFormatter(avgVal)],
      [labels.peak, valueFormatter(peakVal)]
    ];
  }

  for (const [lbl, val] of pillsData) {
    const pill = document.createElement('span');
    pill.className = 'telemetry-stat-pill';
    const k = document.createElement('span');
    k.className = 'telemetry-stat-k';
    k.textContent = lbl;
    const v = document.createElement('strong');
    v.className = 'telemetry-stat-v mono';
    v.textContent = val;
    pill.append(k, v);
    statsRow.append(pill);
  }

  head.append(h4, statsRow);
  card.append(head);

  const legend = document.createElement('div');
  legend.className = 'telemetry-legend';
  if (series.length > 1) {
    for (const sr of series) {
      const item = document.createElement('span');
      item.className = 'legend-item';
      const dot = document.createElement('span');
      dot.className = 'legend-swatch';
      dot.style.backgroundColor = sr.color;
      const txt = document.createElement('span');
      txt.textContent = sr.label;
      item.append(dot, txt);
      legend.append(item);
    }
  }
  card.append(legend);

  if (samples.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'telemetry-empty';
    empty.textContent = labels.empty;
    card.append(empty);
    container.append(card);
    return;
  }

  const wrap = document.createElement('div');
  wrap.className = 'detail-chart-wrap';

  const hoverBox = document.createElement('div');
  hoverBox.className = 'chart-hover-readout mono';
  hoverBox.hidden = true;
  wrap.append(hoverBox);

  const W = 620;
  const H = 190;
  const padL = unit === '%' ? 46 : 60;
  const padR = 14;
  const padT = 14;
  const padB = 26;
  const plotW = W - padL - padR;
  const plotH = H - padT - padB;

  let maxVal = fixedMax;
  if (!maxVal || maxVal <= 0) {
    let observed = 0;
    for (const sr of series) {
      for (const s of samples) {
        const val = sr.getValue(s);
        if (val !== null && val !== undefined && val > observed) observed = val;
      }
    }
    maxVal = observed > 0 ? observed * 1.15 : 100;
  }

  const svg = svgEl('svg', { viewBox: `0 0 ${W} ${H}`, class: 'detail-chart-svg' });
  const defs = svgEl('defs');
  const uid = ++chartUid;

  series.forEach((sr, idx) => {
    const grad = svgEl('linearGradient', { id: `dc-grad-${uid}-${idx}`, x1: '0', y1: '0', x2: '0', y2: '1' });
    grad.append(
      svgEl('stop', { offset: '0%', 'stop-color': sr.color, 'stop-opacity': idx === 0 ? '0.28' : '0.16' }),
      svgEl('stop', { offset: '100%', 'stop-color': sr.color, 'stop-opacity': '0.01' })
    );
    defs.append(grad);
  });
  svg.append(defs);

  for (let step = 0; step <= 4; step++) {
    const frac = step / 4;
    const y = padT + plotH * (1 - frac);
    svg.append(svgEl('line', {
      x1: padL, y1: y, x2: padL + plotW, y2: y,
      class: 'chart-gridline'
    }));
    const lbl = svgEl('text', {
      x: padL - 7, y: y + 4,
      'text-anchor': 'end',
      class: 'chart-axis-text'
    });
    const yVal = maxVal * frac;
    if (axisFormatter) {
      lbl.textContent = axisFormatter(yVal);
    } else if (unit === '%') {
      lbl.textContent = `${Math.round(yVal)}%`;
    } else {
      lbl.textContent = fmtAxisRate(yVal);
    }
    svg.append(lbl);
  }

  const ptsInput = samples.length === 1 ? [samples[0], samples[0]] : samples;
  const xForIdx = (i) => padL + (i / (ptsInput.length - 1)) * plotW;
  const yForVal = (v) => padT + plotH - (Math.max(0, Math.min(maxVal, v)) / maxVal) * plotH;

  series.forEach((sr, sIdx) => {
    const segCoords = [];
    for (let i = 0; i < ptsInput.length; i++) {
      const val = sr.getValue(ptsInput[i]);
      if (val !== null && val !== undefined && Number.isFinite(val)) {
        segCoords.push([xForIdx(i), yForVal(val)]);
      }
    }
    if (segCoords.length === 0) return;
    if (segCoords.length === 1) {
      svg.append(svgEl('circle', {
        cx: segCoords[0][0].toFixed(2),
        cy: segCoords[0][1].toFixed(2),
        r: '3.5',
        fill: sr.color
      }));
      return;
    }
    const lineD = segCoords.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0].toFixed(2)},${p[1].toFixed(2)}`).join(' ');
    const areaD = `${lineD} L${segCoords[segCoords.length - 1][0].toFixed(2)},${(padT + plotH).toFixed(2)} L${segCoords[0][0].toFixed(2)},${(padT + plotH).toFixed(2)} Z`;

    svg.append(
      svgEl('path', { d: areaD, fill: `url(#dc-grad-${uid}-${sIdx})` }),
      svgEl('path', { d: lineD, fill: 'none', stroke: sr.color, 'stroke-width': '2.2', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' })
    );
  });

  const timeFmt = new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit' });
  const xTicks = samples.length === 1 ? [0] : [0, Math.floor((samples.length - 1) / 2), samples.length - 1];
  for (const idx of new Set(xTicks)) {
    const x = samples.length === 1 ? padL + plotW / 2 : xForIdx(idx);
    const tLabel = svgEl('text', {
      x,
      y: H - 6,
      'text-anchor': idx === 0 ? 'start' : idx === samples.length - 1 ? 'end' : 'middle',
      class: 'chart-axis-text'
    });
    tLabel.textContent = timeFmt.format(new Date(samples[idx].ts * 1000));
    svg.append(tLabel);
  }

  const guideLine = svgEl('line', {
    x1: 0, y1: padT, x2: 0, y2: padT + plotH,
    class: 'chart-crosshair',
    visibility: 'hidden'
  });
  svg.append(guideLine);

  const dots = series.map((sr) => {
    const d = svgEl('circle', { r: '4', fill: sr.color, stroke: '#ffffff', 'stroke-width': '1.5', visibility: 'hidden' });
    svg.append(d);
    return d;
  });

  const handleMove = (clientX) => {
    const rect = svg.getBoundingClientRect();
    if (!rect.width) return;
    const relX = ((clientX - rect.left) / rect.width) * W;
    const clampedX = Math.max(padL, Math.min(padL + plotW, relX));
    const ratio = (clampedX - padL) / plotW;
    const idx = Math.round(ratio * (samples.length - 1));
    const sample = samples[Math.max(0, Math.min(samples.length - 1, idx))];
    const snapX = samples.length === 1 ? padL + plotW / 2 : xForIdx(idx);

    guideLine.setAttribute('x1', String(snapX));
    guideLine.setAttribute('x2', String(snapX));
    guideLine.setAttribute('visibility', 'visible');

    const parts = [timeFmt.format(new Date(sample.ts * 1000))];
    series.forEach((sr, sIdx) => {
      const val = sr.getValue(sample);
      if (val === null || val === undefined || !Number.isFinite(val)) {
        dots[sIdx].setAttribute('visibility', 'hidden');
      } else {
        dots[sIdx].setAttribute('cx', String(snapX));
        dots[sIdx].setAttribute('cy', String(yForVal(val)));
        dots[sIdx].setAttribute('visibility', 'visible');
        parts.push(`${sr.shortLabel || sr.label}: ${valueFormatter(val)}`);
      }
    });
    hoverBox.textContent = parts.join('  ·  ');
    hoverBox.hidden = false;
  };

  const hideHover = () => {
    guideLine.setAttribute('visibility', 'hidden');
    dots.forEach((d) => d.setAttribute('visibility', 'hidden'));
    hoverBox.hidden = true;
  };

  svg.addEventListener('mousemove', (e) => handleMove(e.clientX));
  svg.addEventListener('touchmove', (e) => {
    if (e.touches && e.touches[0]) handleMove(e.touches[0].clientX);
  }, { passive: true });
  svg.addEventListener('mouseleave', hideHover);
  svg.addEventListener('touchend', hideHover);

  wrap.append(svg);
  card.append(wrap);
  container.append(card);
}
