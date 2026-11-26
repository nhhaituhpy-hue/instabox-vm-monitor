import { LANGUAGES, detectInitialLang, tr } from './i18n.js';
import { buildMiniAreaChart, fmtBytes, fmtRate, renderDetailChart } from './charts.js';

const SVG_NS = 'http://www.w3.org/2000/svg';

function svgIcon(paths, viewBox = '0 0 24 24', fill = 'none') {
  const s = document.createElementNS(SVG_NS, 'svg');
  s.setAttribute('class', 'ui-icon');
  s.setAttribute('width', '16');
  s.setAttribute('height', '16');
  s.setAttribute('viewBox', viewBox);
  s.setAttribute('fill', fill);
  s.setAttribute('stroke', 'currentColor');
  s.setAttribute('stroke-width', '2');
  s.setAttribute('stroke-linecap', 'round');
  s.setAttribute('stroke-linejoin', 'round');
  s.setAttribute('aria-hidden', 'true');
  for (const def of paths) {
    const el = document.createElementNS(SVG_NS, def.tag || 'path');
    for (const [k, v] of Object.entries(def)) {
      if (k !== 'tag') el.setAttribute(k, String(v));
    }
    s.append(el);
  }
  return s;
}

const ICONS = {
  menu: () => svgIcon([
    { d: 'M4 7h16' },
    { d: 'M4 12h16' },
    { d: 'M4 17h16' }
  ]),
  wake: () => svgIcon([{ tag: 'polygon', points: '13 2 3 14 12 14 11 22 21 10 12 10 13 2' }]),
  suspend: () => svgIcon([
    { tag: 'rect', x: '6', y: '4', width: '4', height: '16', rx: '1' },
    { tag: 'rect', x: '14', y: '4', width: '4', height: '16', rx: '1' }
  ]),
  start: () => svgIcon([{ tag: 'polygon', points: '6 4 20 12 6 20 6 4' }]),
  stop: () => svgIcon([{ tag: 'rect', x: '5', y: '5', width: '14', height: '14', rx: '2' }]),
  refresh: () => svgIcon([
    { d: 'M21 12a9 9 0 1 1-2.64-6.36' },
    { d: 'M21 3v6h-6' }
  ]),
  accounts: () => svgIcon([
    { d: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2' },
    { tag: 'circle', cx: '9', cy: '7', r: '4' },
    { d: 'M22 21v-2a4 4 0 0 0-3-3.87' },
    { d: 'M16 3.13a4 4 0 0 1 0 7.75' }
  ]),
  logout: () => svgIcon([
    { d: 'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4' },
    { d: 'M16 17l5-5-5-5' },
    { d: 'M21 12H9' }
  ]),
  grid: () => svgIcon([
    { tag: 'rect', x: '3', y: '3', width: '7', height: '7', rx: '1' },
    { tag: 'rect', x: '14', y: '3', width: '7', height: '7', rx: '1' },
    { tag: 'rect', x: '3', y: '14', width: '7', height: '7', rx: '1' },
    { tag: 'rect', x: '14', y: '14', width: '7', height: '7', rx: '1' }
  ]),
  cards: () => svgIcon([
    { tag: 'rect', x: '3', y: '4', width: '18', height: '7', rx: '1.5' },
    { tag: 'rect', x: '3', y: '13', width: '18', height: '7', rx: '1.5' }
  ]),
  eye: () => svgIcon([
    { d: 'M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z' },
    { tag: 'circle', cx: '12', cy: '12', r: '3' }
  ]),
  eyeOff: () => svgIcon([
    { d: 'M17.94 17.94A10.94 10.94 0 0 1 12 19c-6.5 0-10-7-10-7a18.3 18.3 0 0 1 5.06-5.94' },
    { d: 'M9.9 4.24A9.12 9.12 0 0 1 12 4c6.5 0 10 7 10 7a18.5 18.5 0 0 1-2.16 3.19' },
    { d: 'M1 1l22 22' }
  ]),
  sun: () => svgIcon([
    { tag: 'circle', cx: '12', cy: '12', r: '4' },
    { d: 'M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M4.93 19.07l1.41-1.41m11.32-11.32 1.41-1.41' }
  ]),
  moon: () => svgIcon([{ d: 'M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z' }]),
  monitor: () => svgIcon([
    { tag: 'rect', x: '2', y: '3', width: '20', height: '14', rx: '2' },
    { d: 'M8 21h8m-4-4v4' }
  ]),
  close: () => svgIcon([{ d: 'M18 6 6 18M6 6l12 12' }]),
  trash: () => svgIcon([
    { d: 'M3 6h18' },
    { d: 'M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2' },
    { d: 'M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6' }
  ]),
  edit: () => svgIcon([
    { d: 'M12 20h9' },
    { d: 'M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z' }
  ]),
  clock: () => svgIcon([
    { tag: 'circle', cx: '12', cy: '12', r: '10' },
    { d: 'M12 6v6l4 2' }
  ]),
  plus: () => svgIcon([
    { d: 'M12 5v14' },
    { d: 'M5 12h14' }
  ]),
  check: () => svgIcon([{ tag: 'polyline', points: '20 6 9 17 4 12' }]),
  key: () => svgIcon([
    { tag: 'circle', cx: '7.5', cy: '15.5', r: '4.5' },
    { d: 'M10.7 12.3 19 4' },
    { d: 'M16 7l3 3' },
    { d: 'M18 5l2 2' }
  ]),
  copy: () => svgIcon([
    { tag: 'rect', x: '9', y: '9', width: '13', height: '13', rx: '2' },
    { d: 'M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1' }
  ]),
  terminal: () => svgIcon([
    { tag: 'polyline', points: '4 17 10 11 4 5' },
    { d: 'M12 19h8' }
  ]),
  expand: () => svgIcon([
    { tag: 'polyline', points: '15 3 21 3 21 9' },
    { tag: 'polyline', points: '9 21 3 21 3 15' },
    { d: 'M21 3l-7 7' },
    { d: 'M3 21l7-7' }
  ]),
  compress: () => svgIcon([
    { tag: 'polyline', points: '4 14 10 14 10 20' },
    { tag: 'polyline', points: '20 10 14 10 14 4' },
    { d: 'M14 10l7-7' },
    { d: 'M3 21l7-7' }
  ])
};

const state = {
  lang: detectInitialLang(),
  theme: readTheme(),
  needsSetup: false,
  authenticated: false,
  username: '',
  csrfToken: '',
  clockOffsetMs: 0,
  serverDriftMs: 0,
  accounts: [],
  machines: [],
  machineOptionsCache: new Map(),
  viewMode: readPref('ibmon-view', 'grid'),
  filterAccount: 'all',
  filterState: 'all',
  searchQuery: '',
  pendingActions: new Set(),
  telemetryTarget: null,
  telemetryRange: '1h',
  telemetrySamples: [],
  editingAccountId: null
};

const vmCardRegistry = new Map();

function readPref(key, def) {
  try {
    return localStorage.getItem(key) || def;
  } catch {
    return def;
  }
}

function writePref(key, val) {
  try {
    localStorage.setItem(key, val);
  } catch {}
}

function readTheme() {
  try {
    const v = localStorage.getItem('color-scheme');
    return v === 'light' || v === 'dark' ? v : 'system';
  } catch {
    return 'system';
  }
}

function applyTheme(mode) {
  state.theme = mode;
  const meta = document.querySelector('meta[name="color-scheme"]');
  if (mode === 'light' || mode === 'dark') {
    if (meta) meta.content = mode;
    writePref('color-scheme', mode);
  } else {
    if (meta) meta.content = 'light dark';
    try { localStorage.removeItem('color-scheme'); } catch {}
  }
}

function t(key, vars) {
  return tr(state.lang, key, vars);
}

function upstreamNowMs() {
  return Date.now() + state.serverDriftMs + state.clockOffsetMs;
}

function fmtHMS(totalSeconds) {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(sec).padStart(2, '0')}`;
}

// Mirrors the backend window logic for the countdown label (schedule tz fixed UTC+7).
function scheduleNextTransition(vm) {
  if (!vm.scheduleEnabled || !vm.scheduleWindows) return null;
  let wins;
  try { wins = JSON.parse(vm.scheduleWindows); } catch { return null; }
  if (!Array.isArray(wins) || wins.length === 0) return null;
  const offsetMin = vm.scheduleTzOffsetMin || 420;
  const nowUtcMin = Math.floor(upstreamNowMs() / 60000);
  const shifted = ((nowUtcMin + offsetMin) % 1440 + 1440) % 1440;
  const parsed = wins.map((w) => {
    const [sh, sm] = String(w.start).split(':').map(Number);
    let [eh, em] = String(w.end).split(':').map(Number);
    let endMin = eh * 60 + em;
    if (endMin === 0) endMin = 1440;
    return { s: sh * 60 + sm, e: endMin };
  });
  const inside = parsed.find((w) => shifted >= w.s && shifted < w.e);
  if (inside) {
    const endMin = inside.e;
    return {
      kind: 'stop',
      inMin: inside.e - shifted,
      hhmm: `${String(Math.floor(endMin / 60) % 24).padStart(2, '0')}:${String(endMin % 60).padStart(2, '0')}`,
      tomorrow: endMin >= 1440
    };
  }
  let best = Infinity;
  let bestStart = 0;
  for (const w of parsed) {
    const d = shifted < w.s ? w.s - shifted : 1440 - shifted + w.s;
    if (d < best) { best = d; bestStart = w.s; }
  }
  if (!Number.isFinite(best)) return null;
  return {
    kind: 'start',
    inMin: best,
    hhmm: `${String(Math.floor(bestStart / 60)).padStart(2, '0')}:${String(bestStart % 60).padStart(2, '0')}`,
    tomorrow: best > (1440 - shifted)
  };
}

function fmtCompactDuration(totalSeconds) {
  const s = Math.max(0, Math.floor(totalSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  return `${s}s`;
}

function setTextIfChanged(node, next) {
  const s = String(next);
  if (node && node.textContent !== s) node.textContent = s;
}

function el(tag, attrs = {}, children = []) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'className') node.className = v;
    else if (k === 'textContent') node.textContent = v;
    else if (k.startsWith('on') && typeof v === 'function') node.addEventListener(k.slice(2).toLowerCase(), v);
    else if (k === 'dataset' && typeof v === 'object') {
      for (const [dk, dv] of Object.entries(v)) node.dataset[dk] = String(dv);
    } else node.setAttribute(k, v === true ? '' : String(v));
  }
  for (const child of Array.isArray(children) ? children : [children]) {
    if (child !== null && child !== undefined) node.append(child);
  }
  return node;
}

const liveAnnouncer = (() => {
  const div = document.createElement('div');
  div.setAttribute('aria-live', 'polite');
  div.className = 'visually-hidden';
  document.body.append(div);
  return div;
})();

const toastStack = (() => {
  const div = document.createElement('div');
  div.className = 'toast-stack';
  document.body.append(div);
  return div;
})();

function showToast(message, kind = 'info') {
  liveAnnouncer.textContent = message;
  const item = el('div', { className: 'toast', dataset: { kind }, textContent: message });
  toastStack.append(item);
  setTimeout(() => item.remove(), 4200);
}

async function apiFetch(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (options.body && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }
  const method = (options.method || 'GET').toUpperCase();
  if (method !== 'GET' && method !== 'HEAD' && state.csrfToken) {
    headers.set('X-CSRF-Token', state.csrfToken);
  }
  const res = await fetch(path, { ...options, headers, credentials: 'same-origin' });
  let data = null;
  try {
    data = await res.json();
  } catch {}
  if (!res.ok) {
    const errObj = data?.error || {};
    const err = new Error(errObj.message || `${t('errGeneric')} (${res.status})`);
    err.status = res.status;
    err.code = errObj.code || '';
    err.retryAfterSeconds = errObj.retryAfterSeconds || 0;
    throw err;
  }
  return data;
}

function getVmToneAndLabel(vm) {
  const now = upstreamNowMs();
  if (vm.cooldownUntil) {
    const cd = Date.parse(vm.cooldownUntil);
    if (Number.isFinite(cd) && cd > now) {
      return { tone: 'error', label: t('statusCooldown'), title: '' };
    }
  }
  switch (vm.state) {
    case 'running': {
      // instabox.sh may leave runtime.ready=false on snapshot-resumed or headless VMs
      // even after wake/start completes and exec/SSH work. Show booting only while an
      // operation is still active or within a short 30s cold-boot grace window.
      if (!vm.ready) {
        if (vm.operationId) {
          return { tone: 'booting', label: t('statusBooting'), title: '' };
        }
        const started = vm.runStartedAt ? Date.parse(vm.runStartedAt) : NaN;
        if (!vm.processesPreserved && Number.isFinite(started) && now - started < 30000) {
          return { tone: 'booting', label: t('statusBooting'), title: '' };
        }
        return { tone: 'running', label: t('statusRunning'), title: t('statusUnreadyNote') };
      }
      return { tone: 'running', label: t('statusRunning'), title: '' };
    }
    case 'starting':
    case 'resuming':
    case 'creating':
      return { tone: 'booting', label: t('statusBooting'), title: '' };
    case 'suspended':
      return vm.autoSuspendedAt
        ? { tone: 'idle', label: t('statusIdleSuspended'), title: '' }
        : { tone: 'suspended', label: t('statusSuspended'), title: '' };
    case 'stopping':
    case 'suspending':
    case 'deleting':
      return { tone: 'stopping', label: t('statusStopping'), title: '' };
    case 'error':
      return { tone: 'error', label: t('statusError'), title: '' };
    default:
      return { tone: 'stopped', label: t('statusStopped'), title: '' };
  }
}

function formatClockDisplay(vm) {
  const now = upstreamNowMs();
  if (vm.cooldownUntil) {
    const cd = Date.parse(vm.cooldownUntil);
    if (Number.isFinite(cd) && cd > now) {
      return { text: fmtHMS((cd - now) / 1000), mode: 'cooldown' };
    }
  }
  if (vm.state === 'running' && vm.runStartedAt) {
    const started = Date.parse(vm.runStartedAt);
    if (Number.isFinite(started)) {
      return { text: fmtHMS(Math.max(0, (now - started) / 1000)), mode: 'running' };
    }
  }
  return { text: '--:--:--', mode: 'idle' };
}

function updateLiveClocks() {
  const nodes = document.querySelectorAll('[data-clock-vm]');
  for (const node of nodes) {
    const key = node.getAttribute('data-clock-vm');
    const vm = state.machines.find((m) => `${m.accountId}:${m.id}` === key);
    if (!vm) continue;
    const { text, mode } = formatClockDisplay(vm);
    if (node.textContent !== text) node.textContent = text;
    if (node.dataset.mode !== mode) node.dataset.mode = mode;
  }
}

function createPasswordField({ id, name, label, hint, autocomplete, errorText, required = true, minlength = 8 }) {
  const field = el('div', { className: 'field', dataset: { invalid: 'false' } });
  const lbl = el('label', { for: id, textContent: label });
  field.append(lbl);
  if (hint) {
    field.append(el('span', { id: `${id}-hint`, className: 'hint', textContent: hint }));
  }

  const wrap = el('div', { className: 'input-wrap' });
  const input = el('input', {
    className: 'input input-with-toggle',
    id,
    name,
    type: 'password',
    autocomplete,
    required,
    minlength: minlength || undefined,
    'aria-invalid': 'false',
    'aria-describedby': hint ? `${id}-hint` : undefined
  });

  const toggleBtn = el('button', {
    type: 'button',
    className: 'pw-toggle',
    'aria-pressed': 'false',
    'aria-label': t('showPassword'),
    onClick: () => {
      const showing = input.type === 'text';
      input.type = showing ? 'password' : 'text';
      toggleBtn.setAttribute('aria-pressed', showing ? 'false' : 'true');
      toggleBtn.setAttribute('aria-label', showing ? t('showPassword') : t('hidePassword'));
      toggleBtn.replaceChildren(showing ? ICONS.eye() : ICONS.eyeOff());
    }
  }, [ICONS.eye()]);

  wrap.append(input, toggleBtn);
  const errEl = el('div', { id: `${id}-err`, className: 'error-msg', textContent: errorText });
  field.append(wrap, errEl);

  const sync = (forceCheck = false) => {
    const invalid =
      (forceCheck ? !input.checkValidity() : input.matches(':user-invalid')) ||
      field.dataset.customInvalid === 'true';
    input.setAttribute('aria-invalid', invalid ? 'true' : 'false');
    field.dataset.invalid = invalid ? 'true' : 'false';
    const ids = [hint ? `${id}-hint` : '', invalid ? `${id}-err` : ''].filter(Boolean);
    if (ids.length) input.setAttribute('aria-describedby', ids.join(' '));
    else input.removeAttribute('aria-describedby');
    return !invalid;
  };
  input.addEventListener('blur', () => sync(false));
  input.addEventListener('input', () => {
    delete field.dataset.customInvalid;
    input.setCustomValidity('');
    if (field.dataset.invalid === 'true' || input.getAttribute('aria-invalid') === 'true') sync(false);
  });
  return { field, input, sync };
}

// ============================================================
// Corner Hamburger Navigation Drawer (modern-web-guidance: navigation-drawer)
// ============================================================
let navDrawer = null;
let drawerScroller = null;
let drawerSheet = null;
let drawerObserver = null;
let drawerState = 'closed'; // 'closed' | 'opening' | 'open' | 'closing'

function syncHamburgerExpanded(expanded) {
  const openBtn = document.getElementById('drawer-open');
  if (openBtn) openBtn.setAttribute('aria-expanded', expanded ? 'true' : 'false');
}

function ensureNavDrawer() {
  if (navDrawer) return navDrawer;

  navDrawer = el('dialog', {
    className: 'Drawer',
    id: 'nav-drawer',
    'aria-label': t('menuOpen')
  });
  drawerScroller = el('div', { className: 'Drawer-scroller' });
  drawerSheet = el('nav', { className: 'Drawer-sheet' });

  drawerScroller.append(drawerSheet);
  navDrawer.append(drawerScroller);
  document.body.append(navDrawer);

  const visibleThreshold = 1 / Math.max(window.innerWidth, 320);
  drawerObserver = new IntersectionObserver(
    (entries) => {
      const entry = entries.at(-1);
      if (!entry || !navDrawer.open) return;
      if (entry.intersectionRatio >= 0.1 && drawerState === 'opening') {
        drawerState = 'open';
      }
      if (entry.intersectionRatio >= 0.98) {
        drawerState = 'open';
        syncHamburgerExpanded(true);
      }
      if (entry.intersectionRatio < visibleThreshold && (drawerState === 'open' || drawerState === 'closing')) {
        drawerState = 'closed';
        navDrawer.close();
        syncHamburgerExpanded(false);
      }
    },
    { root: navDrawer, threshold: [visibleThreshold, 0.1, 1] }
  );
  drawerObserver.observe(drawerSheet);

  // Fallback for browsers without scroll-driven animations
  if (!CSS.supports('animation-timeline: scroll()')) {
    drawerScroller.addEventListener('scroll', () => {
      const w = drawerSheet.offsetWidth || 1;
      const ratio = Math.max(0, Math.min(1, 1 - drawerScroller.scrollLeft / w));
      navDrawer.style.setProperty('--drawer-backdrop', String(ratio));
    }, { passive: true });
  }

  // Light dismiss on backdrop tap outside the sheet
  navDrawer.addEventListener('click', (event) => {
    if (event.target.isConnected && !drawerSheet.contains(event.target)) {
      closeDrawer();
    }
  });

  // Escape key fires cancel -> animate slide-out instead of sudden snap
  navDrawer.addEventListener('cancel', (event) => {
    if (event.cancelable) {
      event.preventDefault();
      closeDrawer();
    }
  });

  navDrawer.addEventListener('close', () => {
    drawerState = 'closed';
    syncHamburgerExpanded(false);
  });

  renderDrawerContent();
  return navDrawer;
}

async function openDrawer() {
  ensureNavDrawer();
  renderDrawerContent();
  drawerState = 'opening';
  if (!navDrawer.open) {
    navDrawer.showModal();
  }
  if (!CSS.supports('scroll-initial-target', 'nearest')) {
    drawerScroller.scrollTo({ left: drawerScroller.offsetWidth, behavior: 'instant' });
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
  }
  drawerScroller.scrollTo({ left: 0, behavior: 'auto' });
  syncHamburgerExpanded(true);
}

function closeDrawer() {
  if (!navDrawer || !navDrawer.open) return;
  drawerState = 'closing';
  drawerScroller.scrollTo({ left: drawerScroller.offsetWidth, behavior: 'auto' });
  setTimeout(() => {
    if (navDrawer.open && drawerState === 'closing' && drawerScroller.scrollLeft >= (drawerSheet.offsetWidth - 4)) {
      drawerState = 'closed';
      navDrawer.close();
      syncHamburgerExpanded(false);
    }
  }, 420);
}

let syncDoneTimer = null;
async function triggerSyncAll() {
  if (state.syncing) return;
  state.syncing = true;
  clearTimeout(syncDoneTimer);
  const syncBtn = document.getElementById('sync-all-btn');
  if (syncBtn) {
    delete syncBtn.dataset.syncDone;
    syncBtn.dataset.syncing = 'true';
    syncBtn.setAttribute('aria-busy', 'true');
  }
  const startedAt = performance.now();
  try {
    state.machineOptionsCache.clear();
    const data = await apiFetch('/api/sync', { method: 'POST' });
    const elapsed = performance.now() - startedAt;
    if (elapsed < 450) {
      await new Promise((r) => setTimeout(r, 450 - elapsed));
    }
    applyMachinesPayload(data);
    updateDashboardInPlace();
    renderDrawerContent();
    const btnAfter = document.getElementById('sync-all-btn');
    if (btnAfter) {
      delete btnAfter.dataset.syncing;
      btnAfter.removeAttribute('aria-busy');
      btnAfter.dataset.syncDone = 'true';
      syncDoneTimer = setTimeout(() => {
        const cur = document.getElementById('sync-all-btn');
        if (!cur || cur.dataset.syncing === 'true') return;
        delete cur.dataset.syncDone;
      }, 1150);
    }
  } catch (err) {
    const btnAfter = document.getElementById('sync-all-btn');
    if (btnAfter) {
      delete btnAfter.dataset.syncing;
      btnAfter.removeAttribute('aria-busy');
    }
    showToast(err.message, 'error');
  } finally {
    state.syncing = false;
  }
}

function renderDrawerContent() {
  if (!drawerSheet) return;
  navDrawer.setAttribute('aria-label', t('menuOpen'));

  const closeBtn = el('button', {
    id: 'drawer-close',
    type: 'button',
    className: 'btn btn-icon-only',
    'aria-label': t('closeMenu'),
    autofocus: true,
    onClick: closeDrawer
  }, [ICONS.close()]);

  const head = el('div', { className: 'drawer-head' }, [
    el('div', { className: 'drawer-brand' }, [
      el('img', { className: 'brand-logo', src: '/assets/logo.svg', alt: '', width: '32', height: '32' }),
      el('div', { style: 'min-inline-size:0;' }, [
        el('div', { className: 'brand-title', textContent: t('appName') }),
        state.authenticated && state.username
          ? el('div', { className: 'drawer-user mono', textContent: `@${state.username}` })
          : el('div', { className: 'drawer-user', textContent: t('appSubtitle') })
      ])
    ]),
    closeBtn
  ]);

  const body = el('div', { className: 'drawer-body' });

  if (state.authenticated) {
    body.append(
      el('button', {
        type: 'button',
        className: 'drawer-item',
        onClick: () => {
          closeDrawer();
          setTimeout(() => openAddMachineDialog(), 160);
        }
      }, [
        ICONS.plus(),
        el('span', { textContent: t('addMachineBtn') })
      ]),
      el('button', {
        type: 'button',
        className: 'drawer-item',
        onClick: () => {
          closeDrawer();
          setTimeout(() => openAccountsDialog(), 160);
        }
      }, [
        ICONS.accounts(),
        el('span', { textContent: t('accountsBtn') }),
        el('span', { className: 'drawer-item-badge', textContent: String(state.accounts.length) })
      ]),
      el('button', {
        type: 'button',
        className: 'drawer-item',
        onClick: () => {
          closeDrawer();
          setTimeout(() => openAPIKeysDialog(), 160);
        }
      }, [
        ICONS.key(),
        el('span', { textContent: t('apiKeysBtn') })
      ]),
      el('button', {
        type: 'button',
        className: 'drawer-item',
        onClick: () => {
          closeDrawer();
          triggerSyncAll();
        }
      }, [
        ICONS.refresh(),
        el('span', { textContent: t('refreshAll') })
      ]),
      el('hr', { className: 'drawer-sep' })
    );
  }

  // Appearance segmented selector (updates aria-pressed in-place without detaching buttons)
  body.append(el('div', { className: 'drawer-section-label', textContent: t('themeLabel') }));
  const themeGroup = el('div', { className: 'drawer-theme-group', role: 'group', 'aria-label': t('themeLabel') });
  const themeButtons = [];
  for (const [mode, iconNode, lbl] of [
    ['system', ICONS.monitor(), t('themeSystem')],
    ['light', ICONS.sun(), t('themeLight')],
    ['dark', ICONS.moon(), t('themeDark')]
  ]) {
    const tBtn = el('button', {
      type: 'button',
      className: 'drawer-theme-btn',
      'aria-pressed': state.theme === mode ? 'true' : 'false',
      onClick: () => {
        applyTheme(mode);
        for (const item of themeButtons) {
          item.btn.setAttribute('aria-pressed', item.mode === mode ? 'true' : 'false');
        }
      }
    }, [iconNode, el('span', { textContent: lbl })]);
    themeButtons.push({ mode, btn: tBtn });
    themeGroup.append(tBtn);
  }
  body.append(themeGroup);

  // Language selector (11 languages)
  body.append(el('label', {
    className: 'drawer-section-label',
    for: 'drawer-lang-select',
    style: 'margin-block-start:0.55rem;',
    textContent: t('language')
  }));
  const langSelect = el('select', {
    id: 'drawer-lang-select',
    className: 'drawer-select',
    onChange: (e) => {
      state.lang = e.target.value;
      document.documentElement.lang = state.lang;
      writePref('ibmon-lang', state.lang);
      render();
      renderDrawerContent();
    }
  });
  for (const l of LANGUAGES) {
    const opt = el('option', { value: l.code, textContent: l.label });
    if (l.code === state.lang) opt.selected = true;
    langSelect.append(opt);
  }
  body.append(langSelect);

  const footer = el('div', { className: 'drawer-footer' });
  if (state.authenticated) {
    footer.append(el('button', {
      type: 'button',
      className: 'drawer-item btn-danger',
      style: 'border: 1px solid color-mix(in srgb, var(--danger) 35%, var(--border));',
      onClick: async () => {
        closeDrawer();
        try {
          await apiFetch('/api/auth/logout', { method: 'POST' });
        } catch {}
        closeOpenDialogs();
        state.authenticated = false;
        state.csrfToken = '';
        state.accounts = [];
        state.machines = [];
        state.machineOptionsCache.clear();
        vmCardRegistry.clear();
        render();
      }
    }, [ICONS.logout(), el('span', { textContent: t('logout') })]));
  }

  drawerSheet.replaceChildren(head, body, footer);
}

function renderTopbar() {
  const header = el('header', { className: 'topbar' });
  const inner = el('div', { className: 'topbar-inner' });

  const menuBtn = el('button', {
    id: 'drawer-open',
    type: 'button',
    className: 'hamburger-btn',
    'aria-label': t('menuOpen'),
    'aria-expanded': navDrawer?.open ? 'true' : 'false',
    'aria-controls': 'nav-drawer',
    onClick: () => openDrawer()
  }, [ICONS.menu()]);

  const left = el('div', { className: 'topbar-left' }, [menuBtn]);
  inner.append(left);

  header.append(inner);
  return header;
}

function renderAuthView() {
  const shell = el('main', { className: 'auth-shell' });
  const card = el('section', { className: 'auth-card' });

  const isSetup = state.needsSetup;
  const head = el('div', { className: 'auth-head' }, [
    el('img', { src: '/assets/logo.svg', alt: '', width: '40', height: '40' }),
    el('h1', { className: 'auth-title', textContent: isSetup ? t('setupTitle') : t('loginTitle') })
  ]);
  const desc = el('p', { className: 'auth-desc', textContent: isSetup ? t('setupDesc') : t('loginDesc') });
  const banner = el('div', { className: 'form-error-banner', role: 'alert', hidden: true });

  const form = el('form', { novalidate: true });

  const uField = el('div', { className: 'field', dataset: { invalid: 'false' } });
  uField.append(el('label', { for: 'auth-username', textContent: t('username') }));
  if (isSetup) {
    uField.append(el('span', { id: 'auth-user-hint', className: 'hint', textContent: t('usernameHint') }));
  }
  const uInput = el('input', {
    className: 'input',
    id: 'auth-username',
    name: 'username',
    type: 'text',
    autocomplete: 'username',
    autocorrect: 'off',
    autocapitalize: 'none',
    spellcheck: 'false',
    required: true,
    minlength: '2',
    maxlength: '64',
    'aria-invalid': 'false'
  });
  const uErr = el('div', { id: 'auth-user-err', className: 'error-msg', textContent: t('errUsername') });
  uField.append(uInput, uErr);
  const syncUser = () => {
    const inv = uInput.matches(':user-invalid') || !uInput.checkValidity();
    uInput.setAttribute('aria-invalid', inv ? 'true' : 'false');
    uField.dataset.invalid = inv ? 'true' : 'false';
  };
  uInput.addEventListener('blur', syncUser);
  uInput.addEventListener('input', () => {
    if (uField.dataset.invalid === 'true') syncUser();
  });
  form.append(uField);

  const pw = createPasswordField({
    id: 'auth-password',
    name: 'password',
    label: t('password'),
    hint: isSetup ? t('passwordHint') : '',
    autocomplete: isSetup ? 'new-password' : 'current-password',
    errorText: t('errPassword'),
    minlength: 8
  });
  form.append(pw.field);

  let pwConfirm = null;
  if (isSetup) {
    pwConfirm = createPasswordField({
      id: 'auth-password-confirm',
      name: 'confirmPassword',
      label: t('confirmPassword'),
      autocomplete: 'new-password',
      errorText: t('errPasswordMatch'),
      minlength: 8
    });
    form.append(pwConfirm.field);
  }

  const submitBtn = el('button', {
    type: 'submit',
    className: 'btn btn-primary',
    style: 'inline-size: 100%; min-block-size: 44px;',
    textContent: isSetup ? t('createAdminSubmit') : t('loginSubmit')
  });
  form.append(submitBtn);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    banner.hidden = true;

    syncUser();
    pw.sync();
    if (!uInput.checkValidity()) {
      uInput.focus();
      return;
    }
    if (!pw.input.checkValidity()) {
      pw.input.focus();
      return;
    }
    if (isSetup && pwConfirm) {
      if (pwConfirm.input.value !== pw.input.value) {
        pwConfirm.field.dataset.customInvalid = 'true';
        pwConfirm.input.setCustomValidity(t('errPasswordMatch'));
        pwConfirm.sync();
        pwConfirm.input.focus();
        return;
      }
    }

    if (submitBtn.getAttribute('aria-disabled') === 'true') return;
    submitBtn.setAttribute('aria-disabled', 'true');
    try {
      const endpoint = isSetup ? '/api/auth/setup' : '/api/auth/login';
      const resp = await apiFetch(endpoint, {
        method: 'POST',
        body: JSON.stringify({ username: uInput.value.trim(), password: pw.input.value })
      });
      state.needsSetup = false;
      state.authenticated = true;
      state.username = resp.username;
      state.csrfToken = resp.csrfToken;
      await loadDashboardData();
      render();
    } catch (err) {
      banner.textContent = err.message;
      banner.hidden = false;
    } finally {
      submitBtn.removeAttribute('aria-disabled');
    }
  });

  card.append(head, desc, banner, form);
  shell.append(card);
  return shell;
}

// ============================================================
// Keyed VM Action Button (updates state in-place without destroying focus)
// ============================================================
function createActionButton(initialVm, actionKey, iconNode) {
  let currentVm = initialVm;
  const tipId = `tip-${initialVm.accountId.slice(0, 6)}-${initialVm.id}-${actionKey}`.replace(/[^a-zA-Z0-9_-]/g, '');

  const slot = el('div', { className: 'vm-action-slot' });
  const tip = el('div', {
    id: tipId,
    className: 'action-tooltip',
    role: 'tooltip',
    popover: 'manual'
  });

  let openTimer = null;
  let closeTimer = null;

  const btn = el('button', {
    type: 'button',
    className: 'vm-action-btn',
    dataset: { action: actionKey },
    'aria-describedby': tipId,
    onClick: async (ev) => {
      ev.stopPropagation();
      if (btn.disabled) return;
      hideTooltip();
      const vmComposite = `${currentVm.accountId}:${currentVm.id}`;
      const pendingKey = `${vmComposite}:${actionKey}`;
      state.pendingActions.add(pendingKey);
      const ctrl = vmCardRegistry.get(vmComposite);
      if (ctrl) ctrl.update(currentVm);

      try {
        const res = await apiFetch(
          `/api/machines/${encodeURIComponent(currentVm.accountId)}/${encodeURIComponent(currentVm.id)}/actions/${actionKey}`,
          { method: 'POST' }
        );
        showToast(`${t('actionSent')}: ${currentVm.name} (${actionKey})`);
        if (res?.machine) {
          const idx = state.machines.findIndex((x) => x.accountId === currentVm.accountId && x.id === currentVm.id);
          if (idx >= 0) {
            state.machines[idx] = res.machine;
            currentVm = res.machine;
          }
        }
        setTimeout(() => refreshMachinesSilently(), 1800);
        setTimeout(() => refreshMachinesSilently(), 4500);
      } catch (err) {
        const waitTxt = err.retryAfterSeconds ? ` (${err.retryAfterSeconds}s)` : '';
        showToast(`${err.message}${waitTxt}`, 'error');
      } finally {
        state.pendingActions.delete(pendingKey);
        const latestVm = state.machines.find((x) => x.accountId === currentVm.accountId && x.id === currentVm.id) || currentVm;
        updateDashboardInPlace();
        const activeCtrl = vmCardRegistry.get(vmComposite);
        if (activeCtrl) activeCtrl.update(latestVm);
      }
    }
  }, [iconNode]);

  const positionTip = () => {
    const r = btn.getBoundingClientRect();
    const trRect = tip.getBoundingClientRect();
    let top = r.top - trRect.height - 8;
    if (top < 8) top = r.bottom + 8;
    const left = Math.max(8, Math.min(window.innerWidth - trRect.width - 8, r.left + r.width / 2 - trRect.width / 2));
    tip.style.top = `${top}px`;
    tip.style.left = `${left}px`;
  };

  const showTooltip = () => {
    clearTimeout(closeTimer);
    document.querySelectorAll('.action-tooltip').forEach((other) => {
      if (other !== tip && other.matches(':popover-open')) {
        try { other.hidePopover(); } catch {}
      }
    });
    try {
      if (typeof tip.showPopover === 'function') {
        if (!tip.matches(':popover-open')) tip.showPopover();
      }
      positionTip();
    } catch {}
  };

  const hideTooltip = () => {
    clearTimeout(openTimer);
    clearTimeout(closeTimer);
    try {
      if (typeof tip.hidePopover === 'function' && tip.matches(':popover-open')) {
        tip.hidePopover();
      }
    } catch {}
  };

  btn.addEventListener('mouseenter', () => {
    clearTimeout(openTimer);
    openTimer = setTimeout(showTooltip, 70);
  });
  btn.addEventListener('mouseleave', () => {
    clearTimeout(openTimer);
    clearTimeout(closeTimer);
    closeTimer = setTimeout(hideTooltip, 110);
  });
  btn.addEventListener('focusin', showTooltip);
  btn.addEventListener('focusout', hideTooltip);
  tip.addEventListener('mouseenter', () => clearTimeout(closeTimer));
  tip.addEventListener('mouseleave', () => {
    clearTimeout(closeTimer);
    closeTimer = setTimeout(hideTooltip, 110);
  });
  btn.addEventListener('keydown', (e) => { if (e.key === 'Escape') hideTooltip(); });

  slot.append(btn, tip);

  return {
    slot,
    hideTooltip,
    update(vm, tooltipText, enabled) {
      currentVm = vm;
      const pendingKey = `${vm.accountId}:${vm.id}:${actionKey}`;
      const isPending = state.pendingActions.has(pendingKey);
      btn.classList.toggle('is-pending', isPending);
      btn.disabled = !enabled || isPending;
      if (btn.getAttribute('aria-label') !== tooltipText) {
        btn.setAttribute('aria-label', tooltipText);
      }
      setTextIfChanged(tip, tooltipText);
    }
  };
}

function getVmMountStats(vm, mount) {
  if (vm.disk?.filesystems?.length) {
    const fs = vm.disk.filesystems.find((f) => f.mount === mount);
    if (fs) {
      const pct = Number.isFinite(fs.usedPct)
        ? fs.usedPct
        : (fs.sizeBytes > 0 && fs.usedBytes > 0 ? (fs.usedBytes / fs.sizeBytes) * 100 : 0);
      return { pct, usedBytes: fs.usedBytes || 0, sizeBytes: fs.sizeBytes || 0 };
    }
  }
  const latest = vm.latestMetric;
  if (latest) {
    if (mount === '/' && latest.rootTotal > 0 && latest.rootUsed > 0) {
      return {
        pct: (latest.rootUsed / latest.rootTotal) * 100,
        usedBytes: latest.rootUsed,
        sizeBytes: latest.rootTotal
      };
    }
    if (mount === '/data' && latest.diskTotal > 0 && latest.diskUsed > 0) {
      return {
        pct: (latest.diskUsed / latest.diskTotal) * 100,
        usedBytes: latest.diskUsed,
        sizeBytes: latest.diskTotal
      };
    }
  }
  return { pct: 0, usedBytes: 0, sizeBytes: 0 };
}

function metricsSignature(vm, rootPct, dataPct) {
  const samples = vm.recentMetrics || [];
  const last = samples.length ? samples[samples.length - 1] : null;
  return `${samples.length}|${last ? `${last.ts}:${last.cpuPct}:${last.memUsed}:${last.diskUsed}:${last.rootUsed}` : 'none'}|${rootPct.toFixed(1)}|${dataPct.toFixed(1)}`;
}

// ============================================================
// Keyed VM Card Controller (in-place DOM diffing; zero node loss)
// ============================================================
function createVmCardController(initialVm) {
  let vm = initialVm;
  const wrap = el('div', { className: 'vm-card-wrap' });
  const card = el('article', { className: 'vm-card' });

  const col1 = el('div', { className: 'vm-col-meta' });
  const head = el('div', { className: 'vm-head' });

  const statusPoint = el('span', { className: 'status-point', 'aria-hidden': 'true' });
  const nameBtn = el('button', {
    type: 'button',
    className: 'vm-name-btn',
    onClick: () => openTelemetryDialog(vm)
  });
  const terminalBtn = el('button', {
    type: 'button',
    className: 'vm-inline-btn vm-inline-terminal',
    onClick: (ev) => {
      ev.stopPropagation();
      openTerminalDialog(vm);
    }
  }, [ICONS.terminal()]);
  const renameBtn = el('button', {
    type: 'button',
    className: 'vm-inline-btn',
    onClick: (ev) => {
      ev.stopPropagation();
      openRenameMachineDialog(vm);
    }
  }, [ICONS.edit()]);
  const deleteBtn = el('button', {
    type: 'button',
    className: 'vm-inline-btn vm-inline-danger',
    onClick: (ev) => {
      ev.stopPropagation();
      openDeleteMachineDialog(vm);
    }
  }, [ICONS.trash()]);
  const nameRow = el('div', { className: 'vm-name-row' }, [nameBtn, terminalBtn, renameBtn, deleteBtn]);

  const aliasBadge = el('span', { className: 'badge' });
  const idBadge = el('span', { className: 'badge mono' });
  const tplBadge = el('span', { className: 'badge' });
  const freeBadge = el('span', { className: 'badge' });
  const specEl = el('span', { className: 'vm-spec-inline' });

  const ident = el('div', { className: 'vm-identity' }, [
    statusPoint,
    el('div', { className: 'vm-title-block' }, [
      nameRow,
      el('div', { className: 'vm-submeta vm-meta-inline' }, [aliasBadge, idBadge, tplBadge, freeBadge, specEl])
    ])
  ]);

  const keepaliveInputId = `ka-${vm.accountId}-${vm.id}`.replace(/[^a-zA-Z0-9_-]/g, '_');
  let keepaliveBusy = false;
  const keepaliveInput = el('input', {
    type: 'checkbox',
    id: keepaliveInputId,
    className: 'vm-keepalive-input'
  });
  const keepaliveText = el('span', { className: 'vm-keepalive-text' });
  const keepaliveMeta = el('span', { className: 'vm-keepalive-meta mono', hidden: true });
  const keepaliveToggle = el('label', {
    className: 'vm-keepalive-toggle',
    for: keepaliveInputId,
    dataset: { active: 'false' }
  }, [keepaliveInput, keepaliveText, keepaliveMeta]);

  // --- Time-window schedule row: toggle + "view details" link into the editor modal ---
  const schedInputId = `sch-${vm.accountId}-${vm.id}`.replace(/[^a-zA-Z0-9_-]/g, '_');
  let schedBusy = false;
  const schedEnabledInput = el('input', {
    type: 'checkbox',
    id: schedInputId,
    className: 'vm-keepalive-input'
  });
  const schedText = el('span', { className: 'vm-keepalive-text' });
  const schedMeta = el('span', { className: 'vm-keepalive-meta mono', hidden: true });
  const schedToggle = el('label', {
    className: 'vm-keepalive-toggle',
    for: schedInputId,
    dataset: { active: 'false' }
  }, [schedEnabledInput, schedText, schedMeta]);

  const saveSchedule = async (enabled, windows) => {
    if (schedBusy) return false;
    schedBusy = true;
    schedEnabledInput.disabled = true;
    try {
      const res = await apiFetch(
        `/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}/schedule`,
        { method: 'PUT', body: JSON.stringify({ enabled, windows, tzOffsetMin: 420 }) }
      );
      if (res?.machine) {
        const idx = state.machines.findIndex((x) => x.accountId === vm.accountId && x.id === vm.id);
        if (idx >= 0) state.machines[idx] = res.machine;
        if (!updateDashboardInPlace()) update(res.machine);
      }
      showToast(`${t(enabled ? 'schedEnabledToast' : 'schedDisabledToast')}: ${vm.name}`);
      setTimeout(() => refreshMachinesSilently(), 1200);
      return true;
    } catch (err) {
      showToast(err.message, 'error');
      return false;
    } finally {
      schedBusy = false;
      schedEnabledInput.disabled = false;
    }
  };

  schedEnabledInput.addEventListener('change', async () => {
    const wantOn = schedEnabledInput.checked;
    if (wantOn && !vm.scheduleWindows) {
      // Nothing configured yet: the modal is the place to define the first window.
      schedEnabledInput.checked = false;
      openScheduleDialog(vm);
      return;
    }
    const ok = await saveSchedule(wantOn, vm.scheduleWindows || '');
    if (!ok) schedEnabledInput.checked = !wantOn;
  });

  const schedDetailsBtn = el('button', {
    type: 'button',
    className: 'vm-sched-details',
    onClick: () => openScheduleDialog(vm)
  });

  const schedRow = el('div', { className: 'vm-sched-row' }, [
    schedToggle, schedDetailsBtn
  ]);

  keepaliveInput.addEventListener('change', async () => {
    if (keepaliveBusy) return;
    const nextEnabled = keepaliveInput.checked;
    keepaliveBusy = true;
    keepaliveInput.disabled = true;
    keepaliveToggle.dataset.busy = 'true';
    try {
      const res = await apiFetch(
        `/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}/keepalive`,
        { method: 'PATCH', body: JSON.stringify({ enabled: nextEnabled }) }
      );
      keepaliveBusy = false;
      if (res?.machine) {
        const idx = state.machines.findIndex((x) => x.accountId === vm.accountId && x.id === vm.id);
        if (idx >= 0) state.machines[idx] = res.machine;
        if (!updateDashboardInPlace()) update(res.machine);
      }
      showToast(`${t(nextEnabled ? 'keepaliveEnabledToast' : 'keepaliveDisabledToast')}: ${vm.name}`);
      if (nextEnabled) {
        setTimeout(() => refreshMachinesSilently(), 1200);
        setTimeout(() => refreshMachinesSilently(), 4500);
      }
    } catch (err) {
      keepaliveBusy = false;
      keepaliveInput.checked = !nextEnabled;
      keepaliveToggle.dataset.active = !nextEnabled ? 'true' : 'false';
      showToast(err.message, 'error');
    } finally {
      keepaliveBusy = false;
      keepaliveInput.disabled = false;
      delete keepaliveToggle.dataset.busy;
    }
  });

  const statusBadge = el('span', { className: 'badge' });
  head.append(ident, keepaliveToggle, statusBadge);
  const errEl = el('div', { className: 'account-err', hidden: true });
  col1.append(head, errEl);

  const clockLabelEl = el('span', { className: 'hint' });
  const clockValueEl = el('span', {
    className: 'clock-value mono',
    dataset: { clockVm: `${vm.accountId}:${vm.id}` }
  });
  const clockLimitEl = el('span', { className: 'clock-limit mono' });
  const clockLeft = el('div', { className: 'clock-group' }, [
    el('span', { className: 'clock-label-wrap' }, [ICONS.clock(), clockLabelEl]),
    el('span', { className: 'clock-readout-wrap' }, [clockValueEl, clockLimitEl])
  ]);
  const clockRight = el('div', { className: 'clock-limit mono' });
  const clockRow = el('div', { className: 'vm-clock-row' }, [clockLeft, clockRight]);
  col1.append(schedRow);

  // Column 2: Mini Beszel Charts (CPU, RAM, System (/), Data (/data))
  const cpuLbl = el('span', { className: 'mini-chart-lbl' });
  const cpuVal = el('span', { className: 'mini-chart-val mono' });
  const cpuSvgHost = el('div');

  const ramLbl = el('span', { className: 'mini-chart-lbl' });
  const ramVal = el('span', { className: 'mini-chart-val mono' });
  const ramSvgHost = el('div');

  const rootLbl = el('span', { className: 'mini-chart-lbl' });
  const rootVal = el('span', { className: 'mini-chart-val mono' });
  const rootSvgHost = el('div');
  const rootBox = el('button', { type: 'button', className: 'mini-chart-box', onClick: () => openTelemetryDialog(vm) }, [
    el('div', { className: 'mini-chart-top' }, [rootLbl, rootVal]),
    rootSvgHost
  ]);

  const dataLbl = el('span', { className: 'mini-chart-lbl' });
  const dataVal = el('span', { className: 'mini-chart-val mono' });
  const dataSvgHost = el('div');
  const dataBox = el('button', { type: 'button', className: 'mini-chart-box', onClick: () => openTelemetryDialog(vm) }, [
    el('div', { className: 'mini-chart-top' }, [dataLbl, dataVal]),
    dataSvgHost
  ]);

  const miniGrid = el('div', { className: 'mini-charts-grid' }, [
    el('button', { type: 'button', className: 'mini-chart-box', onClick: () => openTelemetryDialog(vm) }, [
      el('div', { className: 'mini-chart-top' }, [cpuLbl, cpuVal]),
      cpuSvgHost
    ]),
    el('button', { type: 'button', className: 'mini-chart-box', onClick: () => openTelemetryDialog(vm) }, [
      el('div', { className: 'mini-chart-top' }, [ramLbl, ramVal]),
      ramSvgHost
    ]),
    rootBox,
    dataBox
  ]);

  // Column 3: 4 Icon-Only SVG Action Buttons
  const wakeCtrl = createActionButton(vm, 'wake', ICONS.wake());
  const suspendCtrl = createActionButton(vm, 'suspend', ICONS.suspend());
  const startCtrl = createActionButton(vm, 'start', ICONS.start());
  const stopCtrl = createActionButton(vm, 'stop', ICONS.stop());
  const actionsBar = el('div', { className: 'vm-actions' }, [
    wakeCtrl.slot, suspendCtrl.slot, startCtrl.slot, stopCtrl.slot
  ]);

  const metricsRow = el('div', { className: 'vm-card-metrics-row' }, [
    clockRow,
    miniGrid,
    actionsBar
  ]);

  card.append(col1, metricsRow);
  wrap.append(card);

  let lastChartSig = '';

  const update = (nextVm) => {
    vm = nextVm;
    const { tone, label, title } = getVmToneAndLabel(vm);

    if (statusPoint.dataset.tone !== tone) statusPoint.dataset.tone = tone;
    if (statusBadge.dataset.tone !== tone) statusBadge.dataset.tone = tone;
    statusBadge.title = title || '';
    statusPoint.title = title || '';
    setTextIfChanged(statusBadge, label);

    setTextIfChanged(nameBtn, vm.name);
    const termAllowed = vm.state !== 'stopped' && vm.state !== 'stopping' && vm.state !== 'deleting';
    const termLabel = termAllowed ? `${t('openTerminal')}: ${vm.name}` : t('terminalStoppedHint');
    terminalBtn.disabled = !termAllowed;
    if (terminalBtn.getAttribute('aria-label') !== termLabel) {
      terminalBtn.setAttribute('aria-label', termLabel);
      terminalBtn.title = termLabel;
    }
    const renameLabel = `${t('renameMachine')}: ${vm.name}`;
    if (renameBtn.getAttribute('aria-label') !== renameLabel) {
      renameBtn.setAttribute('aria-label', renameLabel);
      renameBtn.title = renameLabel;
    }
    const deleteLabel = `${t('deleteMachine')}: ${vm.name}`;
    if (deleteBtn.getAttribute('aria-label') !== deleteLabel) {
      deleteBtn.setAttribute('aria-label', deleteLabel);
      deleteBtn.title = deleteLabel;
    }
    setTextIfChanged(aliasBadge, vm.accountAlias || vm.accountId);
    setTextIfChanged(idBadge, vm.id);

    if (vm.templateId) {
      tplBadge.hidden = false;
      setTextIfChanged(tplBadge, vm.templateId);
    } else {
      tplBadge.hidden = true;
    }

    if (vm.free) {
      freeBadge.hidden = false;
      setTextIfChanged(freeBadge, t('freePlan'));
    } else {
      freeBadge.hidden = true;
    }

    const kaEnabled = !!vm.keepaliveEnabled;
    if (!keepaliveBusy) {
      keepaliveInput.checked = kaEnabled;
    }
    keepaliveToggle.dataset.active = kaEnabled ? 'true' : 'false';
    setTextIfChanged(keepaliveText, t('keepaliveLabel'));
    keepaliveToggle.title = vm.keepaliveLastError || t('keepaliveDesc');
    const kaLockedBySched = !!vm.scheduleEnabled && !kaEnabled;
    keepaliveInput.disabled = keepaliveBusy || kaLockedBySched;
    keepaliveToggle.dataset.locked = kaLockedBySched ? 'true' : 'false';
    if (kaLockedBySched) keepaliveToggle.title = t('keepaliveLockedBySchedule');
    if (!kaEnabled) {
      keepaliveMeta.hidden = true;
      delete keepaliveMeta.dataset.tone;
    } else if (vm.keepaliveLastError) {
      keepaliveMeta.hidden = false;
      keepaliveMeta.dataset.tone = 'error';
      setTextIfChanged(keepaliveMeta, '!');
    } else if (vm.keepaliveLastAction === 'cooldown_wait') {
      keepaliveMeta.hidden = false;
      keepaliveMeta.dataset.tone = 'warn';
      setTextIfChanged(keepaliveMeta, t('keepaliveWaitCooldown'));
    } else if (vm.keepaliveLastAction === 'manual_paused') {
      keepaliveMeta.hidden = false;
      keepaliveMeta.dataset.tone = 'muted';
      setTextIfChanged(keepaliveMeta, t('keepaliveManualPaused'));
    } else if (vm.keepaliveLeaseExpiresAt > 0) {
      const remSec = Math.max(0, vm.keepaliveLeaseExpiresAt - Math.floor(upstreamNowMs() / 1000));
      if (remSec > 0) {
        keepaliveMeta.hidden = false;
        keepaliveMeta.dataset.tone = 'ok';
        setTextIfChanged(keepaliveMeta, fmtCompactDuration(remSec));
      } else {
        keepaliveMeta.hidden = true;
      }
    } else {
      keepaliveMeta.hidden = true;
    }

    // --- schedule row state sync ---
    const schEnabled = !!vm.scheduleEnabled;
    if (!schedBusy) {
      schedEnabledInput.checked = schEnabled;
    }
    schedToggle.dataset.active = schEnabled ? 'true' : 'false';
    setTextIfChanged(schedText, t('schedLabel'));
    schedToggle.title = vm.scheduleLastError || t('schedDesc');
    // Keep-awake and schedule are opposing modes: whichever is active locks the other.
    const schLockedByKa = kaEnabled && !schEnabled;
    schedEnabledInput.disabled = schedBusy || schLockedByKa;
    schedToggle.dataset.locked = schLockedByKa ? 'true' : 'false';
    if (schLockedByKa) schedToggle.title = t('schedLockedByKeepalive');
    const hasWindows = !!vm.scheduleWindows;
    schedDetailsBtn.hidden = !hasWindows;
    setTextIfChanged(schedDetailsBtn, t('schedDetails'));
    if (!schEnabled) {
      schedMeta.hidden = true;
      delete schedMeta.dataset.tone;
    } else if (vm.scheduleLastError) {
      schedMeta.hidden = false;
      schedMeta.dataset.tone = 'error';
      setTextIfChanged(schedMeta, '!');
      schedMeta.title = vm.scheduleLastError;
    } else {
      const nxt = scheduleNextTransition(vm);
      if (nxt) {
        schedMeta.hidden = false;
        schedMeta.dataset.tone = nxt.kind === 'stop' ? 'ok' : 'muted';
        const key = nxt.kind === 'stop'
          ? (nxt.tomorrow ? 'schedStopAtTomorrow' : 'schedStopAt')
          : (nxt.tomorrow ? 'schedStartAtTomorrow' : 'schedStartAt');
        setTextIfChanged(schedMeta, t(key, { time: nxt.hhmm, wait: fmtCompactDuration(nxt.inMin * 60) }));
      } else if (vm.scheduleLastAction === 'cooldown_wait') {
        schedMeta.hidden = false;
        schedMeta.dataset.tone = 'warn';
        setTextIfChanged(schedMeta, t('keepaliveWaitCooldown'));
      } else {
        schedMeta.hidden = true;
      }
    }

    const diskSummary = vm.rootDiskGiB > 0
      ? `/ ${vm.rootDiskGiB} GB + /data ${vm.persistentDiskGiB}`
      : `${vm.persistentDiskGiB}`;
    const specTxt = t('specFmt', {
      vcpu: vm.vcpus,
      ram: `${(vm.memMiB / 1024).toFixed(vm.memMiB % 1024 === 0 ? 0 : 1)} GiB`,
      disk: diskSummary
    });
    const policyTxt = vm.autoSuspendAfterSeconds
      ? `${t('idlePolicy')}: ${Math.round(vm.autoSuspendAfterSeconds / 60)}m`
      : t('alwaysOn');
    setTextIfChanged(specEl, `${specTxt} · ${policyTxt}`);

    const clockInfo = formatClockDisplay(vm);
    setTextIfChanged(clockValueEl, clockInfo.text);
    if (clockValueEl.dataset.mode !== clockInfo.mode) clockValueEl.dataset.mode = clockInfo.mode;

    // instabox.sh now schedules machine deletion (`deleteAt` / deleteAfterSeconds)
    // and stop-archive (`archiveAt` / archiveSeconds). Show the nearest deadline in
    // place of the max-run cap so the user always sees what ends this machine.
    const nowMs2 = upstreamNowMs();
    const deadlineEntries = [];
    const delAt = vm.deleteAt ? Date.parse(vm.deleteAt) : NaN;
    const arcAt = vm.archiveAt ? Date.parse(vm.archiveAt) : NaN;
    if (Number.isFinite(delAt) && delAt > nowMs2) deadlineEntries.push({ at: delAt, icon: 'delete', label: t('deleteInLabel') });
    if (Number.isFinite(arcAt) && arcAt > nowMs2) deadlineEntries.push({ at: arcAt, icon: 'archive', label: t('archiveInLabel') });
    deadlineEntries.sort((a, b) => a.at - b.at);
    if (deadlineEntries.length > 0) {
      clockLimitEl.hidden = false;
      const d = deadlineEntries[0];
      setTextIfChanged(clockLimitEl, `${d.label} ${fmtCompactDuration(Math.max(0, (d.at - nowMs2) / 1000))}`);
    } else if (vm.maxRunMinutes > 0 && clockInfo.mode === 'running') {
      clockLimitEl.hidden = false;
      setTextIfChanged(clockLimitEl, `/ ${fmtHMS(vm.maxRunMinutes * 60)}`);
    } else {
      clockLimitEl.hidden = true;
    }
    // Abuse-lock cooldowns (cooldownReason abuse_*) look identical to a free-plan
    // max-run cooldown upstream, but they can last up to 24h and cannot be woken
    // even by admins. Surface the reason so users do not misread it as a max-run pause.
    if (clockInfo.mode === 'cooldown') {
      const abuseReason = (vm.cooldownReason || '').startsWith('abuse_')
        ? vm.cooldownReason
        : '';
      setTextIfChanged(clockLabelEl, abuseReason ? t('abuseLockLeft', { reason: abuseReason }) : t('cooldownLeft'));
    } else {
      setTextIfChanged(clockLabelEl, t('runtimeClock'));
    }
    setTextIfChanged(clockRight, `${t('monthlyUsage')}: ${fmtCompactDuration(vm.monthlyRunningSeconds || 0)}`);

    if (vm.lastErrorMsg) {
      errEl.hidden = false;
      setTextIfChanged(errEl, `${vm.lastErrorCode || 'error'}: ${vm.lastErrorMsg}`);
    } else {
      errEl.hidden = true;
    }

    setTextIfChanged(cpuLbl, t('cpuLabel'));
    setTextIfChanged(ramLbl, t('ramLabel'));
    setTextIfChanged(rootLbl, t('diskRootLabel'));
    setTextIfChanged(dataLbl, t('diskDataLabel'));

    const samples = vm.recentMetrics || [];
    const latest = vm.latestMetric || null;
    const rootStat = getVmMountStats(vm, '/');
    const dataStat = getVmMountStats(vm, '/data');
    const sig = metricsSignature(vm, rootStat.pct, dataStat.pct);

    setTextIfChanged(cpuVal, latest ? `${latest.cpuPct.toFixed(1)}%` : '--');
    setTextIfChanged(ramVal, latest && latest.memTotal > 0 ? `${((latest.memUsed / latest.memTotal) * 100).toFixed(1)}%` : '--');
    setTextIfChanged(rootVal, rootStat.pct > 0 ? `${rootStat.pct.toFixed(1)}%` : '--');
    setTextIfChanged(dataVal, dataStat.pct > 0 ? `${dataStat.pct.toFixed(1)}%` : '--');
    rootBox.title = rootStat.sizeBytes > 0 ? `${t('diskRootLabel')}: ${fmtBytes(rootStat.usedBytes)} / ${fmtBytes(rootStat.sizeBytes)}` : t('diskRootLabel');
    dataBox.title = dataStat.sizeBytes > 0 ? `${t('diskDataLabel')}: ${fmtBytes(dataStat.usedBytes)} / ${fmtBytes(dataStat.sizeBytes)}` : t('diskDataLabel');

    if (sig !== lastChartSig) {
      lastChartSig = sig;
      const cpuVals = samples.map((s) => s.cpuPct || 0);
      const ramVals = samples.map((s) => (s.memTotal > 0 ? (s.memUsed / s.memTotal) * 100 : 0));

      const rootSeries = samples
        .filter((s) => s.rootTotal > 0 && s.rootUsed > 0)
        .map((s) => (s.rootUsed / s.rootTotal) * 100);
      const rootVals = rootSeries.length ? rootSeries : (rootStat.pct > 0 ? [rootStat.pct] : []);

      const dataSeries = samples
        .filter((s) => s.diskTotal > 0 && s.diskUsed > 0)
        .map((s) => (s.diskUsed / s.diskTotal) * 100);
      const dataVals = dataSeries.length ? dataSeries : (dataStat.pct > 0 ? [dataStat.pct] : []);

      cpuSvgHost.replaceChildren(buildMiniAreaChart(cpuVals, '#2563eb', 100));
      ramSvgHost.replaceChildren(buildMiniAreaChart(ramVals, '#10b981', 100));
      rootSvgHost.replaceChildren(buildMiniAreaChart(rootVals, '#ec4899', 100));
      dataSvgHost.replaceChildren(buildMiniAreaChart(dataVals, '#f59e0b', 100));
    }

    const cooldownActive = vm.cooldownUntil && Date.parse(vm.cooldownUntil) > upstreamNowMs();
    wakeCtrl.update(vm, t('actWake'), vm.state === 'suspended' && !cooldownActive);
    suspendCtrl.update(vm, t('actPause'), vm.state === 'running');
    startCtrl.update(vm, t('actStart'), (vm.state === 'stopped' || vm.state === 'suspended') && !cooldownActive);
    stopCtrl.update(vm, t('actStop'), vm.state === 'running');
  };

  update(initialVm);

  return {
    wrap,
    update,
    hideTooltips() {
      wakeCtrl.hideTooltip();
      suspendCtrl.hideTooltip();
      startCtrl.hideTooltip();
      stopCtrl.hideTooltip();
    }
  };
}

// ============================================================
// Modern KPI Cards (Subgrid-aligned, interactive status filters)
// ============================================================
function syncStateFilterUI() {
  const sel = document.getElementById('filter-state-select');
  if (sel && sel.value !== state.filterState) {
    sel.value = state.filterState;
  }
  const strip = document.getElementById('stats-strip');
  if (strip?._kpi?.cards) {
    for (const item of strip._kpi.cards) {
      const active = state.filterState === item.filterKey;
      item.card.dataset.active = active ? 'true' : 'false';
      item.card.setAttribute('aria-pressed', active ? 'true' : 'false');
    }
  }
}

function ensureKpiCards(strip) {
  if (strip._kpi) return strip._kpi;

  const cards = [];
  const makeCard = ({ filterKey, tone = '', showLivePill = false }) => {
    const active = state.filterState === filterKey;
    const card = el('button', {
      type: 'button',
      className: 'kpi-card',
      dataset: {
        filter: filterKey,
        ...(tone ? { tone } : {}),
        active: active ? 'true' : 'false'
      },
      'aria-pressed': active ? 'true' : 'false',
      onClick: () => {
        if (state.accounts.length === 0) return;
        state.filterState =
          filterKey === 'all'
            ? 'all'
            : state.filterState === filterKey
              ? 'all'
              : filterKey;
        syncStateFilterUI();
        renderVmListOnly();
      }
    });

    const lbl = el('span', { className: 'kpi-label' });
    const eyebrow = el('span', { className: 'kpi-eyebrow' }, [lbl]);

    const head = el('div', { className: 'kpi-head' }, [eyebrow]);
    if (showLivePill) {
      const liveText = el('span');
      const pill = el('span', { className: 'live-pill' }, [
        el('span', { className: 'live-pill-dot', 'aria-hidden': 'true' }),
        liveText
      ]);
      head.append(pill);
      card._liveText = liveText;
    }

    const val = el('div', { className: 'kpi-value', dataset: tone ? { tone } : {} });
    const track = el('div', { className: 'kpi-track' });
    const foot = el('div', { className: 'kpi-foot' });

    card.append(head, val, track, foot);
    cards.push({ card, filterKey });
    return { card, lbl, val, track, foot, liveText: card._liveText };
  };

  const cTotal = makeCard({ filterKey: 'all', showLivePill: true });
  const segRun = el('span', { className: 'kpi-fill', dataset: { tone: 'running' } });
  const segIdle = el('span', { className: 'kpi-fill', dataset: { tone: 'idle' } });
  const segStop = el('span', { className: 'kpi-fill', dataset: { tone: 'stopped' } });
  const segErr = el('span', { className: 'kpi-fill', dataset: { tone: 'error' } });
  cTotal.track.append(segRun, segIdle, segStop, segErr);

  const cRun = makeCard({ filterKey: 'running', tone: 'running' });
  const runFill = el('span', { className: 'kpi-fill', dataset: { tone: 'running' } });
  cRun.track.append(runFill);

  const cPause = makeCard({ filterKey: 'suspended', tone: 'idle' });
  const pauseFill = el('span', { className: 'kpi-fill', dataset: { tone: 'idle' } });
  cPause.track.append(pauseFill);

  const cStop = makeCard({ filterKey: 'stopped', tone: 'stopped' });
  const stopFill = el('span', { className: 'kpi-fill', dataset: { tone: 'stopped' } });
  cStop.track.append(stopFill);

  strip.replaceChildren(cTotal.card, cRun.card, cPause.card, cStop.card);
  strip._kpi = { cards, cTotal, segRun, segIdle, segStop, segErr, cRun, runFill, cPause, pauseFill, cStop, stopFill };
  return strip._kpi;
}

function populateStatsStrip(strip) {
  const kpi = ensureKpiCards(strip);
  const total = state.machines.length;
  const running = state.machines.filter((m) => m.state === 'running').length;
  const paused = state.machines.filter((m) => m.state === 'suspended').length;
  const stopped = state.machines.filter((m) => m.state === 'stopped').length;
  const errored = Math.max(0, total - running - paused - stopped);

  const activeVcpu = state.machines
    .filter((m) => m.state === 'running')
    .reduce((acc, m) => acc + (m.vcpus || 0), 0);
  const autoPaused = state.machines.filter((m) => m.state === 'suspended' && !!m.autoSuspendedAt).length;
  const manualPaused = Math.max(0, paused - autoPaused);
  const totalMonthlySec = state.machines.reduce((acc, m) => acc + (m.monthlyRunningSeconds || 0), 0);

  const pct = (n) => (total > 0 ? Math.round((n / total) * 100) : 0);
  const widthPct = (n) => (total > 0 && n > 0 ? `${Math.max(3, (n / total) * 100)}%` : '0%');

  setTextIfChanged(kpi.cTotal.lbl, t('totalVMs'));
  setTextIfChanged(kpi.cTotal.liveText, t('liveBadge'));
  setTextIfChanged(kpi.cTotal.val, String(total));
  kpi.segRun.style.width = widthPct(running);
  kpi.segIdle.style.width = widthPct(paused);
  kpi.segStop.style.width = widthPct(stopped);
  kpi.segErr.style.width = widthPct(errored);
  setTextIfChanged(kpi.cTotal.foot, t('kpiAccountsFmt', { accounts: state.accounts.length, vcpu: activeVcpu }));

  setTextIfChanged(kpi.cRun.lbl, t('runningVMs'));
  setTextIfChanged(kpi.cRun.val, String(running));
  kpi.runFill.style.width = widthPct(running);
  setTextIfChanged(kpi.cRun.foot, t('kpiShareFmt', { pct: pct(running) }));

  setTextIfChanged(kpi.cPause.lbl, t('pausedVMs'));
  setTextIfChanged(kpi.cPause.val, String(paused));
  kpi.pauseFill.style.width = widthPct(paused);
  setTextIfChanged(kpi.cPause.foot, t('kpiIdleSplit', { auto: autoPaused, manual: manualPaused }));

  setTextIfChanged(kpi.cStop.lbl, t('stoppedVMs'));
  setTextIfChanged(kpi.cStop.val, String(stopped));
  kpi.stopFill.style.width = widthPct(stopped);
  setTextIfChanged(kpi.cStop.foot, t('kpiMonthlyFleet', { dur: fmtCompactDuration(totalMonthlySec) }));

  for (const item of kpi.cards) {
    const active = state.filterState === item.filterKey;
    item.card.dataset.active = active ? 'true' : 'false';
    item.card.setAttribute('aria-pressed', active ? 'true' : 'false');
  }
}

function updateDashboardHero(heroEl) {
  const hero = heroEl || document.getElementById('dashboard-hero');
  if (!hero || !hero._refs) return;
  const { brandTitle, addBtnText, syncBtnText } = hero._refs;

  setTextIfChanged(brandTitle, t('appName'));
  setTextIfChanged(addBtnText, t('addMachineBtn'));
  setTextIfChanged(syncBtnText, t('refreshAll'));
}

function renderDashboard() {
  const main = el('main', { className: 'main-wrap' });

  // Brand Hero Header + Pill Actions
  const brandTitle = el('h1', { className: 'hero-brand-title' });
  const heroBrand = el('div', { className: 'hero-brand' }, [
    el('img', { className: 'hero-brand-logo', src: '/assets/logo.svg', alt: '', width: '40', height: '40' }),
    brandTitle
  ]);
  const addBtnText = el('span');
  const syncBtnText = el('span');
  const syncIconWrap = el('span', { className: 'btn-sync-icon', 'aria-hidden': 'true' }, [
    el('span', { className: 'btn-sync-glyph btn-sync-glyph-refresh' }, [ICONS.refresh()]),
    el('span', { className: 'btn-sync-glyph btn-sync-glyph-check' }, [ICONS.check()])
  ]);

  const hero = el('section', { id: 'dashboard-hero', className: 'dashboard-hero' }, [
    el('div', { className: 'hero-text' }, [heroBrand]),
    el('div', { className: 'hero-actions' }, [
      el('button', {
        type: 'button',
        className: 'btn-pill-dark',
        onClick: () => openAddMachineDialog()
      }, [ICONS.plus(), addBtnText]),
      el('button', {
        id: 'sync-all-btn',
        type: 'button',
        className: 'btn-ghost-link btn-sync',
        onClick: () => triggerSyncAll()
      }, [syncIconWrap, syncBtnText])
    ])
  ]);
  hero._refs = { brandTitle, addBtnText, syncBtnText };
  updateDashboardHero(hero);
  main.append(hero);

  // Modern KPI Cards Strip
  const strip = el('section', { id: 'stats-strip', className: 'stats-strip' });
  populateStatsStrip(strip);
  main.append(strip);

  // Empty state if zero accounts
  if (state.accounts.length === 0) {
    main.append(el('section', { className: 'empty-state' }, [
      el('h2', { textContent: t('noAccountsTitle') }),
      el('p', { textContent: t('noAccountsDesc') }),
      el('button', {
        type: 'button',
        className: 'btn btn-primary',
        textContent: t('addMachineBtn'),
        onClick: () => openAddMachineDialog('connect')
      })
    ]));
    return main;
  }

  // Toolbar: Search + Account filter + State filter + Grid/Card switcher
  const toolbar = el('search', { className: 'toolbar' });
  const searchWrap = el('div', { className: 'toolbar-search' });
  const searchInput = el('input', {
    type: 'search',
    className: 'input',
    placeholder: t('searchPlaceholder'),
    value: state.searchQuery,
    'aria-label': t('searchPlaceholder'),
    onInput: (e) => {
      state.searchQuery = e.target.value;
      renderVmListOnly();
    }
  });
  searchWrap.append(searchInput);

  const accSelect = el('select', {
    id: 'filter-account-select',
    className: 'select-compact',
    'aria-label': t('allAccounts'),
    onChange: (e) => {
      state.filterAccount = e.target.value;
      renderVmListOnly();
    }
  });
  syncAccountFilterSelect(accSelect);

  const stateSelect = el('select', {
    id: 'filter-state-select',
    className: 'select-compact',
    'aria-label': t('allStates'),
    onChange: (e) => {
      state.filterState = e.target.value;
      syncStateFilterUI();
      renderVmListOnly();
    }
  }, [
    el('option', { value: 'all', textContent: t('allStates') }),
    el('option', { value: 'running', textContent: t('statusRunning') }),
    el('option', { value: 'suspended', textContent: t('statusSuspended') }),
    el('option', { value: 'stopped', textContent: t('statusStopped') })
  ]);
  stateSelect.value = state.filterState;

  const gridBtn = el('button', {
    type: 'button',
    'aria-pressed': state.viewMode === 'grid' ? 'true' : 'false',
    'aria-label': t('viewGrid')
  }, [ICONS.grid(), el('span', { textContent: t('viewGrid') })]);
  const cardsBtn = el('button', {
    type: 'button',
    'aria-pressed': state.viewMode === 'cards' ? 'true' : 'false',
    'aria-label': t('viewCards')
  }, [ICONS.cards(), el('span', { textContent: t('viewCards') })]);

  gridBtn.addEventListener('click', () => {
    state.viewMode = 'grid';
    writePref('ibmon-view', 'grid');
    gridBtn.setAttribute('aria-pressed', 'true');
    cardsBtn.setAttribute('aria-pressed', 'false');
    renderVmListOnly();
  });
  cardsBtn.addEventListener('click', () => {
    state.viewMode = 'cards';
    writePref('ibmon-view', 'cards');
    cardsBtn.setAttribute('aria-pressed', 'true');
    gridBtn.setAttribute('aria-pressed', 'false');
    renderVmListOnly();
  });

  const viewSeg = el('div', { className: 'segmented', role: 'group' }, [gridBtn, cardsBtn]);

  toolbar.append(searchWrap, accSelect, stateSelect, viewSeg);
  main.append(toolbar);

  const listHost = el('section', { id: 'vm-list-host' });
  populateVmList(listHost);
  main.append(listHost);
  return main;
}

function machineStateRank(vm) {
  switch (vm?.state) {
    case 'running':
      return 0;
    case 'starting':
    case 'resuming':
    case 'creating':
      return 1;
    case 'suspended':
      return 2;
    case 'stopping':
    case 'suspending':
    case 'deleting':
      return 3;
    case 'stopped':
      return 4;
    default:
      return 5;
  }
}

function getFilteredMachines() {
  const q = state.searchQuery.trim().toLowerCase();
  return state.machines
    .filter((vm) => {
      if (state.filterAccount !== 'all' && vm.accountId !== state.filterAccount) return false;
      if (state.filterState !== 'all' && vm.state !== state.filterState) return false;
      if (q) {
        const hay = `${vm.name} ${vm.id} ${vm.templateId} ${vm.accountAlias || ''}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    })
    .sort((a, b) => machineStateRank(a) - machineStateRank(b));
}

function populateVmList(host) {
  const items = getFilteredMachines();
  if (items.length === 0) {
    document.querySelectorAll('.action-tooltip').forEach((tip) => {
      try { if (tip.matches(':popover-open')) tip.hidePopover(); } catch {}
    });
    vmCardRegistry.clear();
    host.replaceChildren(el('div', { className: 'empty-state' }, [
      el('h3', { textContent: t('noMachinesTitle') }),
      el('p', { textContent: t('noMachinesDesc') })
    ]));
    host._renderedKeys = null;
    host._container = null;
    return;
  }

  const targetClass = state.viewMode === 'cards' ? 'vm-container-cards' : 'vm-container-grid';
  let container = host._container;
  if (!container || !host.contains(container)) {
    container = el('div', { className: targetClass });
    host._container = container;
    host._renderedKeys = [];
    host.replaceChildren(container);
  } else if (container.className !== targetClass) {
    container.className = targetClass;
  }

  const nextKeys = items.map((vm) => `${vm.accountId}:${vm.id}`);
  const prevKeys = host._renderedKeys || [];
  const sameOrder =
    prevKeys.length === nextKeys.length &&
    prevKeys.every((k, idx) => k === nextKeys[idx]);

  if (sameOrder) {
    // Pure in-place update: never removes or re-inserts any DOM nodes!
    for (let i = 0; i < items.length; i++) {
      const ctrl = vmCardRegistry.get(nextKeys[i]);
      if (ctrl) ctrl.update(items[i]);
    }
    return;
  }

  // Reconcile cards atomically while preserving existing node instances
  const keepSet = new Set(nextKeys);
  for (const [k, ctrl] of vmCardRegistry.entries()) {
    if (!keepSet.has(k)) {
      ctrl.hideTooltips();
      vmCardRegistry.delete(k);
    }
  }

  const nodes = items.map((vm) => {
    const key = `${vm.accountId}:${vm.id}`;
    let ctrl = vmCardRegistry.get(key);
    if (!ctrl) {
      ctrl = createVmCardController(vm);
      vmCardRegistry.set(key, ctrl);
    } else {
      ctrl.update(vm);
    }
    return ctrl.wrap;
  });

  container.replaceChildren(...nodes);
  host._renderedKeys = nextKeys;
}

function renderVmListOnly() {
  const host = document.getElementById('vm-list-host');
  if (host) populateVmList(host);
}

function syncAccountFilterSelect(accSelect) {
  const sel = accSelect || document.getElementById('filter-account-select');
  if (!sel) return;
  const sig = state.accounts.map((a) => `${a.id}:${a.alias}`).join('|') + `|${state.lang}`;
  if (sel.dataset.sig === sig) return;
  sel.dataset.sig = sig;
  const opts = [el('option', { value: 'all', textContent: t('allAccounts') })];
  for (const a of state.accounts) {
    opts.push(el('option', { value: a.id, textContent: a.alias }));
  }
  sel.replaceChildren(...opts);
  if (state.filterAccount !== 'all' && !state.accounts.some((a) => a.id === state.filterAccount)) {
    state.filterAccount = 'all';
  }
  sel.value = state.filterAccount;
}

function updateDashboardInPlace() {
  const strip = document.getElementById('stats-strip');
  const host = document.getElementById('vm-list-host');
  if (strip && host) {
    updateDashboardHero();
    syncAccountFilterSelect();
    populateStatsStrip(strip);
    populateVmList(host);
    return true;
  }
  return false;
}

function enableDialogLightDismiss(dialog) {
  dialog.setAttribute('closedby', 'any');
  if (!('closedBy' in HTMLDialogElement.prototype)) {
    const outside = (e) => {
      if (e.target !== dialog) return false;
      const r = dialog.getBoundingClientRect();
      return e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom;
    };
    let pressedOutside = false;
    dialog.addEventListener('pointerdown', (e) => { pressedOutside = outside(e); });
    dialog.addEventListener('pointerup', (e) => { if (pressedOutside && outside(e)) dialog.close(); });
  }
}

// ============================================================
// Add Machine Dialog (Create New VM OR Attach New API Key)
// ============================================================
let addMachineDialog = null;
function openAddMachineDialog(initialTab) {
  if (!addMachineDialog) {
    addMachineDialog = el('dialog', {
      className: 'app-dialog app-dialog-narrow',
      'aria-labelledby': 'add-machine-title'
    });
    enableDialogLightDismiss(addMachineDialog);
    addMachineDialog.addEventListener('close', () => {
      const apiKeyInput = addMachineDialog.querySelector('#addm-apikey');
      if (apiKeyInput) apiKeyInput.value = '';
    });
    document.body.append(addMachineDialog);
  }
  const activeTab = initialTab || (state.accounts.length > 0 ? 'create' : 'connect');
  renderAddMachineDialogContent(activeTab);
  if (!addMachineDialog.open) {
    addMachineDialog.showModal();
  } else {
    addMachineDialog.querySelector('[autofocus]')?.focus();
  }
}

function renderAddMachineDialogContent(initialTab) {
  if (!addMachineDialog) return;

  let activeTab = initialTab === 'connect' || state.accounts.length === 0 ? 'connect' : 'create';

  const header = el('div', { className: 'dialog-header' }, [
    el('h2', { id: 'add-machine-title', textContent: t('addMachineTitle') }),
    el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('close'),
      onClick: () => addMachineDialog.close()
    }, [ICONS.close()])
  ]);

  const body = el('div', { className: 'dialog-body' });
  body.append(el('p', { className: 'hint', style: 'margin-block-end:0.85rem;', textContent: t('addMachineDesc') }));

  const tabsWrap = el('div', { className: 'dialog-tabs' });
  const tabCreateBtn = el('button', {
    type: 'button',
    role: 'tab',
    id: 'tab-create-vm',
    'aria-selected': activeTab === 'create' ? 'true' : 'false',
    textContent: t('tabCreateVm')
  });
  const tabConnectBtn = el('button', {
    type: 'button',
    role: 'tab',
    id: 'tab-connect-key',
    'aria-selected': activeTab === 'connect' ? 'true' : 'false',
    textContent: t('tabConnectKey')
  });
  if (activeTab === 'create') tabCreateBtn.setAttribute('focusgroupstart', '');
  else tabConnectBtn.setAttribute('focusgroupstart', '');

  const tablist = el('div', {
    className: 'dialog-tablist',
    role: 'tablist',
    focusgroup: 'tablist nomemory',
    'aria-label': t('addMachineTitle')
  }, [tabCreateBtn, tabConnectBtn]);

  const panelCreate = el('div', {
    role: 'tabpanel',
    'aria-labelledby': 'tab-create-vm',
    hidden: activeTab !== 'create'
  });
  const panelConnect = el('div', {
    role: 'tabpanel',
    'aria-labelledby': 'tab-connect-key',
    hidden: activeTab !== 'connect'
  });

  const tabs = [
    { id: 'create', btn: tabCreateBtn, panel: panelCreate },
    { id: 'connect', btn: tabConnectBtn, panel: panelConnect }
  ];

  const syncRovingTabindex = () => {
    if ('focusGroup' in HTMLElement.prototype) return;
    for (const item of tabs) {
      item.btn.tabIndex = item.id === activeTab ? 0 : -1;
    }
  };

  const setActiveTab = (nextId, moveFocus = false) => {
    activeTab = nextId;
    for (const item of tabs) {
      const sel = item.id === nextId;
      item.btn.setAttribute('aria-selected', sel ? 'true' : 'false');
      item.btn.toggleAttribute('focusgroupstart', sel);
      item.panel.hidden = !sel;
      if (sel) {
        item.btn.scrollIntoView({ block: 'nearest', inline: 'nearest', container: 'nearest' });
        if (moveFocus) item.btn.focus();
      }
    }
    syncRovingTabindex();
  };

  syncRovingTabindex();
  tabCreateBtn.addEventListener('click', () => setActiveTab('create'));
  tabConnectBtn.addEventListener('click', () => setActiveTab('connect'));

  tablist.addEventListener('keydown', (e) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
    e.preventDefault();
    const cur = tabs.findIndex((x) => x.id === activeTab);
    let next = cur;
    if (e.key === 'ArrowRight') next = (cur + 1) % tabs.length;
    else if (e.key === 'ArrowLeft') next = (cur - 1 + tabs.length) % tabs.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = tabs.length - 1;
    setActiveTab(tabs[next].id, true);
  });

  // --- Panel 1: Create New VM ---
  const createInner = el('div', { className: 'tabpanel-inner' });
  if (state.accounts.length === 0) {
    createInner.append(
      el('p', { className: 'hint', textContent: t('needAccountFirst') }),
      el('div', {}, [
        el('button', {
          type: 'button',
          className: 'btn btn-primary',
          textContent: t('switchToAttachKey'),
          onClick: () => {
            setActiveTab('connect');
            document.getElementById('addm-alias')?.focus();
          }
        })
      ])
    );
  } else {
    const createErrBanner = el('div', { className: 'form-error-banner', role: 'alert', hidden: true });
    const createForm = el('form', { novalidate: true, 'aria-labelledby': 'tab-create-vm' });

    const defaultAccId =
      state.filterAccount !== 'all' && state.accounts.some((a) => a.id === state.filterAccount)
        ? state.filterAccount
        : state.accounts[0].id;

    const accSelect = el('select', {
      id: 'addm-account',
      name: 'accountId',
      className: 'input',
      autofocus: activeTab === 'create'
    });
    for (const acc of state.accounts) {
      const label = `${acc.alias}${acc.instaboxLogin ? ` (${acc.instaboxLogin}` : ''}${acc.orgTier ? ` · ${acc.orgTier}` : ''}${acc.instaboxLogin ? ')' : ''}`;
      const opt = el('option', { value: acc.id, textContent: label });
      if (acc.id === defaultAccId) opt.selected = true;
      accSelect.append(opt);
    }
    const accField = el('div', { className: 'field', style: 'margin-block-end:0;' }, [
      el('label', { for: 'addm-account', textContent: t('targetAccountLabel') }),
      accSelect
    ]);

    const nameHint = el('span', { id: 'addm-name-hint', className: 'hint', textContent: t('machineNameHint') });
    const nameInput = el('input', {
      className: 'input',
      id: 'addm-name',
      name: 'name',
      type: 'text',
      maxlength: '63',
      pattern: '^[a-z0-9][a-z0-9-]{0,62}$',
      placeholder: t('machineNamePlaceholder'),
      autocorrect: 'off',
      autocapitalize: 'none',
      spellcheck: 'false',
      'aria-invalid': 'false',
      'aria-describedby': 'addm-name-hint',
      dataset: { error: 'addm-name-err' }
    });
    const nameErr = el('div', { id: 'addm-name-err', className: 'error-msg', textContent: t('errMachineName') });
    const nameField = el('div', { className: 'field', style: 'margin-block-end:0;', dataset: { invalid: 'false' } }, [
      el('label', { for: 'addm-name', textContent: t('machineNameLabel') }),
      nameHint,
      nameInput,
      nameErr
    ]);

    const syncNameValidity = (announce = false) => {
      const raw = nameInput.value.trim();
      if (raw !== nameInput.value) nameInput.value = raw;
      const valid = raw === '' || /^[a-z0-9][a-z0-9-]{0,62}$/.test(raw);
      nameInput.setCustomValidity(valid ? '' : t('errMachineName'));
      nameInput.setAttribute('aria-invalid', valid ? 'false' : 'true');
      nameField.dataset.invalid = valid ? 'false' : 'true';
      nameInput.setAttribute('aria-describedby', valid ? 'addm-name-hint' : 'addm-name-hint addm-name-err');
      if (!valid && announce) liveAnnouncer.textContent = `${t('machineNameLabel')}: ${t('errMachineName')}`;
      return valid;
    };
    nameInput.addEventListener('blur', () => syncNameValidity(true));
    nameInput.addEventListener('input', () => {
      nameInput.setCustomValidity('');
      if (nameField.dataset.invalid === 'true') syncNameValidity(false);
    });

    const tplSelect = el('select', {
      id: 'addm-template',
      name: 'templateId',
      className: 'input'
    });
    const tplField = el('div', { className: 'field', style: 'margin-block-end:0;' }, [
      el('label', { for: 'addm-template', textContent: t('templateLabel') }),
      tplSelect
    ]);

    const tplPreview = el('div', { className: 'template-preview' });
    const tplPreviewHead = el('div', { className: 'template-preview-head' });
    const tplPreviewDesc = el('div', { className: 'template-preview-desc' });
    tplPreview.append(tplPreviewHead, tplPreviewDesc);

    const specSelect = el('select', {
      id: 'addm-spec',
      name: 'spec',
      className: 'input'
    });
    const specField = el('div', { className: 'field', style: 'margin-block-end:0;' }, [
      el('label', { for: 'addm-spec', textContent: t('specLabel') }),
      specSelect
    ]);

    let currentOptions = null;
    const renderTemplatePreview = () => {
      const templates = currentOptions?.templates || [];
      const chosen = templates.find((x) => x.id === tplSelect.value) || templates[0];
      if (!chosen) {
        tplPreview.hidden = true;
        return;
      }
      tplPreview.hidden = false;
      const spec = chosen.defaultSpec || {};
      const ramGiB = spec.memMiB ? (spec.memMiB / 1024).toFixed(spec.memMiB % 1024 === 0 ? 0 : 1) : '?';
      const diskTxt = spec.rootDiskGiB
        ? `/ ${spec.rootDiskGiB} + /data ${spec.persistentDiskGiB || 20} GB`
        : `/data ${spec.persistentDiskGiB || 20} GB`;
      const badges = [
        el('span', { className: 'mono', textContent: `${chosen.id}@${chosen.version}` }),
        el('span', { className: 'badge', textContent: `${spec.vcpus || 4} vCPU · ${ramGiB} GiB · ${diskTxt}` })
      ];
      if (chosen.default) badges.push(el('span', { className: 'badge', dataset: { tone: 'running' }, textContent: t('defaultTag') }));
      if (!chosen.planAllowed) badges.push(el('span', { className: 'badge', dataset: { tone: 'idle' }, textContent: t('paidPlanTag') }));
      tplPreviewHead.replaceChildren(...badges);
      tplPreviewDesc.textContent = chosen.description || '';
    };

    tplSelect.addEventListener('change', renderTemplatePreview);

    let optionsSeq = 0;
    const loadOptionsForAccount = async (accountId) => {
      const seq = ++optionsSeq;
      const cached = state.machineOptionsCache.get(accountId);
      if (!cached) {
        currentOptions = null;
        tplSelect.replaceChildren(el('option', { value: 'agent-browser', textContent: t('loadingOptions') }));
        specSelect.replaceChildren(el('option', { value: '', textContent: '4 vCPU · 8 GiB RAM · 20 GB /data' }));
        renderTemplatePreview();
      }
      try {
        const opts = cached || (await apiFetch(`/api/accounts/${encodeURIComponent(accountId)}/machine-options`));
        state.machineOptionsCache.set(accountId, opts);
        if (seq !== optionsSeq) return;
        currentOptions = opts;

        const tplOpts = (opts.templates || []).map((tpl) => {
          const suffix = tpl.default ? ` (${t('defaultTag')})` : (!tpl.planAllowed ? ` (${t('paidPlanTag')})` : '');
          return el('option', { value: tpl.id, textContent: `${tpl.id}${suffix}` });
        });
        if (tplOpts.length === 0) {
          tplOpts.push(el('option', { value: 'agent-browser', textContent: 'agent-browser' }));
        }
        tplSelect.replaceChildren(...tplOpts);
        const defTpl = (opts.templates || []).find((x) => x.default && x.planAllowed) || (opts.templates || [])[0];
        if (defTpl) tplSelect.value = defTpl.id;
        renderTemplatePreview();

        const specs = opts.allowedSpecs || [];
        const specOpts = specs.map((sp) => {
          const ramGiB = (sp.memMiB / 1024).toFixed(sp.memMiB % 1024 === 0 ? 0 : 1);
          return el('option', {
            value: `${sp.vcpus}:${sp.memMiB}`,
            textContent: `${sp.vcpus} vCPU · ${ramGiB} GiB RAM · ${sp.persistentDiskGiB || 20} GB /data`
          });
        });
        if (specOpts.length === 0) {
          specOpts.push(el('option', { value: '', textContent: '4 vCPU · 8 GiB RAM · 20 GB /data' }));
        }
        specSelect.replaceChildren(...specOpts);
      } catch (err) {
        if (seq !== optionsSeq) return;
        currentOptions = null;
        renderTemplatePreview();
        tplSelect.replaceChildren(
          el('option', { value: 'agent-browser', textContent: 'agent-browser' }),
          el('option', { value: 'android', textContent: 'android' }),
          el('option', { value: 'omarchy', textContent: 'omarchy' }),
          el('option', { value: 'ci', textContent: 'ci' })
        );
        createErrBanner.textContent = err.message;
        createErrBanner.hidden = false;
      }
    };

    accSelect.addEventListener('change', () => {
      createErrBanner.hidden = true;
      loadOptionsForAccount(accSelect.value);
    });
    loadOptionsForAccount(accSelect.value);

    // Lifetime fields, per the current instabox.sh API:
    //  - Idle pause (autoSuspendAfterSeconds): paid plans take 5-30 whole minutes
    //    (or 0 = never); the free plan is fixed at 30m — show the effective choice.
    //  - Max run (maxRunMinutes): pause after N running minutes even while busy.
    //  - Delete after (deleteAfterSeconds): hard self-destruct deadline 60-86400s,
    //    replacing the retired `timeout`/`onTimeout` lifetime fields.
    const idleSelect = el('select', { id: 'addm-idle', name: 'idlePause', className: 'input' }, [
      el('option', { value: '', textContent: t('idlePolicyDefault') }),
      el('option', { value: '300', textContent: '5m' }),
      el('option', { value: '600', textContent: '10m' }),
      el('option', { value: '900', textContent: '15m' }),
      el('option', { value: '1800', textContent: '30m' }),
      el('option', { value: '0', textContent: t('alwaysOn') + ' (0)' })
    ]);
    const idleField = el('div', { className: 'field', style: 'margin-block-end:0;' }, [
      el('label', { for: 'addm-idle', textContent: t('idlePolicyLabel') }),
      el('span', { className: 'hint', textContent: t('idlePolicyHint') }),
      idleSelect
    ]);

    const deleteAfterSelect = el('select', { id: 'addm-delete-after', name: 'deleteAfter', className: 'input' }, [
      el('option', { value: '', textContent: t('deleteAfterNever') }),
      el('option', { value: '3600', textContent: '1h' }),
      el('option', { value: '14400', textContent: '4h' }),
      el('option', { value: '28800', textContent: '8h' }),
      el('option', { value: '86400', textContent: '24h' })
    ]);
    const deleteAfterField = el('div', { className: 'field', style: 'margin-block-end:0;' }, [
      el('label', { for: 'addm-delete-after', textContent: t('deleteAfterLabel') }),
      el('span', { className: 'hint', textContent: t('deleteAfterHint') }),
      deleteAfterSelect
    ]);

    const createBtn = el('button', {
      type: 'submit',
      className: 'btn btn-primary',
      style: 'min-block-size:44px;',
      textContent: t('createVmSubmit')
    });

    createForm.append(
      createErrBanner,
      el('div', { className: 'form-stack' }, [
        accField,
        nameField,
        el('div', { className: 'form-row-2' }, [tplField, specField]),
        tplPreview,
        el('div', { className: 'form-row-2' }, [idleField, deleteAfterField]),
        createBtn
      ])
    );
    createForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      createErrBanner.hidden = true;
      if (!syncNameValidity(false)) {
        nameInput.focus();
        return;
      }
      if (createBtn.getAttribute('aria-disabled') === 'true') return;
      createBtn.setAttribute('aria-disabled', 'true');
      try {
        const [vcpusStr, memStr] = (specSelect.value || '').split(':');
        const payload = {
          accountId: accSelect.value,
          name: nameInput.value.trim(),
          templateId: tplSelect.value || 'agent-browser',
          vcpus: Number(vcpusStr) || 0,
          memMiB: Number(memStr) || 0
        };
        if (idleSelect.value !== '') {
          payload.autoSuspendAfterSeconds = Number(idleSelect.value);
        }
        if (deleteAfterSelect.value !== '') {
          payload.deleteAfterSeconds = Number(deleteAfterSelect.value);
        }
        const res = await apiFetch('/api/machines', {
          method: 'POST',
          body: JSON.stringify(payload)
        });
        applyMachinesPayload(res);
        if (!updateDashboardInPlace()) render();
        renderDrawerContent();
        addMachineDialog.close();
        showToast(`${t('machineCreated')}${payload.name ? `: ${payload.name}` : ''}`);
        setTimeout(() => refreshMachinesSilently(), 2000);
        setTimeout(() => refreshMachinesSilently(), 5500);
        setTimeout(() => refreshMachinesSilently(), 11000);
      } catch (err) {
        createErrBanner.textContent = err.message;
        createErrBanner.hidden = false;
      } finally {
        createBtn.removeAttribute('aria-disabled');
      }
    });

    createInner.append(createForm);
  }
  panelCreate.append(createInner);

  // --- Panel 2: Attach New API Key ---
  const connectInner = el('div', { className: 'tabpanel-inner' });
  const connectErrBanner = el('div', { className: 'form-error-banner', role: 'alert', hidden: true });
  const connectForm = el('form', { novalidate: true, 'aria-labelledby': 'tab-connect-key' });

  const aliasInput = el('input', {
    className: 'input',
    id: 'addm-alias',
    name: 'alias',
    type: 'text',
    required: true,
    maxlength: '64',
    placeholder: t('aliasPlaceholder'),
    'aria-invalid': 'false',
    dataset: { error: 'addm-alias-err' }
  });
  if (activeTab === 'connect') aliasInput.setAttribute('autofocus', '');
  const aliasErr = el('div', { id: 'addm-alias-err', className: 'error-msg', textContent: t('errGeneric') });
  const aliasField = el('div', { className: 'field', style: 'margin-block-end:0;', dataset: { invalid: 'false' } }, [
    el('label', { for: 'addm-alias', textContent: t('aliasLabel') }),
    aliasInput,
    aliasErr
  ]);
  const syncAlias = () => {
    const valid = aliasInput.value.trim().length > 0;
    aliasInput.setCustomValidity(valid ? '' : t('errGeneric'));
    aliasInput.setAttribute('aria-invalid', valid ? 'false' : 'true');
    aliasField.dataset.invalid = valid ? 'false' : 'true';
    if (!valid) aliasInput.setAttribute('aria-describedby', 'addm-alias-err');
    else aliasInput.removeAttribute('aria-describedby');
    return valid;
  };
  aliasInput.addEventListener('blur', syncAlias);
  aliasInput.addEventListener('input', () => {
    aliasInput.setCustomValidity('');
    if (aliasField.dataset.invalid === 'true') syncAlias();
  });

  const keyPw = createPasswordField({
    id: 'addm-apikey',
    name: 'apiKey',
    label: t('apiKeyLabel'),
    autocomplete: 'off',
    errorText: t('errApiKey'),
    required: true,
    minlength: 8
  });
  keyPw.input.placeholder = t('apiKeyPlaceholder');
  keyPw.input.autocorrect = 'off';
  keyPw.input.autocapitalize = 'none';
  keyPw.input.spellcheck = false;

  const saveKeyBtn = el('button', {
    type: 'submit',
    className: 'btn btn-primary',
    style: 'min-block-size:44px;',
    textContent: t('saveAccount')
  });

  keyPw.field.style.marginBlockEnd = '0';
  connectForm.append(
    connectErrBanner,
    el('div', { className: 'form-stack' }, [aliasField, keyPw.field, saveKeyBtn])
  );
  connectForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    connectErrBanner.hidden = true;
    const okAlias = syncAlias();
    const apiKey = keyPw.input.value.trim();
    if (apiKey.length < 8) {
      keyPw.field.dataset.customInvalid = 'true';
      keyPw.input.setCustomValidity(t('errApiKey'));
    } else {
      delete keyPw.field.dataset.customInvalid;
      keyPw.input.setCustomValidity('');
    }
    const okKey = keyPw.sync(true);
    if (!okAlias) {
      aliasInput.focus();
      return;
    }
    if (!okKey || apiKey.length < 8) {
      keyPw.input.focus();
      return;
    }
    if (saveKeyBtn.getAttribute('aria-disabled') === 'true') return;
    saveKeyBtn.setAttribute('aria-disabled', 'true');
    try {
      const hadZeroAccounts = state.accounts.length === 0;
      await apiFetch('/api/accounts', {
        method: 'POST',
        body: JSON.stringify({ alias: aliasInput.value.trim(), apiKey })
      });
      keyPw.input.value = '';
      state.machineOptionsCache.clear();
      await loadDashboardData();
      if (hadZeroAccounts || !updateDashboardInPlace()) render();
      renderDrawerContent();
      addMachineDialog.close();
      showToast(`${t('saveAccount')}: ${aliasInput.value.trim()}`);
    } catch (err) {
      connectErrBanner.textContent = err.message;
      connectErrBanner.hidden = false;
    } finally {
      saveKeyBtn.removeAttribute('aria-disabled');
    }
  });

  connectInner.append(connectForm);
  if (state.accounts.length > 0) {
    connectInner.append(el('div', { className: 'dialog-subfooter' }, [
      el('button', {
        type: 'button',
        className: 'btn-ghost-link',
        textContent: t('manageSavedKeys', { count: state.accounts.length }),
        onClick: () => {
          addMachineDialog.close();
          openAccountsDialog();
        }
      })
    ]));
  }
  panelConnect.append(connectInner);

  tabsWrap.append(tablist, panelCreate, panelConnect);
  body.append(tabsWrap);
  addMachineDialog.replaceChildren(el('div', { className: 'dialog-inner' }, [header, body]));
}

// ============================================================
// Rename Machine Dialog & Delete Machine Confirmation Dialog
// ============================================================
let renameMachineDialog = null;
function openRenameMachineDialog(vm) {
  if (!renameMachineDialog) {
    renameMachineDialog = el('dialog', {
      className: 'app-dialog app-dialog-narrow',
      'aria-labelledby': 'rename-vm-title'
    });
    enableDialogLightDismiss(renameMachineDialog);
    document.body.append(renameMachineDialog);
  }

  const header = el('div', { className: 'dialog-header' }, [
    el('h2', { id: 'rename-vm-title', textContent: t('renameMachineTitle') }),
    el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('close'),
      onClick: () => renameMachineDialog.close()
    }, [ICONS.close()])
  ]);

  const body = el('div', { className: 'dialog-body' });
  const errBanner = el('div', { className: 'form-error-banner', role: 'alert', hidden: true });
  const form = el('form', { novalidate: true, 'aria-labelledby': 'rename-vm-title' });

  const nameHint = el('span', { id: 'rename-vm-hint', className: 'hint', textContent: t('machineNameHint') });
  const nameInput = el('input', {
    className: 'input',
    id: 'rename-vm-input',
    name: 'name',
    type: 'text',
    required: true,
    maxlength: '63',
    pattern: '^[a-z0-9][a-z0-9-]{0,62}$',
    value: vm.name || '',
    autocorrect: 'off',
    autocapitalize: 'none',
    spellcheck: 'false',
    autofocus: true,
    'aria-invalid': 'false',
    'aria-describedby': 'rename-vm-hint',
    dataset: { error: 'rename-vm-err' }
  });
  const nameErr = el('div', { id: 'rename-vm-err', className: 'error-msg', textContent: t('errMachineName') });
  const nameField = el('div', { className: 'field', style: 'margin-block-end:0;', dataset: { invalid: 'false' } }, [
    el('label', { for: 'rename-vm-input', textContent: t('newMachineName') }),
    nameHint,
    nameInput,
    nameErr
  ]);

  const syncValidity = (announce = false) => {
    const raw = nameInput.value.trim();
    if (raw !== nameInput.value) nameInput.value = raw;
    const valid = /^[a-z0-9][a-z0-9-]{0,62}$/.test(raw);
    nameInput.setCustomValidity(valid ? '' : t('errMachineName'));
    nameInput.setAttribute('aria-invalid', valid ? 'false' : 'true');
    nameField.dataset.invalid = valid ? 'false' : 'true';
    nameInput.setAttribute('aria-describedby', valid ? 'rename-vm-hint' : 'rename-vm-hint rename-vm-err');
    if (!valid && announce) liveAnnouncer.textContent = `${t('newMachineName')}: ${t('errMachineName')}`;
    return valid;
  };
  nameInput.addEventListener('blur', () => syncValidity(true));
  nameInput.addEventListener('input', () => {
    nameInput.setCustomValidity('');
    if (nameField.dataset.invalid === 'true') syncValidity(false);
  });

  const btnRow = el('div', { className: 'dialog-actions-row' }, [
    el('button', {
      type: 'button',
      className: 'btn',
      textContent: t('cancelEdit'),
      onClick: () => renameMachineDialog.close()
    }),
    el('button', {
      type: 'submit',
      className: 'btn btn-primary',
      textContent: t('saveRename')
    })
  ]);
  const saveBtn = btnRow.lastElementChild;

  form.append(errBanner, el('div', { className: 'form-stack' }, [
    el('div', { className: 'hint mono', textContent: `${vm.accountAlias || vm.accountId} · ${vm.id}` }),
    nameField,
    btnRow
  ]));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errBanner.hidden = true;
    if (!syncValidity(false)) {
      nameInput.focus();
      return;
    }
    const nextName = nameInput.value.trim();
    if (nextName === vm.name) {
      renameMachineDialog.close();
      return;
    }
    if (saveBtn.getAttribute('aria-disabled') === 'true') return;
    saveBtn.setAttribute('aria-disabled', 'true');
    try {
      const res = await apiFetch(
        `/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}`,
        { method: 'PATCH', body: JSON.stringify({ name: nextName }) }
      );
      if (res?.machine) {
        const idx = state.machines.findIndex((x) => x.accountId === vm.accountId && x.id === vm.id);
        if (idx >= 0) state.machines[idx] = res.machine;
        if (!updateDashboardInPlace()) render();
      } else {
        await loadDashboardData();
        if (!updateDashboardInPlace()) render();
      }
      renameMachineDialog.close();
      showToast(`${t('machineRenamed')}: ${nextName}`);
    } catch (err) {
      errBanner.textContent = err.message;
      errBanner.hidden = false;
    } finally {
      saveBtn.removeAttribute('aria-disabled');
    }
  });

  body.append(form);
  renameMachineDialog.replaceChildren(el('div', { className: 'dialog-inner' }, [header, body]));
  if (!renameMachineDialog.open) renameMachineDialog.showModal();
  requestAnimationFrame(() => {
    nameInput.focus();
    nameInput.select();
  });
}

let deleteMachineDialog = null;
function openDeleteMachineDialog(vm) {
  if (!deleteMachineDialog) {
    deleteMachineDialog = el('dialog', {
      className: 'app-dialog app-dialog-narrow',
      'aria-labelledby': 'delete-vm-title'
    });
    enableDialogLightDismiss(deleteMachineDialog);
    document.body.append(deleteMachineDialog);
  }

  const header = el('div', { className: 'dialog-header' }, [
    el('h2', { id: 'delete-vm-title', textContent: t('deleteMachineTitle') }),
    el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('close'),
      onClick: () => deleteMachineDialog.close()
    }, [ICONS.close()])
  ]);

  const errBanner = el('div', { className: 'form-error-banner', role: 'alert', hidden: true });
  const desc = el('p', {
    textContent: t('deleteMachineDesc', { name: vm.name, id: vm.id })
  });
  const metaPill = el('div', { className: 'hint mono', textContent: `${vm.accountAlias || vm.accountId} · ${vm.id}` });

  const cancelBtn = el('button', {
    type: 'button',
    className: 'btn',
    autofocus: true,
    textContent: t('cancelEdit'),
    onClick: () => deleteMachineDialog.close()
  });

  const confirmBtn = el('button', {
    type: 'button',
    className: 'btn btn-danger-solid',
    textContent: t('confirmDeleteMachineBtn'),
    onClick: async () => {
      if (confirmBtn.getAttribute('aria-disabled') === 'true') return;
      errBanner.hidden = true;
      confirmBtn.setAttribute('aria-disabled', 'true');
      try {
        const res = await apiFetch(
          `/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}`,
          { method: 'DELETE' }
        );
        applyMachinesPayload(res);
        if (!updateDashboardInPlace()) render();
        renderDrawerContent();
        deleteMachineDialog.close();
        showToast(`${t('machineDeleted')}: ${vm.name}`);
        setTimeout(() => refreshMachinesSilently(), 1800);
        setTimeout(() => refreshMachinesSilently(), 4500);
      } catch (err) {
        errBanner.textContent = err.message;
        errBanner.hidden = false;
      } finally {
        confirmBtn.removeAttribute('aria-disabled');
      }
    }
  });

  const body = el('div', { className: 'dialog-body' }, [
    el('div', { className: 'form-stack' }, [
      errBanner,
      desc,
      metaPill,
      el('div', { className: 'dialog-actions-row' }, [cancelBtn, confirmBtn])
    ])
  ]);

  deleteMachineDialog.replaceChildren(el('div', { className: 'dialog-inner' }, [header, body]));
  if (!deleteMachineDialog.open) deleteMachineDialog.showModal();
  requestAnimationFrame(() => cancelBtn.focus());
}

// ============================================================
// Accounts Manager Dialog
// ============================================================
let accountsDialog = null;
function openAccountsDialog() {
  if (!accountsDialog) {
    accountsDialog = el('dialog', {
      className: 'app-dialog app-dialog-narrow',
      'aria-labelledby': 'accounts-dialog-title'
    });
    enableDialogLightDismiss(accountsDialog);
    document.body.append(accountsDialog);
  }
  renderAccountsDialogContent();
  if (!accountsDialog.open) accountsDialog.showModal();
}

function renderAccountsDialogContent() {
  if (!accountsDialog) return;

  const inner = el('div', { className: 'dialog-inner' });
  const header = el('div', { className: 'dialog-header' }, [
    el('h2', { id: 'accounts-dialog-title', textContent: t('accountsTitle') }),
    el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('close'),
      onClick: () => accountsDialog.close()
    }, [ICONS.close()])
  ]);

  const body = el('div', { className: 'dialog-body' });
  body.append(el('p', { className: 'hint', style: 'margin-block-end:1rem;', textContent: t('accountsDesc') }));

  const list = el('div', { className: 'account-list' });
  for (const acc of state.accounts) {
    const row = el('div', { className: 'account-row' });
    const info = el('div', { className: 'account-info' }, [
      el('div', { className: 'account-alias' }, [
        el('span', { textContent: acc.alias }),
        el('span', { className: 'badge mono', textContent: acc.keyMask }),
        acc.orgTier ? el('span', { className: 'badge', textContent: acc.orgTier }) : null
      ]),
      el('div', { className: 'account-meta', textContent: `${acc.instaboxLogin || '-'} · ${acc.lastSyncAt ? `${t('lastSync')}: ${new Date(acc.lastSyncAt * 1000).toLocaleTimeString(state.lang)}` : t('neverSynced')}` }),
      acc.lastError ? el('div', { className: 'account-err', textContent: acc.lastError }) : null
    ]);

    const controls = el('div', { className: 'account-controls' }, [
      el('button', {
        type: 'button',
        className: 'btn btn-icon-only',
        'aria-label': t('syncNow'),
        title: t('syncNow'),
        onClick: async () => {
          try {
            state.machineOptionsCache.delete(acc.id);
            const data = await apiFetch(`/api/accounts/${encodeURIComponent(acc.id)}/sync`, { method: 'POST' });
            applyMachinesPayload(data);
            renderAccountsDialogContent();
            if (!updateDashboardInPlace()) render();
          } catch (err) {
            showToast(err.message, 'error');
          }
        }
      }, [ICONS.refresh()]),
      el('button', {
        type: 'button',
        className: 'btn btn-icon-only',
        'aria-label': t('editAliasTitle'),
        title: t('editAliasTitle'),
        onClick: () => {
          state.editingAccountId = acc.id;
          renderAccountsDialogContent();
        }
      }, [ICONS.edit()]),
      el('button', {
        type: 'button',
        className: 'btn btn-icon-only btn-danger',
        'aria-label': t('deleteAccount'),
        title: t('deleteAccount'),
        onClick: async () => {
          if (!window.confirm(t('confirmDeleteAcc', { alias: acc.alias }))) return;
          try {
            await apiFetch(`/api/accounts/${encodeURIComponent(acc.id)}`, { method: 'DELETE' });
            state.machineOptionsCache.delete(acc.id);
            await loadDashboardData();
            renderAccountsDialogContent();
            render();
          } catch (err) {
            showToast(err.message, 'error');
          }
        }
      }, [ICONS.trash()])
    ]);

    row.append(info, controls);
    list.append(row);
  }
  body.append(list);

  const editing = state.editingAccountId ? state.accounts.find((a) => a.id === state.editingAccountId) : null;
  const formTitle = el('h3', { style: 'margin-block-end:0.75rem;', textContent: editing ? t('editAliasTitle') : t('addAccount') });
  const errBanner = el('div', { className: 'form-error-banner', hidden: true });
  const form = el('form', { novalidate: true });

  const aliasField = el('div', { className: 'field' }, [
    el('label', { for: 'acc-alias', textContent: t('aliasLabel') }),
    el('input', {
      className: 'input',
      id: 'acc-alias',
      name: 'alias',
      type: 'text',
      required: true,
      maxlength: '64',
      placeholder: t('aliasPlaceholder'),
      value: editing ? editing.alias : ''
    })
  ]);

  const keyPw = createPasswordField({
    id: 'acc-apikey',
    name: 'apiKey',
    label: editing ? t('newApiKeyOptional') : t('apiKeyLabel'),
    autocomplete: 'off',
    errorText: t('errGeneric'),
    required: !editing,
    minlength: editing ? 0 : 8
  });
  keyPw.input.placeholder = t('apiKeyPlaceholder');
  keyPw.input.autocorrect = 'off';
  keyPw.input.autocapitalize = 'none';
  keyPw.input.spellcheck = false;

  const btnRow = el('div', { style: 'display:flex;gap:0.5rem;flex-wrap:wrap;' });
  const saveBtn = el('button', {
    type: 'submit',
    className: 'btn btn-primary',
    textContent: editing ? t('updateAccount') : t('saveAccount')
  });
  btnRow.append(saveBtn);
  if (editing) {
    btnRow.append(el('button', {
      type: 'button',
      className: 'btn',
      textContent: t('cancelEdit'),
      onClick: () => {
        state.editingAccountId = null;
        renderAccountsDialogContent();
      }
    }));
  }

  form.append(aliasField, keyPw.field, btnRow);
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errBanner.hidden = true;
    const aliasInput = aliasField.querySelector('input');
    const alias = aliasInput.value.trim();
    const apiKey = keyPw.input.value.trim();
    if (!alias) {
      aliasInput.focus();
      return;
    }
    if (!editing && !apiKey) {
      keyPw.input.focus();
      return;
    }

    if (saveBtn.getAttribute('aria-disabled') === 'true') return;
    saveBtn.setAttribute('aria-disabled', 'true');
    try {
      const hadZeroAccounts = state.accounts.length === 0;
      if (editing) {
        await apiFetch(`/api/accounts/${encodeURIComponent(editing.id)}`, {
          method: 'PATCH',
          body: JSON.stringify({ alias, apiKey })
        });
        state.editingAccountId = null;
      } else {
        await apiFetch('/api/accounts', {
          method: 'POST',
          body: JSON.stringify({ alias, apiKey })
        });
      }
      state.machineOptionsCache.clear();
      await loadDashboardData();
      renderAccountsDialogContent();
      if (hadZeroAccounts || !updateDashboardInPlace()) render();
      renderDrawerContent();
    } catch (err) {
      errBanner.textContent = err.message;
      errBanner.hidden = false;
    } finally {
      saveBtn.removeAttribute('aria-disabled');
    }
  });

  body.append(formTitle, errBanner, form);
  inner.append(header, body);
  accountsDialog.replaceChildren(inner);
}

// ============================================================
// Copy Button (Invoker Commands + Fallback) & API Keys / MCP Dialog
// ============================================================
const copyTimers = new WeakMap();
let lastCopyMessage = '';

function legacyCopyText(text) {
  const field = Object.assign(document.createElement('textarea'), { value: text });
  field.setAttribute('readonly', '');
  field.style.cssText = 'position:fixed;inset-block-start:0;opacity:0';
  document.body.append(field);
  field.select();
  try {
    return document.execCommand('copy');
  } finally {
    field.remove();
  }
}

function reportCopyStatus(button, ok) {
  const tip = button.nextElementSibling;
  const iconWrap = button.querySelector('.icon');
  clearTimeout(copyTimers.get(button));
  button.removeAttribute('data-copied');
  button.removeAttribute('data-failed');
  let message = ok ? t('copiedTip') : t('copyFailedTip');
  if (message === lastCopyMessage) message += '\u00A0';
  lastCopyMessage = message;
  if (tip) tip.textContent = '';
  void button.offsetWidth;
  button.setAttribute(ok ? 'data-copied' : 'data-failed', '');
  if (iconWrap) iconWrap.replaceChildren(ok ? ICONS.check() : ICONS.close());
  if (tip) requestAnimationFrame(() => (tip.textContent = message));
  copyTimers.set(button, setTimeout(() => {
    button.removeAttribute('data-copied');
    button.removeAttribute('data-failed');
    if (iconWrap) iconWrap.replaceChildren(ICONS.copy());
    if (tip) tip.textContent = '';
  }, 2000));
}

async function performCopy(button, target) {
  if (!button || !target) return;
  const text = target.matches('input, textarea') ? target.value : target.innerText.trim();
  if (!text) return;
  try {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      await navigator.clipboard.writeText(text);
    } else if (!legacyCopyText(text)) {
      throw new Error('clipboard unavailable');
    }
    reportCopyStatus(button, true);
  } catch {
    try {
      if (legacyCopyText(text)) {
        reportCopyStatus(button, true);
        return;
      }
    } catch {}
    reportCopyStatus(button, false);
  }
}

if ('CommandEvent' in window) {
  document.addEventListener('command', (event) => {
    if (event.command === '--copy') performCopy(event.source, event.target);
  }, true);
} else {
  document.addEventListener('click', (event) => {
    const button = event.target.closest('button[command="--copy"]');
    if (!button) return;
    const target = document.getElementById(button.getAttribute('commandfor'));
    if (target) performCopy(button, target);
  });
}

function makeCopyableCodeBlock(codeId, codeText, copyAriaLabel) {
  const codeEl = el('code', { id: codeId, textContent: codeText });
  const preEl = el('pre', { tabindex: '0', className: 'mcp-pre mono' }, [codeEl]);
  const copyBtn = el('button', {
    type: 'button',
    className: 'copy-button',
    command: '--copy',
    commandfor: codeId,
    'aria-label': copyAriaLabel
  }, [el('span', { className: 'icon', 'aria-hidden': 'true' }, [ICONS.copy()])]);
  const tipEl = el('span', { className: 'tip', role: 'status' });
  const copyWrap = el('span', { className: 'copy' }, [copyBtn, tipEl]);
  return el('div', { className: 'copyable' }, [copyWrap, preEl]);
}

let apiKeysDialog = null;
let apiKeysListState = [];
let newlyCreatedApiToken = '';

async function openAPIKeysDialog() {
  if (!apiKeysDialog) {
    apiKeysDialog = el('dialog', {
      className: 'app-dialog app-dialog-narrow',
      'aria-labelledby': 'apikeys-dialog-title'
    });
    enableDialogLightDismiss(apiKeysDialog);
    apiKeysDialog.addEventListener('close', () => {
      newlyCreatedApiToken = '';
      renderAPIKeysDialogContent();
    });
    document.body.append(apiKeysDialog);
  }
  newlyCreatedApiToken = '';
  renderAPIKeysDialogContent();
  if (!apiKeysDialog.open) apiKeysDialog.showModal();
  try {
    const res = await apiFetch('/api/api-keys');
    apiKeysListState = res.apiKeys || [];
    renderAPIKeysDialogContent();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

function renderAPIKeysDialogContent() {
  if (!apiKeysDialog) return;

  const inner = el('div', { className: 'dialog-inner' });
  const header = el('div', { className: 'dialog-header' }, [
    el('h2', { id: 'apikeys-dialog-title', textContent: t('apiKeysTitle') }),
    el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('close'),
      onClick: () => apiKeysDialog.close()
    }, [ICONS.close()])
  ]);

  const body = el('div', { className: 'dialog-body' });
  body.append(el('p', { className: 'hint', style: 'margin-block-end:0.9rem;', textContent: t('apiKeysDesc') }));

  const mcpUrl = `${window.location.origin}/mcp`;
  body.append(
    el('div', { className: 'field', style: 'margin-block-end:0.85rem;' }, [
      el('label', { textContent: t('mcpEndpointLabel') }),
      makeCopyableCodeBlock('mcp-url-snippet', mcpUrl, `${t('copyLabel')}: ${t('mcpEndpointLabel')}`)
    ])
  );

  const tokenPlaceholder = newlyCreatedApiToken || 'ibmon_YOUR_API_KEY';
  const yamlSnippet = `mcp_servers:\n  instabox:\n    url: "${mcpUrl}"\n    headers:\n      Authorization: "Bearer ${tokenPlaceholder}"\n    timeout: 620`;
  body.append(
    el('div', { className: 'field', style: 'margin-block-end:1rem;' }, [
      el('label', { textContent: t('mcpConfigLabel') }),
      makeCopyableCodeBlock('mcp-yaml-snippet', yamlSnippet, `${t('copyLabel')}: ${t('mcpConfigLabel')}`)
    ])
  );

  if (newlyCreatedApiToken) {
    body.append(
      el('div', { className: 'secret-once-banner', role: 'status' }, [
        el('div', { className: 'secret-once-head', textContent: t('apiKeyCreatedWarn') }),
        makeCopyableCodeBlock('new-apikey-token', newlyCreatedApiToken, `${t('copyLabel')}: API Key`)
      ])
    );
  }

  const list = el('div', { className: 'account-list' });
  if (apiKeysListState.length === 0) {
    list.append(el('p', { className: 'hint', textContent: t('noApiKeys') }));
  } else {
    for (const k of apiKeysListState) {
      const lastUsedStr = k.lastUsedAt
        ? `${t('lastUsed')}: ${new Date(k.lastUsedAt * 1000).toLocaleString(state.lang)}`
        : t('neverUsed');
      const row = el('div', { className: 'account-row' }, [
        el('div', { className: 'account-info' }, [
          el('div', { className: 'account-alias' }, [
            el('span', { textContent: k.name }),
            el('span', { className: 'badge mono', textContent: k.keyMask })
          ]),
          el('div', { className: 'account-meta', textContent: lastUsedStr })
        ]),
        el('div', { className: 'account-controls' }, [
          el('button', {
            type: 'button',
            className: 'btn btn-icon-only btn-danger',
            'aria-label': `${t('revokeApiKey')}: ${k.name}`,
            title: t('revokeApiKey'),
            onClick: async () => {
              if (!window.confirm(t('confirmRevokeApiKey', { name: k.name }))) return;
              try {
                await apiFetch(`/api/api-keys/${encodeURIComponent(k.id)}`, { method: 'DELETE' });
                apiKeysListState = apiKeysListState.filter((item) => item.id !== k.id);
                renderAPIKeysDialogContent();
              } catch (err) {
                showToast(err.message, 'error');
              }
            }
          }, [ICONS.trash()])
        ])
      ]);
      list.append(row);
    }
  }
  body.append(list);

  const errBanner = el('div', { className: 'form-error-banner', role: 'alert', hidden: true });
  const nameInput = el('input', {
    className: 'input',
    id: 'new-apikey-name',
    name: 'name',
    type: 'text',
    required: true,
    maxlength: '64',
    placeholder: t('apiKeyNamePlaceholder')
  });
  const createBtn = el('button', {
    type: 'submit',
    className: 'btn btn-primary',
    textContent: t('createApiKeyBtn')
  });
  const form = el('form', { novalidate: true, className: 'form-stack' }, [
    errBanner,
    el('div', { className: 'field', style: 'margin-block-end:0;' }, [
      el('label', { for: 'new-apikey-name', textContent: t('apiKeyNameLabel') }),
      nameInput
    ]),
    el('div', { className: 'dialog-actions-row' }, [createBtn])
  ]);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errBanner.hidden = true;
    const keyName = nameInput.value.trim();
    if (!keyName) {
      nameInput.focus();
      return;
    }
    if (createBtn.getAttribute('aria-disabled') === 'true') return;
    createBtn.setAttribute('aria-disabled', 'true');
    try {
      const res = await apiFetch('/api/api-keys', {
        method: 'POST',
        body: JSON.stringify({ name: keyName })
      });
      newlyCreatedApiToken = res.token || '';
      if (res.apiKey) apiKeysListState = [res.apiKey, ...apiKeysListState];
      renderAPIKeysDialogContent();
    } catch (err) {
      errBanner.textContent = err.message;
      errBanner.hidden = false;
    } finally {
      createBtn.removeAttribute('aria-disabled');
    }
  });

  body.append(form);
  inner.append(header, body);
  apiKeysDialog.replaceChildren(inner);
}

// ============================================================
// Interactive VM Console Terminal Modal (xterm.js + WS Proxy)
// ============================================================
let xtermModulesPromise = null;
function loadXtermModules() {
  if (!xtermModulesPromise) {
    xtermModulesPromise = Promise.all([
      import('/vendor/xterm/xterm.mjs'),
      import('/vendor/xterm/addon-fit.mjs')
    ]).then(([xtermMod, fitMod]) => ({
      Terminal: xtermMod.Terminal,
      FitAddon: fitMod.FitAddon
    }));
  }
  return xtermModulesPromise;
}

let terminalDialog = null;
let terminalSessionState = null;

function disposeTerminalConnection() {
  if (!terminalSessionState) return;
  const st = terminalSessionState;
  st.disposed = true;
  if (st.resizeTimer) cancelAnimationFrame(st.resizeTimer);
  try { st.resizeObserver?.disconnect(); } catch {}
  try {
    if (st.ws && (st.ws.readyState === WebSocket.OPEN || st.ws.readyState === WebSocket.CONNECTING)) {
      st.ws.close(1000, 'closed');
    }
  } catch {}
  try { st.term?.dispose(); } catch {}
  terminalSessionState = null;
}

async function openTerminalDialog(vm) {
  disposeTerminalConnection();

  if (!terminalDialog) {
    terminalDialog = el('dialog', {
      className: 'app-dialog terminal-dialog',
      'aria-labelledby': 'terminal-dialog-title'
    });
    terminalDialog.addEventListener('cancel', (e) => {
      // Allow Esc key inside xterm instead of accidentally closing the shell window.
      if (document.activeElement && terminalDialog.contains(document.activeElement) && document.activeElement.closest('.xterm')) {
        e.preventDefault();
      }
    });
    terminalDialog.addEventListener('close', () => {
      terminalDialog.classList.remove('is-fullscreen');
      disposeTerminalConnection();
    });
    document.body.append(terminalDialog);
  }

  let sessionName = 'main';
  let isFullscreen = false;

  const titleEl = el('h2', { id: 'terminal-dialog-title', textContent: vm.name });
  const statusPill = el('span', { className: 'terminal-status-pill', dataset: { state: 'connecting' }, textContent: t('terminalStatusConnecting') });
  const subtitleEl = el('div', { className: 'hint mono', textContent: `${vm.accountAlias || vm.accountId} · ${vm.id}` });

  const sessionDatalist = el('datalist', { id: 'terminal-session-presets' }, [
    el('option', { value: 'main' }),
    el('option', { value: 'dev' }),
    el('option', { value: 'logs' }),
    el('option', { value: 'ops' })
  ]);
  const sessionInput = el('input', {
    type: 'text',
    className: 'terminal-session-input mono',
    value: sessionName,
    maxlength: '32',
    placeholder: t('terminalSessionPlaceholder'),
    list: 'terminal-session-presets',
    'aria-label': t('terminalSessionLabel')
  });
  const sessionWrap = el('label', { className: 'terminal-session-wrap' }, [
    el('span', { className: 'terminal-session-lbl', textContent: t('terminalSessionLabel') }),
    sessionInput,
    sessionDatalist
  ]);

  const reconnectBtn = el('button', {
    type: 'button',
    className: 'btn',
    textContent: t('terminalReconnect')
  });
  const restartBtn = el('button', {
    type: 'button',
    className: 'btn btn-danger',
    textContent: t('terminalRestartShell')
  });
  const fsIconHost = el('span', { style: 'display:inline-flex;' }, [ICONS.expand()]);
  const fsBtn = el('button', {
    type: 'button',
    className: 'btn btn-icon-only',
    title: t('terminalFullscreen'),
    'aria-label': t('terminalFullscreen')
  }, [fsIconHost]);
  const closeBtn = el('button', {
    type: 'button',
    className: 'btn btn-icon-only',
    title: t('close'),
    'aria-label': t('close'),
    onClick: () => terminalDialog.close()
  }, [ICONS.close()]);

  const header = el('div', { className: 'dialog-header terminal-head' }, [
    el('div', { className: 'terminal-head-left' }, [
      ICONS.terminal(),
      el('div', { style: 'min-inline-size:0;' }, [
        el('div', { className: 'terminal-title-row' }, [titleEl, statusPill]),
        subtitleEl
      ])
    ]),
    el('div', { className: 'terminal-controls' }, [
      sessionWrap,
      reconnectBtn,
      restartBtn,
      fsBtn,
      closeBtn
    ])
  ]);

  const bannerText = el('span');
  const banner = el('div', { className: 'terminal-banner', role: 'status', hidden: true }, [bannerText]);

  const stage = el('div', { className: 'terminal-stage' });
  const footerNote = el('span', { className: 'hint', textContent: t('terminalPersistentNote', { session: sessionName }) });
  const sizeBadge = el('span', { className: 'hint mono', textContent: '80×24' });

  const quickButtons = [
    { label: 'Esc', data: '\x1b' },
    { label: 'Tab', data: '\t' },
    { label: 'Ctrl+C', data: '\x03' },
    { label: 'Ctrl+D', data: '\x04' },
    { label: 'Ctrl+L', data: '\x0c' },
    { label: '↑', data: '\x1b[A' },
    { label: '↓', data: '\x1b[B' },
    { label: '←', data: '\x1b[D' },
    { label: '→', data: '\x1b[C' }
  ];
  const quickbar = el('div', { className: 'terminal-quickbar' },
    quickButtons.map((q) => el('button', {
      type: 'button',
      className: 'terminal-key-btn mono',
      textContent: q.label,
      onClick: () => {
        if (!terminalSessionState) return;
        terminalSessionState.sendBytes(q.data);
        terminalSessionState.term?.focus();
      }
    }))
  );
  quickbar.append(el('button', {
    type: 'button',
    className: 'terminal-key-btn',
    textContent: 'Clear',
    onClick: () => {
      terminalSessionState?.term?.clear();
      terminalSessionState?.term?.focus();
    }
  }));

  const body = el('div', { className: 'terminal-body' }, [
    banner,
    stage,
    quickbar,
    el('div', { className: 'terminal-footer' }, [footerNote, sizeBadge])
  ]);

  terminalDialog.replaceChildren(el('div', { className: 'dialog-inner' }, [header, body]));
  terminalDialog.classList.remove('is-fullscreen');
  if (!terminalDialog.open) terminalDialog.showModal();

  const setStatus = (stKey, labelText) => {
    statusPill.dataset.state = stKey;
    statusPill.textContent = labelText;
  };
  const showBanner = (msg, kind = 'error') => {
    bannerText.textContent = msg;
    banner.dataset.kind = kind;
    banner.hidden = false;
  };
  const hideBanner = () => {
    banner.hidden = true;
  };

  let TerminalCtor, FitAddonCtor;
  try {
    ({ Terminal: TerminalCtor, FitAddon: FitAddonCtor } = await loadXtermModules());
  } catch (err) {
    setStatus('error', t('terminalStatusError'));
    showBanner(err?.message || 'Failed to load xterm.js');
    return;
  }
  if (!terminalDialog.open) return;

  const term = new TerminalCtor({
    cursorBlink: true,
    fontFamily: '"Google Sans Code", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
    fontSize: 13.5,
    lineHeight: 1.32,
    scrollback: 5000,
    allowProposedApi: false,
    theme: {
      background: '#0b1020',
      foreground: '#e5e7eb',
      cursor: '#38bdf8',
      cursorAccent: '#0b1020',
      selectionBackground: 'rgba(56, 189, 248, 0.28)',
      black: '#1e293b',
      red: '#f87171',
      green: '#34d399',
      yellow: '#fbbf24',
      blue: '#60a5fa',
      magenta: '#c084fc',
      cyan: '#22d3ee',
      white: '#e2e8f0',
      brightBlack: '#64748b',
      brightRed: '#fca5a5',
      brightGreen: '#6ee7b7',
      brightYellow: '#fde047',
      brightBlue: '#93c5fd',
      brightMagenta: '#d8b4fe',
      brightCyan: '#67e8f9',
      brightWhite: '#f8fafc'
    }
  });
  const fitAddon = new FitAddonCtor();
  term.loadAddon(fitAddon);
  term.open(stage);

  const encoder = new TextEncoder();
  const stateObj = {
    disposed: false,
    term,
    fitAddon,
    ws: null,
    resizeObserver: null,
    resizeTimer: 0,
    sendBytes(text) {
      if (stateObj.ws && stateObj.ws.readyState === WebSocket.OPEN) {
        stateObj.ws.send(encoder.encode(text));
      }
    }
  };
  terminalSessionState = stateObj;

  const safeFit = () => {
    if (stateObj.disposed) return;
    try {
      fitAddon.fit();
      sizeBadge.textContent = `${term.cols}×${term.rows}`;
    } catch {}
  };
  requestAnimationFrame(() => {
    safeFit();
    term.focus();
  });

  if ('ResizeObserver' in window) {
    stateObj.resizeObserver = new ResizeObserver(() => {
      if (stateObj.resizeTimer) cancelAnimationFrame(stateObj.resizeTimer);
      stateObj.resizeTimer = requestAnimationFrame(safeFit);
    });
    stateObj.resizeObserver.observe(stage);
  }

  term.onData((data) => {
    if (stateObj.ws && stateObj.ws.readyState === WebSocket.OPEN) {
      stateObj.ws.send(encoder.encode(data));
    }
  });
  term.onBinary((raw) => {
    if (stateObj.ws && stateObj.ws.readyState === WebSocket.OPEN) {
      const buf = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) buf[i] = raw.charCodeAt(i) & 0xff;
      stateObj.ws.send(buf);
    }
  });
  term.onResize(({ cols, rows }) => {
    sizeBadge.textContent = `${cols}×${rows}`;
    if (stateObj.ws && stateObj.ws.readyState === WebSocket.OPEN) {
      stateObj.ws.send(JSON.stringify({ type: 'resize', cols, rows }));
    }
  });

  const connect = (restart = false) => {
    if (stateObj.disposed) return;
    const cand = sessionInput.value.trim().toLowerCase() || 'main';
    if (!/^[a-z0-9-]{1,32}$/.test(cand)) {
      showBanner(t('terminalInvalidSession'));
      sessionInput.focus();
      return;
    }
    sessionName = cand;
    sessionInput.value = sessionName;
    footerNote.textContent = t('terminalPersistentNote', { session: sessionName });

    if (stateObj.ws) {
      try {
        stateObj.ws.onopen = null;
        stateObj.ws.onmessage = null;
        stateObj.ws.onerror = null;
        stateObj.ws.onclose = null;
        stateObj.ws.close(1000, 'reconnect');
      } catch {}
      stateObj.ws = null;
    }

    hideBanner();
    safeFit();
    setStatus('connecting', t('terminalStatusConnecting'));
    if (restart) term.reset();

    const wsProto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const qs = new URLSearchParams({
      cols: String(Math.max(10, Math.min(1000, term.cols || 80))),
      rows: String(Math.max(4, Math.min(500, term.rows || 24))),
      session: sessionName
    });
    if (restart) qs.set('restart', '1');
    const wsUrl = `${wsProto}//${window.location.host}/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}/terminal?${qs.toString()}`;

    const sock = new WebSocket(wsUrl);
    sock.binaryType = 'arraybuffer';
    stateObj.ws = sock;

    let gotServerErr = false;
    let isSessionReady = false;

    sock.onopen = () => {
      if (stateObj.disposed || stateObj.ws !== sock) return;
      safeFit();
      sock.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      term.focus();
    };

    sock.onmessage = (ev) => {
      if (stateObj.disposed || stateObj.ws !== sock) return;
      if (ev.data instanceof ArrayBuffer) {
        if (!isSessionReady && !gotServerErr) {
          isSessionReady = true;
          setStatus('connected', t('terminalStatusConnected'));
          hideBanner();
        }
        term.write(new Uint8Array(ev.data));
        return;
      }
      if (typeof ev.data === 'string') {
        try {
          const msg = JSON.parse(ev.data);
          if (msg.type === 'session') {
            isSessionReady = true;
            setStatus('connected', t('terminalStatusConnected'));
            hideBanner();
            term.focus();
          } else if (msg.type === 'exit') {
            gotServerErr = true;
            const code = msg.code ?? 0;
            setStatus('disconnected', t('terminalStatusExited', { code: String(code) }));
            showBanner(t('terminalStatusExited', { code: String(code) }), 'info');
          } else if (msg.type === 'taken') {
            gotServerErr = true;
            setStatus('disconnected', t('terminalStatusTaken'));
            showBanner(t('terminalStatusTaken'), 'info');
          } else if (msg.type === 'error') {
            gotServerErr = true;
            setStatus('error', t('terminalStatusError'));
            showBanner(msg.message || t('terminalStatusError'));
          }
        } catch {
          term.write(ev.data);
        }
      }
    };

    sock.onerror = () => {
      if (stateObj.disposed || stateObj.ws !== sock || gotServerErr) return;
      setStatus('error', t('terminalStatusError'));
    };

    sock.onclose = (ev) => {
      if (stateObj.disposed || stateObj.ws !== sock) return;
      if (!gotServerErr) {
        setStatus('disconnected', t('terminalStatusDisconnected'));
        if (ev.code !== 1000) {
          showBanner(ev.reason || t('terminalStatusDisconnected'));
        }
      }
    };
  };

  reconnectBtn.addEventListener('click', () => connect(false));
  sessionInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      connect(false);
    }
  });
  restartBtn.addEventListener('click', () => {
    const s = sessionInput.value.trim().toLowerCase() || 'main';
    if (!window.confirm(t('terminalConfirmRestart', { session: s }))) return;
    connect(true);
  });
  fsBtn.addEventListener('click', () => {
    isFullscreen = !isFullscreen;
    terminalDialog.classList.toggle('is-fullscreen', isFullscreen);
    fsIconHost.replaceChildren(isFullscreen ? ICONS.compress() : ICONS.expand());
    const lbl = t(isFullscreen ? 'terminalExitFullscreen' : 'terminalFullscreen');
    fsBtn.title = lbl;
    fsBtn.setAttribute('aria-label', lbl);
    requestAnimationFrame(() => {
      safeFit();
      term.focus();
    });
  });

  connect(false);
}

// ============================================================
// Beszel-style Telemetry Detail Modal (persistent scroll container!)
// ============================================================
let telemetryDialog = null;
let telemetrySeq = 0;

function ensureTelemetryDialogShell() {
  if (!telemetryDialog) {
    telemetryDialog = el('dialog', {
      className: 'app-dialog',
      'aria-labelledby': 'telemetry-dialog-title'
    });
    enableDialogLightDismiss(telemetryDialog);
    telemetryDialog.addEventListener('close', () => {
      state.telemetryTarget = null;
    });
    document.body.append(telemetryDialog);
  }
  if (telemetryDialog._shell) return telemetryDialog._shell;

  const titleEl = el('h2', { id: 'telemetry-dialog-title' });
  const subtitleEl = el('div', { className: 'hint mono' });
  const termOpenBtn = el('button', {
    type: 'button',
    className: 'btn',
    onClick: () => {
      const curVm = telemetryDialog._currentVm;
      if (!curVm) return;
      telemetryDialog.close();
      openTerminalDialog(curVm);
    }
  }, [ICONS.terminal(), el('span', { textContent: t('openTerminal') })]);
  const header = el('div', { className: 'dialog-header' }, [
    el('div', {}, [titleEl, subtitleEl]),
    el('div', { className: 'dialog-header-actions' }, [
      termOpenBtn,
      el('button', {
        type: 'button',
        className: 'btn btn-icon-only',
        'aria-label': t('close'),
        onClick: () => {
          telemetryDialog.close();
          state.telemetryTarget = null;
        }
      }, [ICONS.close()])
    ])
  ]);

  const body = el('div', { className: 'dialog-body' });
  const toolbar = el('div', { className: 'telemetry-toolbar' });
  const seg = el('div', { className: 'segmented' });
  const rangeBtns = {};
  for (const r of ['15m', '1h', '6h', '24h', '7d']) {
    const b = el('button', {
      type: 'button',
      'aria-pressed': state.telemetryRange === r ? 'true' : 'false',
      textContent: r,
      onClick: async () => {
        state.telemetryRange = r;
        Object.entries(rangeBtns).forEach(([k, btn]) => {
          btn.setAttribute('aria-pressed', k === r ? 'true' : 'false');
        });
        await fetchAndRenderTelemetry();
      }
    });
    rangeBtns[r] = b;
    seg.append(b);
  }
  toolbar.append(seg);

  const cpuBox = el('div');
  const ramBox = el('div');
  const diskBox = el('div');
  const netBox = el('div');
  const grid = el('div', { className: 'telemetry-grid' }, [cpuBox, ramBox, diskBox, netBox]);

  body.append(toolbar, grid);
  const inner = el('div', { className: 'dialog-inner' }, [header, body]);
  telemetryDialog.replaceChildren(inner);

  telemetryDialog._shell = { titleEl, subtitleEl, termOpenBtn, body, rangeBtns, cpuBox, ramBox, diskBox, netBox };
  return telemetryDialog._shell;
}

// --- Schedule editor modal --------------------------------------------------
// Uizze-style: calm flat surface, one dominant primary action, explicit rows the
// user can add/edit/delete, inline validation copy — no decorative chrome.
let scheduleDialog = null;

function ensureScheduleDialogShell() {
  if (!scheduleDialog) {
    scheduleDialog = el('dialog', {
      className: 'app-dialog sched-dialog',
      'aria-labelledby': 'sched-dialog-title'
    });
    enableDialogLightDismiss(scheduleDialog);
    document.body.append(scheduleDialog);
  }
  if (scheduleDialog._shell) return scheduleDialog._shell;

  const titleEl = el('h2', { id: 'sched-dialog-title' });
  const subtitleEl = el('div', { className: 'hint mono' });
  const closeBtn = el('button', {
    type: 'button',
    className: 'btn btn-icon-only',
    'aria-label': t('close'),
    onClick: () => scheduleDialog.close()
  }, [ICONS.close()]);
  const header = el('div', { className: 'dialog-header' }, [
    el('div', {}, [titleEl, subtitleEl]),
    el('div', { className: 'dialog-header-actions' }, [closeBtn])
  ]);

  const descEl = el('p', { className: 'sched-modal-desc' });
  const rowsHost = el('div', { className: 'sched-rows' });
  const errEl = el('div', { className: 'sched-modal-error', hidden: true });

  const addBtn = el('button', { type: 'button', className: 'btn sched-add-btn' });
  const cancelBtn = el('button', {
    type: 'button', className: 'btn',
    onClick: () => scheduleDialog.close()
  });
  const saveBtn = el('button', { type: 'button', className: 'btn sched-save-btn' });
  const footer = el('div', { className: 'sched-modal-footer' }, [cancelBtn, saveBtn]);

  const body = el('div', { className: 'dialog-body' }, [descEl, rowsHost, addBtn, errEl, footer]);
  const inner = el('div', { className: 'dialog-inner' }, [header, body]);
  scheduleDialog.replaceChildren(inner);

  scheduleDialog._shell = { titleEl, subtitleEl, descEl, rowsHost, addBtn, errEl, cancelBtn, saveBtn };
  return scheduleDialog._shell;
}

function scheduleWindowRow(start, end, onDelete) {
  const startInput = el('input', {
    type: 'time', className: 'sched-time mono', value: start || '06:00',
    'aria-label': 'start'
  });
  const endInput = el('input', {
    type: 'time', className: 'sched-time mono', value: end || '08:00',
    'aria-label': 'end'
  });
  const delBtn = el('button', {
    type: 'button', className: 'btn btn-icon-only sched-del',
    'aria-label': t('schedDelete'),
    title: t('schedDelete'),
    onClick: onDelete
  }, [ICONS.close()]);
  const dash = el('span', { className: 'sched-dash', textContent: '→' });
  const row = el('div', { className: 'sched-row' }, [startInput, dash, endInput, delBtn]);
  row._get = () => ({ start: startInput.value, end: endInput.value });
  return row;
}

function validateWindowRows(rows) {
  const entries = [];
  for (const row of rows) {
    const { start, end } = row._get();
    if (!start || !end) return { error: t('schedErrEmpty') };
    entries.push({ start, end });
  }
  if (entries.length === 0) return { error: t('schedErrNeedOne') };
  // Sort by start for overlap detection (server re-validates authoritatively).
  const toMin = (s) => {
    const [h, m] = s.split(':').map(Number);
    return h * 60 + m;
  };
  const sorted = [...entries].sort((a, b) => toMin(a.start) - toMin(b.start));
  for (const w of sorted) {
    let endMin = toMin(w.end);
    if (endMin === 0) endMin = 1440;
    if (toMin(w.start) >= endMin) return { error: t('schedErrInverted') };
    if (endMin - toMin(w.start) < 5) return { error: t('schedErrTooShort') };
  }
  for (let i = 1; i < sorted.length; i++) {
    const prevEnd = toMin(sorted[i - 1].end) === 0 ? 1440 : toMin(sorted[i - 1].end);
    if (toMin(sorted[i].start) < prevEnd) return { error: t('schedErrOverlap') };
  }
  return { entries };
}

async function openScheduleDialog(vmRef) {
  // The card closure's `vm` object is never refreshed — always re-read the latest
  // record from state so edits start from what is actually saved, not from a
  // possibly ancient snapshot (which previously re-introduced windows the user
  // never configured or overwrote newer edits).
  const fresh = state.machines.find((x) => x.accountId === vmRef.accountId && x.id === vmRef.id);
  const vm = fresh || vmRef;
  const shell = ensureScheduleDialogShell();
  const { rowsHost, addBtn, errEl, cancelBtn, saveBtn, descEl } = shell;

  setTextIfChanged(shell.titleEl, `${vm.name} · ${t('schedTitle')}`);
  setTextIfChanged(shell.subtitleEl, `${vm.accountAlias || vm.accountId} · UTC+7`);
  setTextIfChanged(descEl, t('schedModalDesc'));
  setTextIfChanged(addBtn, `+ ${t('schedAdd')}`);
  setTextIfChanged(cancelBtn, t('cancel'));
  setTextIfChanged(saveBtn, t('schedSave'));
  errEl.hidden = true;

  // Seed rows from the stored windows.
  rowsHost.replaceChildren();
  const refreshSaveMode = () => {
    const empty = rowsHost.children.length === 0;
    saveBtn.dataset.mode = empty ? 'clear' : 'save';
    setTextIfChanged(saveBtn, empty ? t('schedClearBtn') : t('schedSave'));
    errEl.hidden = true;
  };
  const addRow = (start, end) => {
    const row = scheduleWindowRow(start, end, () => {
      row.remove();
      refreshSaveMode();
    });
    rowsHost.append(row);
    refreshSaveMode();
    return row;
  };
  let seeded = [];
  try { seeded = JSON.parse(vm.scheduleWindows || '[]'); } catch { seeded = []; }
  if (Array.isArray(seeded) && seeded.length > 0) {
    for (const w of seeded) addRow(w.start, w.end === '00:00' ? '00:00' : w.end);
  } else {
    // Only a machine that never had any window configured gets the starter row.
    addRow('06:00', '08:00');
  }
  refreshSaveMode();
  addBtn.onclick = () => {
    if (rowsHost.children.length >= 12) {
      errEl.hidden = false;
      setTextIfChanged(errEl, t('schedErrTooMany'));
      return;
    }
    errEl.hidden = true;
    addRow('18:00', '19:00');
  };

  saveBtn.onclick = async () => {
    const rows = [...rowsHost.children];
    let body;
    let toastKey;
    if (rows.length === 0) {
      // Explicitly cleared: disable the schedule and wipe the stored windows.
      body = { enabled: false, windows: '[]', tzOffsetMin: 420 };
      toastKey = 'schedClearedToast';
    } else {
      const { entries, error } = validateWindowRows(rows);
      if (error) {
        errEl.hidden = false;
        setTextIfChanged(errEl, error);
        return;
      }
      body = { enabled: true, windows: entries.map((w) => `${w.start}-${w.end}`).join(', '), tzOffsetMin: 420 };
      toastKey = 'schedEnabledToast';
    }
    errEl.hidden = true;
    saveBtn.disabled = true;
    setTextIfChanged(saveBtn, t('schedSaving'));
    try {
      const res = await apiFetch(
        `/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}/schedule`,
        { method: 'PUT', body: JSON.stringify(body) }
      );
      if (res?.machine) {
        const idx = state.machines.findIndex((x) => x.accountId === vm.accountId && x.id === vm.id);
        if (idx >= 0) state.machines[idx] = res.machine;
        updateDashboardInPlace();
      }
      showToast(`${t(toastKey)}: ${vm.name}`);
      scheduleDialog.close();
      setTimeout(() => refreshMachinesSilently(), 1200);
    } catch (err) {
      errEl.hidden = false;
      setTextIfChanged(errEl, err.message);
    } finally {
      saveBtn.disabled = false;
      refreshSaveMode();
    }
  };

  if (!scheduleDialog.open) scheduleDialog.showModal();
}

async function openTelemetryDialog(vm) {
  state.telemetryTarget = { accountId: vm.accountId, machineId: vm.id };
  const shell = ensureTelemetryDialogShell();
  telemetryDialog._currentVm = vm;
  const termAllowed = vm.state !== 'stopped' && vm.state !== 'stopping' && vm.state !== 'deleting';
  shell.termOpenBtn.disabled = !termAllowed;
  shell.termOpenBtn.title = termAllowed ? t('openTerminal') : t('terminalStoppedHint');
  setTextIfChanged(shell.termOpenBtn.lastChild, t('openTerminal'));
  setTextIfChanged(shell.titleEl, `${vm.name} · ${t('telemetryTitle')}`);
  setTextIfChanged(shell.subtitleEl, `${vm.accountAlias || vm.accountId} · ${vm.id}`);
  shell.body.scrollTop = 0;
  if (!telemetryDialog.open) telemetryDialog.showModal();
  await fetchAndRenderTelemetry();
}

async function fetchAndRenderTelemetry() {
  if (!state.telemetryTarget || !telemetryDialog) return;
  const { accountId, machineId } = state.telemetryTarget;
  const reqSeq = ++telemetrySeq;
  const reqRange = state.telemetryRange;
  try {
    const data = await apiFetch(
      `/api/machines/${encodeURIComponent(accountId)}/${encodeURIComponent(machineId)}/telemetry?range=${encodeURIComponent(reqRange)}`
    );
    if (
      reqSeq !== telemetrySeq ||
      !state.telemetryTarget ||
      state.telemetryTarget.accountId !== accountId ||
      state.telemetryTarget.machineId !== machineId ||
      state.telemetryRange !== reqRange ||
      !telemetryDialog?.open
    ) {
      return;
    }
    state.telemetrySamples = data.samples || [];
    renderTelemetryDialogContent(data.machine);
  } catch (err) {
    if (reqSeq === telemetrySeq) showToast(err.message, 'error');
  }
}

function renderTelemetryDialogContent(vm) {
  const shell = ensureTelemetryDialogShell();
  telemetryDialog._currentVm = vm;
  const termAllowed = vm.state !== 'stopped' && vm.state !== 'stopping' && vm.state !== 'deleting';
  shell.termOpenBtn.disabled = !termAllowed;
  shell.termOpenBtn.title = termAllowed ? t('openTerminal') : t('terminalStoppedHint');
  const prevScrollTop = shell.body.scrollTop;

  setTextIfChanged(shell.titleEl, `${vm.name} · ${t('telemetryTitle')}`);
  setTextIfChanged(shell.subtitleEl, `${vm.accountAlias || vm.accountId} · ${vm.id}`);
  Object.entries(shell.rangeBtns).forEach(([k, btn]) => {
    btn.setAttribute('aria-pressed', state.telemetryRange === k ? 'true' : 'false');
  });

  const chartLabels = {
    now: t('currentVal'),
    avg: t('avgVal'),
    peak: t('peakVal'),
    empty: t('noMetricsYet')
  };

  renderDetailChart(shell.cpuBox, {
    title: `${t('cpuLabel')} (%)`,
    unit: '%',
    samples: state.telemetrySamples,
    fixedMax: 100,
    series: [{ label: t('cpuLabel'), shortLabel: 'CPU', color: '#2563eb', getValue: (s) => s.cpuPct || 0 }],
    labels: chartLabels,
    locale: state.lang
  });

  renderDetailChart(shell.ramBox, {
    title: `${t('ramLabel')} (%)`,
    unit: '%',
    samples: state.telemetrySamples,
    fixedMax: 100,
    series: [{
      label: t('ramLabel'),
      shortLabel: 'RAM',
      color: '#10b981',
      getValue: (s) => (s.memTotal > 0 ? (s.memUsed / s.memTotal) * 100 : 0)
    }],
    labels: chartLabels,
    locale: state.lang
  });

  const rootStat = getVmMountStats(vm, '/');
  const dataStat = getVmMountStats(vm, '/data');
  renderDetailChart(shell.diskBox, {
    title: `${t('diskLabel')} (/ & /data)`,
    unit: '%',
    samples: state.telemetrySamples,
    fixedMax: 100,
    statPills: [
      [
        t('diskRootLabel'),
        rootStat.pct > 0
          ? `${rootStat.pct.toFixed(1)}%${rootStat.sizeBytes > 0 ? ` (${fmtBytes(rootStat.usedBytes)} / ${fmtBytes(rootStat.sizeBytes)})` : ''}`
          : '--'
      ],
      [
        t('diskDataLabel'),
        dataStat.pct > 0
          ? `${dataStat.pct.toFixed(1)}%${dataStat.sizeBytes > 0 ? ` (${fmtBytes(dataStat.usedBytes)} / ${fmtBytes(dataStat.sizeBytes)})` : ''}`
          : '--'
      ]
    ],
    series: [
      {
        label: t('diskRootLabel'),
        shortLabel: '/',
        color: '#ec4899',
        getValue: (s) => (s.rootTotal > 0 && s.rootUsed > 0 ? (s.rootUsed / s.rootTotal) * 100 : null)
      },
      {
        label: t('diskDataLabel'),
        shortLabel: '/data',
        color: '#f59e0b',
        getValue: (s) => (s.diskTotal > 0 && s.diskUsed > 0 ? (s.diskUsed / s.diskTotal) * 100 : null)
      }
    ],
    labels: chartLabels,
    locale: state.lang
  });

  renderDetailChart(shell.netBox, {
    title: `${t('netLabel')} & ${t('diskIoLabel')}`,
    unit: 'rate',
    samples: state.telemetrySamples,
    fixedMax: null,
    valueFormatter: fmtRate,
    series: [
      { label: 'Net In', shortLabel: 'In', color: '#06b6d4', getValue: (s) => s.netInBps || 0 },
      { label: 'Net Out', shortLabel: 'Out', color: '#8b5cf6', getValue: (s) => s.netOutBps || 0 },
      { label: 'Disk R/W', shortLabel: 'Disk', color: '#f97316', getValue: (s) => (s.diskReadBps || 0) + (s.diskWriteBps || 0) }
    ],
    labels: chartLabels,
    locale: state.lang
  });

  shell.body.scrollTop = prevScrollTop;
}

function applyMachinesPayload(data) {
  if (!data) return;
  if (typeof data.serverTimeMs === 'number') {
    state.serverDriftMs = data.serverTimeMs - Date.now();
  }
  if (typeof data.clockOffsetMs === 'number') {
    state.clockOffsetMs = data.clockOffsetMs;
  }
  if (Array.isArray(data.accounts)) {
    state.accounts = data.accounts;
  }
  if (Array.isArray(data.machines)) {
    state.machines = data.machines;
  }
}

async function loadDashboardData() {
  const data = await apiFetch('/api/machines');
  applyMachinesPayload(data);
}

function closeOpenDialogs() {
  try { if (navDrawer?.open) navDrawer.close(); } catch {}
  try { if (addMachineDialog?.open) addMachineDialog.close(); } catch {}
  try { if (renameMachineDialog?.open) renameMachineDialog.close(); } catch {}
  try { if (deleteMachineDialog?.open) deleteMachineDialog.close(); } catch {}
  try { if (accountsDialog?.open) accountsDialog.close(); } catch {}
  try { if (apiKeysDialog?.open) apiKeysDialog.close(); } catch {}
  try { if (telemetryDialog?.open) telemetryDialog.close(); } catch {}
  try { if (terminalDialog?.open) terminalDialog.close(); } catch {}
  disposeTerminalConnection();
  state.telemetryTarget = null;
  state.editingAccountId = null;
}

async function refreshMachinesSilently() {
  if (!state.authenticated || document.hidden) return;
  try {
    await loadDashboardData();
    if (!updateDashboardInPlace()) {
      render();
    }
    if (state.telemetryTarget && telemetryDialog?.open) {
      await fetchAndRenderTelemetry();
    }
  } catch (err) {
    if (err.status === 401) {
      state.authenticated = false;
      state.machineOptionsCache.clear();
      closeOpenDialogs();
      vmCardRegistry.clear();
      render();
    }
  }
}

function render() {
  document.querySelectorAll('.action-tooltip').forEach((tip) => {
    try { if (tip.matches(':popover-open')) tip.hidePopover(); } catch {}
  });
  document.documentElement.lang = state.lang;
  const app = document.getElementById('app');
  if (!app) return;
  vmCardRegistry.clear();
  app.replaceChildren(renderTopbar(), state.authenticated ? renderDashboard() : renderAuthView());
  renderDrawerContent();
}

async function boot() {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').catch(() => {});
  }

  window.addEventListener('storage', (e) => {
    if (e.key === 'color-scheme') {
      applyTheme(readTheme());
      renderDrawerContent();
    }
  });

  try {
    const status = await apiFetch('/api/auth/status');
    state.needsSetup = !!status.needsSetup;
    state.authenticated = !!status.authenticated;
    state.username = status.username || '';
    state.csrfToken = status.csrfToken || '';
    if (typeof status.serverTimeMs === 'number') {
      state.serverDriftMs = status.serverTimeMs - Date.now();
    }
    if (typeof status.clockOffsetMs === 'number') {
      state.clockOffsetMs = status.clockOffsetMs;
    }
    if (state.authenticated) {
      await loadDashboardData();
    }
  } catch (err) {
    showToast(err.message, 'error');
  }

  render();
  setInterval(updateLiveClocks, 1000);
  setInterval(refreshMachinesSilently, 15000);
}

boot();
