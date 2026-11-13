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
  download: () => svgIcon([
    { d: 'M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4' },
    { d: 'M7 10l5 5 5-5' },
    { d: 'M12 15V3' }
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

let deferredInstallPrompt = null;
const supportsWebInstall = typeof Navigator !== 'undefined' && typeof Navigator.prototype.install === 'function';
const supportsInterestFor = typeof HTMLButtonElement !== 'undefined' && 'interestForElement' in HTMLButtonElement.prototype;
const supportsHintPopover = (() => {
  try {
    const d = document.createElement('div');
    d.popover = 'hint';
    return d.popover === 'hint';
  } catch {
    return false;
  }
})();

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

function cycleTheme() {
  const order = ['system', 'light', 'dark'];
  const next = order[(order.indexOf(state.theme) + 1) % order.length];
  applyTheme(next);
  render();
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
    node.textContent = text;
    node.dataset.mode = mode;
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
      toggleBtn.textContent = '';
      toggleBtn.append(showing ? ICONS.eye() : ICONS.eyeOff());
    }
  }, [ICONS.eye()]);

  wrap.append(input, toggleBtn);
  const errEl = el('div', { id: `${id}-err`, className: 'error-msg', textContent: errorText });
  field.append(wrap, errEl);

  const sync = () => {
    const invalid = input.matches(':user-invalid') || field.dataset.customInvalid === 'true';
    input.setAttribute('aria-invalid', invalid ? 'true' : 'false');
    field.dataset.invalid = invalid ? 'true' : 'false';
    const ids = [hint ? `${id}-hint` : '', invalid ? `${id}-err` : ''].filter(Boolean);
    if (ids.length) input.setAttribute('aria-describedby', ids.join(' '));
    else input.removeAttribute('aria-describedby');
  };
  input.addEventListener('blur', sync);
  input.addEventListener('input', () => {
    delete field.dataset.customInvalid;
    input.setCustomValidity('');
    if (input.hasAttribute('aria-invalid')) sync();
  });
  return { field, input, sync };
}

function renderTopbar() {
  const header = el('header', { className: 'topbar' });
  const inner = el('div', { className: 'topbar-inner' });

  const brand = el('div', { className: 'brand' }, [
    el('img', { className: 'brand-logo', src: '/assets/logo.svg', alt: '', width: '36', height: '36' }),
    el('div', {}, [
      el('div', { className: 'brand-title', textContent: t('appName') }),
      el('div', { className: 'brand-sub', textContent: t('appSubtitle') })
    ])
  ]);

  const actions = el('div', { className: 'topbar-actions' });

  // Progressive enhancement PWA Install button
  const installBtn = el('button', {
    id: 'install-app',
    type: 'button',
    className: 'btn',
    hidden: !(supportsWebInstall || deferredInstallPrompt),
    onClick: async () => {
      if (installBtn.getAttribute('aria-disabled') === 'true') return;
      installBtn.setAttribute('aria-disabled', 'true');
      try {
        if (supportsWebInstall) {
          await navigator.install();
          installBtn.hidden = true;
        } else if (deferredInstallPrompt) {
          const ev = deferredInstallPrompt;
          deferredInstallPrompt = null;
          await ev.prompt();
          await ev.userChoice;
          installBtn.hidden = true;
        }
      } catch {} finally {
        installBtn.removeAttribute('aria-disabled');
      }
    }
  }, [ICONS.download(), el('span', { textContent: t('installApp') })]);
  actions.append(installBtn);

  // Language switcher (11 languages)
  const langSelect = el('select', {
    className: 'select-compact',
    'aria-label': t('language'),
    onChange: (e) => {
      state.lang = e.target.value;
      document.documentElement.lang = state.lang;
      writePref('ibmon-lang', state.lang);
      render();
    }
  });
  for (const l of LANGUAGES) {
    const opt = el('option', { value: l.code, textContent: l.label });
    if (l.code === state.lang) opt.selected = true;
    langSelect.append(opt);
  }
  actions.append(langSelect);

  // Theme toggle (System -> Light -> Dark)
  const themeIcon = state.theme === 'dark' ? ICONS.moon() : state.theme === 'light' ? ICONS.sun() : ICONS.monitor();
  const themeLabel = state.theme === 'dark' ? t('themeDark') : state.theme === 'light' ? t('themeLight') : t('themeSystem');
  actions.append(el('button', {
    type: 'button',
    className: 'btn',
    'aria-label': themeLabel,
    onClick: cycleTheme
  }, [themeIcon, el('span', { textContent: themeLabel })]));

  if (state.authenticated) {
    actions.append(el('button', {
      type: 'button',
      className: 'btn',
      onClick: async () => {
        try {
          const data = await apiFetch('/api/sync', { method: 'POST' });
          applyMachinesPayload(data);
          render();
        } catch (err) {
          showToast(err.message, 'error');
        }
      }
    }, [ICONS.refresh(), el('span', { textContent: t('refreshAll') })]));

    actions.append(el('button', {
      type: 'button',
      className: 'btn btn-primary',
      onClick: () => openAccountsDialog()
    }, [ICONS.accounts(), el('span', { textContent: `${t('accountsBtn')} (${state.accounts.length})` })]));

    actions.append(el('button', {
      type: 'button',
      className: 'btn btn-icon-only',
      'aria-label': t('logout'),
      title: t('logout'),
      onClick: async () => {
        try {
          await apiFetch('/api/auth/logout', { method: 'POST' });
        } catch {}
        closeOpenDialogs();
        state.authenticated = false;
        state.csrfToken = '';
        state.accounts = [];
        state.machines = [];
        render();
      }
    }, [ICONS.logout()]));
  }

  inner.append(brand, actions);
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

  // Username field
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

function makeActionButton(vm, actionKey, tooltipText, enabled, iconNode) {
  const vmComposite = `${vm.accountId}:${vm.id}`;
  const pendingKey = `${vmComposite}:${actionKey}`;
  const isPending = state.pendingActions.has(pendingKey);
  const tipId = `tip-${vm.accountId.slice(0, 6)}-${vm.id}-${actionKey}`.replace(/[^a-zA-Z0-9_-]/g, '');

  const slot = el('div', { className: 'vm-action-slot' });
  const btn = el('button', {
    type: 'button',
    className: `vm-action-btn${isPending ? ' is-pending' : ''}`,
    dataset: { action: actionKey },
    'aria-label': tooltipText,
    interestfor: tipId,
    disabled: !enabled || isPending,
    onClick: async (ev) => {
      ev.stopPropagation();
      if (btn.disabled) return;
      hideTooltip();
      state.pendingActions.add(pendingKey);
      render();
      try {
        const res = await apiFetch(
          `/api/machines/${encodeURIComponent(vm.accountId)}/${encodeURIComponent(vm.id)}/actions/${actionKey}`,
          { method: 'POST' }
        );
        showToast(`${t('actionSent')}: ${vm.name} (${actionKey})`);
        if (res?.machine) {
          const idx = state.machines.findIndex((x) => x.accountId === vm.accountId && x.id === vm.id);
          if (idx >= 0) state.machines[idx] = res.machine;
        }
        // Poll quickly to reflect state transition
        setTimeout(() => refreshMachinesSilently(), 1800);
        setTimeout(() => refreshMachinesSilently(), 4500);
      } catch (err) {
        const waitTxt = err.retryAfterSeconds ? ` (${err.retryAfterSeconds}s)` : '';
        showToast(`${err.message}${waitTxt}`, 'error');
      } finally {
        state.pendingActions.delete(pendingKey);
        render();
      }
    }
  }, [iconNode]);

  const tip = el('div', {
    id: tipId,
    className: 'action-tooltip',
    role: 'tooltip',
    popover: 'manual',
    textContent: tooltipText
  });

  let openTimer = null;
  let closeTimer = null;
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

  btn.removeAttribute('interestfor');
  btn.setAttribute('aria-describedby', tipId);
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
  return slot;
}

function getVmDiskPct(vm) {
  if (vm.disk?.filesystems?.length) {
    const dataFs = vm.disk.filesystems.find((f) => f.mount === '/data') || vm.disk.filesystems[0];
    if (dataFs && Number.isFinite(dataFs.usedPct)) return dataFs.usedPct;
  }
  const latest = vm.latestMetric;
  if (latest && latest.diskTotal > 0 && latest.diskUsed > 0) {
    return (latest.diskUsed / latest.diskTotal) * 100;
  }
  return 0;
}

function renderVmCard(vm) {
  const wrap = el('div', { className: 'vm-card-wrap' });
  const card = el('article', { className: 'vm-card' });
  const { tone, label } = getVmToneAndLabel(vm);

  // Column 1: Status Point Checker + Identity + Clock
  const col1 = el('div', { className: 'vm-col-meta' });
  const head = el('div', { className: 'vm-head' });

  const ident = el('div', { className: 'vm-identity' }, [
    el('span', { className: 'status-point', dataset: { tone }, 'aria-hidden': 'true' }),
    el('div', { className: 'vm-title-block' }, [
      el('button', {
        type: 'button',
        className: 'vm-name-btn',
        textContent: vm.name,
        onClick: () => openTelemetryDialog(vm)
      }),
      el('div', { className: 'vm-submeta' }, [
        el('span', { className: 'badge', textContent: vm.accountAlias || vm.accountId }),
        el('span', { className: 'mono', textContent: vm.id }),
        vm.templateId ? el('span', { className: 'badge', textContent: vm.templateId }) : null,
        vm.free ? el('span', { className: 'badge', textContent: t('freePlan') }) : null
      ])
    ])
  ]);

  const statusBadge = el('span', { className: 'badge', dataset: { tone }, textContent: label });
  head.append(ident, statusBadge);
  col1.append(head);

  const specTxt = t('specFmt', {
    vcpu: vm.vcpus,
    ram: `${(vm.memMiB / 1024).toFixed(vm.memMiB % 1024 === 0 ? 0 : 1)} GiB`,
    disk: vm.persistentDiskGiB
  });
  const policyTxt = vm.autoSuspendAfterSeconds
    ? `${t('idlePolicy')}: ${Math.round(vm.autoSuspendAfterSeconds / 60)}m`
    : t('alwaysOn');
  col1.append(el('div', { className: 'vm-submeta', textContent: `${specTxt} · ${policyTxt}` }));

  // Live Runtime Clock row (consistent 2-line layout across all states)
  const clockInfo = formatClockDisplay(vm);
  const clockRow = el('div', { className: 'vm-clock-row' });
  const clockLeft = el('div', { className: 'clock-group' }, [
    el('span', { className: 'clock-label-wrap' }, [
      ICONS.clock(),
      el('span', { className: 'hint', textContent: clockInfo.mode === 'cooldown' ? t('cooldownLeft') : t('runtimeClock') })
    ]),
    el('span', { className: 'clock-readout-wrap' }, [
      el('span', {
        className: 'clock-value mono',
        dataset: { clockVm: `${vm.accountId}:${vm.id}`, mode: clockInfo.mode },
        textContent: clockInfo.text
      }),
      vm.maxRunMinutes > 0 && clockInfo.mode === 'running'
        ? el('span', { className: 'clock-limit mono', textContent: `/ ${fmtHMS(vm.maxRunMinutes * 60)}` })
        : null
    ])
  ]);
  const clockRight = el('div', { className: 'clock-limit mono', textContent: `${t('monthlyUsage')}: ${fmtCompactDuration(vm.monthlyRunningSeconds || 0)}` });
  clockRow.append(clockLeft, clockRight);
  col1.append(clockRow);

  if (vm.lastErrorMsg) {
    col1.append(el('div', { className: 'account-err', textContent: `${vm.lastErrorCode || 'error'}: ${vm.lastErrorMsg}` }));
  }

  // Column 2: Mini Beszel Charts (CPU / RAM / Disk)
  const samples = vm.recentMetrics || [];
  const latest = vm.latestMetric || null;
  const cpuVals = samples.map((s) => s.cpuPct || 0);
  const ramVals = samples.map((s) => (s.memTotal > 0 ? (s.memUsed / s.memTotal) * 100 : 0));
  const diskVals = samples.map((s) => (s.diskTotal > 0 && s.diskUsed > 0 ? (s.diskUsed / s.diskTotal) * 100 : getVmDiskPct(vm)));

  const cpuNow = latest ? `${latest.cpuPct.toFixed(1)}%` : '--';
  const ramNow = latest && latest.memTotal > 0 ? `${((latest.memUsed / latest.memTotal) * 100).toFixed(0)}%` : '--';
  const diskPct = getVmDiskPct(vm);
  const diskNow = diskPct > 0 ? `${diskPct.toFixed(0)}%` : '--';

  const miniGrid = el('div', { className: 'mini-charts-grid' });
  for (const [lbl, valTxt, series, color] of [
    [t('cpuLabel'), cpuNow, cpuVals, '#2563eb'],
    [t('ramLabel'), ramNow, ramVals, '#10b981'],
    [t('diskLabel'), diskNow, diskVals, '#f59e0b']
  ]) {
    const box = el('button', {
      type: 'button',
      className: 'mini-chart-box',
      onClick: () => openTelemetryDialog(vm)
    }, [
      el('div', { className: 'mini-chart-top' }, [
        el('span', { className: 'mini-chart-lbl', textContent: lbl }),
        el('span', { className: 'mini-chart-val mono', textContent: valTxt })
      ]),
      buildMiniAreaChart(series, color, 100)
    ]);
    miniGrid.append(box);
  }

  // Column 3: 4 Icon-Only SVG Action Buttons (Wake, Pause, Start, Stop — Stop executes immediately!)
  const cooldownActive = vm.cooldownUntil && Date.parse(vm.cooldownUntil) > upstreamNowMs();
  const canWake = vm.state === 'suspended' && !cooldownActive;
  const canSuspend = vm.state === 'running';
  const canStart = (vm.state === 'stopped' || vm.state === 'suspended') && !cooldownActive;
  const canStop = vm.state === 'running';

  const actionsBar = el('div', { className: 'vm-actions' }, [
    makeActionButton(vm, 'wake', t('actWake'), canWake, ICONS.wake()),
    makeActionButton(vm, 'suspend', t('actPause'), canSuspend, ICONS.suspend()),
    makeActionButton(vm, 'start', t('actStart'), canStart, ICONS.start()),
    makeActionButton(vm, 'stop', t('actStop'), canStop, ICONS.stop())
  ]);

  card.append(col1, miniGrid, actionsBar);
  wrap.append(card);
  return wrap;
}

function populateStatsStrip(strip) {
  strip.textContent = '';
  const total = state.machines.length;
  const running = state.machines.filter((m) => m.state === 'running').length;
  const paused = state.machines.filter((m) => m.state === 'suspended').length;
  const stopped = state.machines.filter((m) => m.state === 'stopped').length;
  for (const [lbl, val, tone] of [
    [t('totalVMs'), total, ''],
    [t('runningVMs'), running, 'running'],
    [t('pausedVMs'), paused, 'idle'],
    [t('stoppedVMs'), stopped, 'stopped']
  ]) {
    const tile = el('div', { className: 'stat-tile' }, [
      el('span', { className: 'stat-tile-label', textContent: lbl }),
      el('span', {
        className: 'stat-tile-value mono',
        style: tone === 'running' ? 'color:var(--ok)' : tone === 'idle' ? 'color:var(--warn)' : '',
        textContent: String(val)
      })
    ]);
    strip.append(tile);
  }
}

function renderDashboard() {
  const main = el('main', { className: 'main-wrap' });

  // Summary Metrics Strip
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
        textContent: t('addFirstAccount'),
        onClick: () => openAccountsDialog()
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
    className: 'select-compact',
    'aria-label': t('allAccounts'),
    onChange: (e) => {
      state.filterAccount = e.target.value;
      render();
    }
  }, [el('option', { value: 'all', textContent: t('allAccounts') })]);
  for (const a of state.accounts) {
    const opt = el('option', { value: a.id, textContent: a.alias });
    if (state.filterAccount === a.id) opt.selected = true;
    accSelect.append(opt);
  }

  const stateSelect = el('select', {
    className: 'select-compact',
    'aria-label': t('allStates'),
    onChange: (e) => {
      state.filterState = e.target.value;
      render();
    }
  }, [
    el('option', { value: 'all', textContent: t('allStates') }),
    el('option', { value: 'running', textContent: t('statusRunning') }),
    el('option', { value: 'suspended', textContent: t('statusSuspended') }),
    el('option', { value: 'stopped', textContent: t('statusStopped') })
  ]);
  stateSelect.value = state.filterState;

  const viewSeg = el('div', { className: 'segmented', role: 'group' }, [
    el('button', {
      type: 'button',
      'aria-pressed': state.viewMode === 'grid' ? 'true' : 'false',
      'aria-label': t('viewGrid'),
      onClick: () => {
        state.viewMode = 'grid';
        writePref('ibmon-view', 'grid');
        render();
      }
    }, [ICONS.grid(), el('span', { textContent: t('viewGrid') })]),
    el('button', {
      type: 'button',
      'aria-pressed': state.viewMode === 'cards' ? 'true' : 'false',
      'aria-label': t('viewCards'),
      onClick: () => {
        state.viewMode = 'cards';
        writePref('ibmon-view', 'cards');
        render();
      }
    }, [ICONS.cards(), el('span', { textContent: t('viewCards') })])
  ]);

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
  document.querySelectorAll('.action-tooltip').forEach((tip) => {
    try { if (tip.matches(':popover-open')) tip.hidePopover(); } catch {}
  });
  host.textContent = '';
  const items = getFilteredMachines();
  if (items.length === 0) {
    host.append(el('div', { className: 'empty-state' }, [
      el('h3', { textContent: t('noMachinesTitle') }),
      el('p', { textContent: t('noMachinesDesc') })
    ]));
    return;
  }
  const container = el('div', {
    className: state.viewMode === 'cards' ? 'vm-container-cards' : 'vm-container-grid'
  });
  for (const vm of items) {
    container.append(renderVmCard(vm));
  }
  host.append(container);
}

function renderVmListOnly() {
  const host = document.getElementById('vm-list-host');
  if (host) populateVmList(host);
}

// Accounts Manager Dialog
let accountsDialog = null;
function openAccountsDialog() {
  if (!accountsDialog) {
    accountsDialog = el('dialog', { className: 'app-dialog' });
    document.body.append(accountsDialog);
  }
  renderAccountsDialogContent();
  if (!accountsDialog.open) accountsDialog.showModal();
}

function renderAccountsDialogContent() {
  if (!accountsDialog) return;
  accountsDialog.textContent = '';

  const inner = el('div', { className: 'dialog-inner' });
  const header = el('div', { className: 'dialog-header' }, [
    el('h2', { textContent: t('accountsTitle') }),
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
            const data = await apiFetch(`/api/accounts/${encodeURIComponent(acc.id)}/sync`, { method: 'POST' });
            applyMachinesPayload(data);
            renderAccountsDialogContent();
            render();
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

  // Add / Edit Account form
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
      await loadDashboardData();
      renderAccountsDialogContent();
      render();
    } catch (err) {
      errBanner.textContent = err.message;
      errBanner.hidden = false;
    } finally {
      saveBtn.removeAttribute('aria-disabled');
    }
  });

  body.append(formTitle, errBanner, form);
  inner.append(header, body);
  accountsDialog.append(inner);
}

// Beszel-style Telemetry Detail Modal
let telemetryDialog = null;
let telemetrySeq = 0;
async function openTelemetryDialog(vm) {
  state.telemetryTarget = { accountId: vm.accountId, machineId: vm.id };
  if (!telemetryDialog) {
    telemetryDialog = el('dialog', { className: 'app-dialog' });
    document.body.append(telemetryDialog);
  }
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
  telemetryDialog.textContent = '';
  const inner = el('div', { className: 'dialog-inner' });
  const header = el('div', { className: 'dialog-header' }, [
    el('div', {}, [
      el('h2', { textContent: `${vm.name} · ${t('telemetryTitle')}` }),
      el('div', { className: 'hint mono', textContent: `${vm.accountAlias || vm.accountId} · ${vm.id}` })
    ]),
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
  for (const r of ['15m', '1h', '6h', '24h', '7d']) {
    seg.append(el('button', {
      type: 'button',
      'aria-pressed': state.telemetryRange === r ? 'true' : 'false',
      textContent: r,
      onClick: async () => {
        state.telemetryRange = r;
        await fetchAndRenderTelemetry();
      }
    }));
  }
  toolbar.append(seg);
  body.append(toolbar);

  const grid = el('div', { className: 'telemetry-grid' });
  const chartLabels = {
    now: t('currentVal'),
    avg: t('avgVal'),
    peak: t('peakVal'),
    empty: t('noMetricsYet')
  };

  const cpuBox = el('div');
  renderDetailChart(cpuBox, {
    title: `${t('cpuLabel')} (%)`,
    unit: '%',
    samples: state.telemetrySamples,
    fixedMax: 100,
    series: [{ label: t('cpuLabel'), shortLabel: 'CPU', color: '#2563eb', getValue: (s) => s.cpuPct || 0 }],
    labels: chartLabels,
    locale: state.lang
  });

  const ramBox = el('div');
  renderDetailChart(ramBox, {
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

  const diskBox = el('div');
  renderDetailChart(diskBox, {
    title: `${t('diskLabel')} (/data & /)`,
    unit: '%',
    samples: state.telemetrySamples,
    fixedMax: 100,
    series: [
      {
        label: '/data',
        shortLabel: '/data',
        color: '#f59e0b',
        getValue: (s) => (s.diskTotal > 0 && s.diskUsed > 0 ? (s.diskUsed / s.diskTotal) * 100 : null)
      },
      {
        label: '/',
        shortLabel: '/',
        color: '#ec4899',
        getValue: (s) => (s.rootTotal > 0 && s.rootUsed > 0 ? (s.rootUsed / s.rootTotal) * 100 : null)
      }
    ],
    labels: chartLabels,
    locale: state.lang
  });

  const netBox = el('div');
  renderDetailChart(netBox, {
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

  grid.append(cpuBox, ramBox, diskBox, netBox);
  body.append(grid);
  inner.append(header, body);
  telemetryDialog.append(inner);
}

function applyMachinesPayload(data) {
  if (!data) return;
  if (typeof data.serverTimeMs === 'number') {
    state.serverDriftMs = data.serverTimeMs - Date.now();
  }
  if (typeof data.clockOffsetMs === 'number') {
    state.clockOffsetMs = data.clockOffsetMs;
  }
  state.accounts = data.accounts || [];
  state.machines = data.machines || [];
}

async function loadDashboardData() {
  const data = await apiFetch('/api/machines');
  applyMachinesPayload(data);
}

function closeOpenDialogs() {
  try { if (accountsDialog?.open) accountsDialog.close(); } catch {}
  try { if (telemetryDialog?.open) telemetryDialog.close(); } catch {}
  state.telemetryTarget = null;
  state.editingAccountId = null;
}

async function refreshMachinesSilently() {
  if (!state.authenticated || document.hidden) return;
  try {
    await loadDashboardData();
    const strip = document.getElementById('stats-strip');
    const host = document.getElementById('vm-list-host');
    if (strip && host) {
      populateStatsStrip(strip);
      populateVmList(host);
    } else {
      render();
    }
    if (state.telemetryTarget && telemetryDialog?.open) {
      await fetchAndRenderTelemetry();
    }
  } catch (err) {
    if (err.status === 401) {
      state.authenticated = false;
      closeOpenDialogs();
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
  app.textContent = '';
  app.append(renderTopbar());
  app.append(state.authenticated ? renderDashboard() : renderAuthView());
}

async function boot() {
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js').catch(() => {});
  }
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault();
    deferredInstallPrompt = e;
    const btn = document.getElementById('install-app');
    if (btn && !supportsWebInstall) btn.hidden = false;
  });

  window.addEventListener('storage', (e) => {
    if (e.key === 'color-scheme') {
      applyTheme(readTheme());
      render();
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
