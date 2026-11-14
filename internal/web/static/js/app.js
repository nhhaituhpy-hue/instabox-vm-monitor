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
      return { tone: 'error', label: t('statusCooldown') };
    }
  }
  switch (vm.state) {
    case 'running':
      return vm.ready
        ? { tone: 'running', label: t('statusRunning') }
        : { tone: 'booting', label: t('statusBooting') };
    case 'starting':
    case 'resuming':
    case 'creating':
      return { tone: 'booting', label: t('statusBooting') };
    case 'suspended':
      return vm.autoSuspendedAt
        ? { tone: 'idle', label: t('statusIdleSuspended') }
        : { tone: 'suspended', label: t('statusSuspended') };
    case 'stopping':
    case 'suspending':
    case 'deleting':
      return { tone: 'stopping', label: t('statusStopping') };
    case 'error':
      return { tone: 'error', label: t('statusError') };
    default:
      return { tone: 'stopped', label: t('statusStopped') };
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

async function triggerSyncAll() {
  try {
    state.machineOptionsCache.clear();
    const data = await apiFetch('/api/sync', { method: 'POST' });
    applyMachinesPayload(data);
    updateDashboardInPlace();
    renderDrawerContent();
  } catch (err) {
    showToast(err.message, 'error');
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

  const brand = el('div', { className: 'brand' }, [
    el('img', { className: 'brand-logo', src: '/assets/logo.svg', alt: '', width: '32', height: '32' }),
    el('span', { className: 'brand-title', textContent: t('appName') })
  ]);

  const left = el('div', { className: 'topbar-left' }, [menuBtn, brand]);
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
  const nameRow = el('div', { className: 'vm-name-row' }, [nameBtn, renameBtn, deleteBtn]);

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

  const statusBadge = el('span', { className: 'badge' });
  head.append(ident, statusBadge);
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
    const { tone, label } = getVmToneAndLabel(vm);

    if (statusPoint.dataset.tone !== tone) statusPoint.dataset.tone = tone;
    if (statusBadge.dataset.tone !== tone) statusBadge.dataset.tone = tone;
    setTextIfChanged(statusBadge, label);

    setTextIfChanged(nameBtn, vm.name);
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
    setTextIfChanged(clockLabelEl, clockInfo.mode === 'cooldown' ? t('cooldownLeft') : t('runtimeClock'));
    setTextIfChanged(clockValueEl, clockInfo.text);
    if (clockValueEl.dataset.mode !== clockInfo.mode) clockValueEl.dataset.mode = clockInfo.mode;

    if (vm.maxRunMinutes > 0 && clockInfo.mode === 'running') {
      clockLimitEl.hidden = false;
      setTextIfChanged(clockLimitEl, `/ ${fmtHMS(vm.maxRunMinutes * 60)}`);
    } else {
      clockLimitEl.hidden = true;
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
// Modern KPI Cards (Subgrid-aligned, in-place updates)
// ============================================================
function ensureKpiCards(strip) {
  if (strip._kpi) return strip._kpi;

  const makeCard = ({ tone = '', showLivePill = false }) => {
    const card = el('article', { className: 'kpi-card' });

    const rule = el('span', { className: 'kpi-accent-rule', dataset: tone ? { tone } : {} });
    const lbl = el('span', { className: 'kpi-label' });
    const eyebrow = el('div', { className: 'kpi-eyebrow' }, [rule, lbl]);

    const head = el('header', { className: 'kpi-head' }, [eyebrow]);
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
    const foot = el('p', { className: 'kpi-foot' });

    card.append(head, val, track, foot);
    return { card, lbl, val, track, foot, liveText: card._liveText };
  };

  const cTotal = makeCard({ showLivePill: true });
  const segRun = el('span', { className: 'kpi-fill', dataset: { tone: 'running' } });
  const segIdle = el('span', { className: 'kpi-fill', dataset: { tone: 'idle' } });
  const segStop = el('span', { className: 'kpi-fill', dataset: { tone: 'stopped' } });
  const segErr = el('span', { className: 'kpi-fill', dataset: { tone: 'error' } });
  cTotal.track.append(segRun, segIdle, segStop, segErr);

  const cRun = makeCard({ tone: 'running' });
  const runFill = el('span', { className: 'kpi-fill', dataset: { tone: 'running' } });
  cRun.track.append(runFill);

  const cPause = makeCard({ tone: 'idle' });
  const pauseFill = el('span', { className: 'kpi-fill', dataset: { tone: 'idle' } });
  cPause.track.append(pauseFill);

  const cStop = makeCard({ tone: 'stopped' });
  const stopFill = el('span', { className: 'kpi-fill', dataset: { tone: 'stopped' } });
  cStop.track.append(stopFill);

  strip.replaceChildren(cTotal.card, cRun.card, cPause.card, cStop.card);
  strip._kpi = { cTotal, segRun, segIdle, segStop, segErr, cRun, runFill, cPause, pauseFill, cStop, stopFill };
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
}

function updateDashboardHero(heroEl) {
  const hero = heroEl || document.getElementById('dashboard-hero');
  if (!hero || !hero._refs) return;
  const { titleText, metaEl, addBtnText, syncBtnText } = hero._refs;

  const total = state.machines.length;
  const running = state.machines.filter((m) => m.state === 'running').length;
  let heading = t('heroAllStable');
  if (state.accounts.length === 0) heading = t('heroNoAccounts');
  else if (running > 0) heading = t('heroActive', { count: running });

  setTextIfChanged(titleText, heading.replace(/\.+$/, ''));
  setTextIfChanged(metaEl, t('heroMeta', { accounts: state.accounts.length, machines: total }));
  setTextIfChanged(addBtnText, t('addMachineBtn'));
  setTextIfChanged(syncBtnText, `${t('refreshAll')} →`);
}

function renderDashboard() {
  const main = el('main', { className: 'main-wrap' });

  // Editorial Hero Header + Pill Actions (CLIProxyAPI-inspired)
  const titleText = el('span');
  const heroTitle = el('h1', { className: 'hero-title' }, [
    titleText,
    el('span', { className: 'hero-dot', textContent: '.' })
  ]);
  const metaEl = el('p', { className: 'hero-meta mono' });
  const addBtnText = el('span');
  const syncBtnText = el('span');

  const hero = el('section', { id: 'dashboard-hero', className: 'dashboard-hero' }, [
    el('div', { className: 'hero-text' }, [heroTitle, metaEl]),
    el('div', { className: 'hero-actions' }, [
      el('button', {
        type: 'button',
        className: 'btn-pill-dark',
        onClick: () => openAddMachineDialog()
      }, [ICONS.plus(), addBtnText]),
      el('button', {
        type: 'button',
        className: 'btn-ghost-link',
        onClick: () => triggerSyncAll()
      }, [syncBtnText])
    ])
  ]);
  hero._refs = { titleText, metaEl, addBtnText, syncBtnText };
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
    className: 'select-compact',
    'aria-label': t('allStates'),
    onChange: (e) => {
      state.filterState = e.target.value;
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

function getFilteredMachines() {
  const q = state.searchQuery.trim().toLowerCase();
  return state.machines.filter((vm) => {
    if (state.filterAccount !== 'all' && vm.accountId !== state.filterAccount) return false;
    if (state.filterState !== 'all' && vm.state !== state.filterState) return false;
    if (q) {
      const hay = `${vm.name} ${vm.id} ${vm.templateId} ${vm.accountAlias || ''}`.toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });
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
  const header = el('div', { className: 'dialog-header' }, [
    el('div', {}, [titleEl, subtitleEl]),
    el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('close'),
      onClick: () => {
        telemetryDialog.close();
        state.telemetryTarget = null;
      }
    }, [ICONS.close()])
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

  telemetryDialog._shell = { titleEl, subtitleEl, body, rangeBtns, cpuBox, ramBox, diskBox, netBox };
  return telemetryDialog._shell;
}

async function openTelemetryDialog(vm) {
  state.telemetryTarget = { accountId: vm.accountId, machineId: vm.id };
  const shell = ensureTelemetryDialogShell();
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
  try { if (telemetryDialog?.open) telemetryDialog.close(); } catch {}
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
