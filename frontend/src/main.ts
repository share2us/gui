import './style.css';

// The Go backend (app.go) is injected by Wails at runtime as window.go.main.App.
// We call it directly so the frontend does not depend on regenerated bindings.
type Status = {
  loggedIn: boolean;
  email: string;
  isApiToken: boolean;
  canReceive: boolean;
  shellInstalled: boolean;
  autostartEnabled: boolean;
};
type Device = {
  sessionId: string;
  name: string;
  label: string;
  publicKey: string;
  hasKey: boolean;
  current: boolean;
};
type ShareRequest = {
  paths: string[];
  target: string;
  recipients?: string[];
  email?: string;
  deviceId?: string;
  devicePub?: string;
  password?: string;
  oneTime?: boolean;
  expires?: string;
  keep?: boolean;
  allowReshare?: boolean;
  note?: string;
};
type ShareOutcome = {
  path: string;
  ok: boolean;
  link?: string;
  publicId?: string;
  error?: string;
};

type LoginInfo = { userCode: string; verificationUrl: string; verificationUri: string };
type UpdateInfo = {
  available: boolean;
  current: string;
  latest: string;
  assetUrl: string;
  assetName: string;
  page: string;
};
type DeviceAccess = {
  sessionId: string;
  label: string;
  current: boolean;
  hasKey: boolean;
  exposed: boolean;
};

interface AppBackend {
  Status(): Promise<Status>;
  PendingPaths(): Promise<string[]>;
  ListDevices(): Promise<Device[]>;
  Share(req: ShareRequest): Promise<ShareOutcome[]>;
  InstallShell(): Promise<void>;
  BeginLogin(): Promise<LoginInfo>;
  CompleteLogin(): Promise<Status>;
  SetAutostart(on: boolean): Promise<void>;
  SetShellIntegration(on: boolean): Promise<void>;
  DeviceAccessForContact(email: string): Promise<DeviceAccess[]>;
  SetDeviceAccess(email: string, sessionId: string, exposed: boolean): Promise<void>;
  Logout(): Promise<void>;
  AddPasted(ext: string, dataB64: string): Promise<string>;
  CheckUpdate(): Promise<UpdateInfo>;
  ApplyUpdate(): Promise<void>;
}
const backend = (): AppBackend => (window as any).go.main.App;

type Target = 'public' | 'private' | 'device' | 'network';

const state = {
  paths: [] as string[],
  status: null as Status | null,
  devices: [] as Device[],
  target: 'public' as Target,
  selectedDevice: '' as string,
  loginPhase: 'idle' as 'idle' | 'waiting' | 'error',
  loginInfo: null as LoginInfo | null,
  loginError: '' as string,
  settingsOpen: false as boolean,
  view: 'share' as 'share' | 'trust',
  trustEmail: '' as string,
  trustDevices: null as DeviceAccess[] | null,
  trustError: '' as string,
  theme: 'dark' as 'dark' | 'light',
  update: null as UpdateInfo | null,
};

// Brand mark (S2u), inlined so it needs no external asset under the strict CSP.
const BRAND_SVG =
  `<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#2fd6b6"/><stop offset="1" stop-color="#17a68c"/></linearGradient></defs><rect width="24" height="24" rx="6" fill="url(#g)"/><text x="12" y="16.5" font-family="system-ui,sans-serif" font-size="10" font-weight="700" text-anchor="middle" fill="#0b2b26">S2U</text></svg>`;

function applyTheme() {
  document.documentElement.setAttribute('data-theme', state.theme);
}
function initTheme() {
  const saved = localStorage.getItem('s2u-theme');
  state.theme = saved === 'light' ? 'light' : 'dark';
  applyTheme();
}
function toggleTheme() {
  state.theme = state.theme === 'dark' ? 'light' : 'dark';
  localStorage.setItem('s2u-theme', state.theme);
  applyTheme();
  render();
}

const root = document.querySelector<HTMLDivElement>('#app')!;

async function logout() {
  try {
    await backend().Logout();
  } catch (e) {
    /* ignore */
  }
  try {
    state.status = await backend().Status();
  } catch {}
  state.devices = [];
  state.view = 'share';
  state.settingsOpen = false;
  render();
}

async function boot() {
  initTheme();
  root.innerHTML = `<div class="loading">Loading…</div>`;
  try {
    const [status, paths] = await Promise.all([
      backend().Status(),
      backend().PendingPaths(),
    ]);
    state.status = status;
    state.paths = paths || [];
    render();
    setupInputListeners();
    checkForUpdate();
  } catch (e) {
    root.innerHTML = `<div class="error-box">Could not start: ${escapeHtml(String(e))}</div>`;
  }
}

function render() {
  if (state.view === 'trust') {
    renderTrust();
    return;
  }
  renderShare();
}

function renderShare() {
  const s = state.status!;
  root.innerHTML = `
    <div class="modal">
      <header class="modal-head">
        <div class="brand">${BRAND_SVG}<span>Share2Us</span></div>
        <div class="head-actions">
          <button class="icon-btn" id="theme-toggle" title="Toggle light/dark">${state.theme === 'dark' ? '☀' : '☾'}</button>
          ${
            s.loggedIn
              ? `<span class="who" title="${escapeHtml(s.email)}">${escapeHtml(s.email)}</span><button class="icon-btn" id="logout-btn" title="Sign out">⏻</button>`
              : `<span class="who">not signed in</span>`
          }
        </div>
      </header>
      ${updateBanner()}
      ${s.loggedIn ? '' : signInBanner()}
      <div class="modal-body">
        ${filesBlock()}
        <nav class="tabs">
          ${tab('public', 'Public link')}
          ${tab('private', 'Private (email)')}
          ${tab('device', 'Send to device')}
          ${tab('network', 'Network', true)}
        </nav>
        <section class="panel" id="panel">${panel()}</section>
        <div class="results" id="results"></div>
        ${settingsBlock()}
      </div>
      <footer class="modal-foot">
        ${canShare() ? '' : `<div class="foot-reason">${escapeHtml(shareDisabledReason())}</div>`}
        <button class="btn-primary" id="share-btn" ${
          canShare() ? '' : `disabled title="${escapeHtml(shareDisabledReason())}"`
        }>Share</button>
      </footer>
    </div>`;
  wire();
}

function updateBanner(): string {
  const u = state.update;
  if (!u?.available) return '';
  return `<div class="update-bar">
    <span>Update available — <strong>v${escapeHtml(u.latest)}</strong></span>
    <button class="btn-mini" id="apply-update">Install</button>
  </div>`;
}

async function checkForUpdate() {
  try {
    const info = await backend().CheckUpdate();
    if (info?.available) {
      state.update = info;
      render();
    }
  } catch {}
}

function signInBanner(): string {
  if (state.loginPhase === 'waiting' && state.loginInfo) {
    const info = state.loginInfo;
    return `<div class="banner">
      Approve this device in your browser${info.userCode ? ` — code <code>${escapeHtml(info.userCode)}</code>` : ''}.
      <span class="hint">Waiting for approval…</span>
      ${info.verificationUrl ? `<button class="btn-mini" id="reopen-login">Reopen page</button>` : ''}
    </div>`;
  }
  const err = state.loginPhase === 'error' && state.loginError
    ? `<div class="banner-err">${escapeHtml(state.loginError)}</div>`
    : '';
  return `<div class="banner">
    Sign in to share. <button class="btn-mini" id="sign-in">Sign in</button>
    <span class="hint">opens your browser</span>
  </div>${err}`;
}

function filesBlock(): string {
  if (!state.paths.length) {
    return `<div class="canvas" id="canvas">
      <div class="canvas-ico">⬍</div>
      <div class="canvas-title">Drop a file here, or paste with Ctrl+V</div>
      <div class="canvas-sub">Screenshots, images, and text work too — or right-click a file in your file manager → s2u → Share.</div>
    </div>`;
  }
  return `<div class="files">${state.paths
    .map(
      (p, i) =>
        `<div class="file-chip" title="${escapeHtml(p)}">${escapeHtml(basename(p))}<button class="chip-x" data-i="${i}" title="Remove">×</button></div>`
    )
    .join('')}<span class="files-add" title="Paste (Ctrl+V) or drop to add more">＋ paste / drop</span></div>`;
}

function tab(t: Target, label: string, soon = false): string {
  const active = state.target === t ? 'active' : '';
  const dis = soon ? 'soon' : '';
  return `<button class="tab ${active} ${dis}" data-tab="${t}" ${soon ? 'disabled' : ''}>${label}${
    soon ? ' <span class="pill">soon</span>' : ''
  }</button>`;
}

function panel(): string {
  switch (state.target) {
    case 'public':
      return `
        ${noteRow()}
        ${expiryRow()}
        ${checkRow('one-time', 'One-time (delete after first download)')}
        ${passwordRow()}`;
    case 'private':
      return `
        <label class="fld">Recipient emails <span class="hint">comma-separated</span>
          <input id="recipients" type="text" placeholder="alice@example.com, bob@example.com" />
        </label>
        ${noteRow()}
        ${expiryRow()}
        ${checkRow('allow-reshare', 'Allow recipients to reshare')}
        ${passwordRow()}`;
    case 'device':
      return devicePanel();
    case 'network':
      return `<div class="soon-note">Send over the local network (LAN/Tailscale) is coming next.</div>`;
  }
}

function devicePanel(): string {
  if (state.status?.isApiToken) {
    return `<div class="soon-note">Device sends need an interactive login (not an API token).</div>`;
  }
  if (!state.devices.length) {
    return `<div class="soon-note" id="dev-loading">Loading your devices…</div>`;
  }
  const opts = state.devices
    .map((d) => {
      const dis = d.hasKey ? '' : 'disabled';
      const cur = d.current ? ' (this device)' : '';
      const noKey = d.hasKey ? '' : ' — no key yet';
      return `<option value="${d.sessionId}" data-pub="${escapeHtml(d.publicKey)}" ${dis}>${escapeHtml(
        d.label
      )}${cur}${noKey}</option>`;
    })
    .join('');
  return `
    <label class="fld">Device
      <select id="device-select">${opts}</select>
    </label>
    <div class="hint">The file lands in that device's Downloads folder, end-to-end encrypted.</div>`;
}

function expiryRow(): string {
  return `
    <label class="fld">Expires
      <select id="expires">
        <option value="">Default</option>
        <option value="1h">1 hour</option>
        <option value="1d">1 day</option>
        <option value="7d">7 days</option>
        <option value="30d">30 days</option>
        <option value="keep">Keep (no expiry)</option>
      </select>
    </label>`;
}

function noteRow(): string {
  return `<label class="fld">Note <span class="hint">shown to viewers, optional</span>
    <input id="note" type="text" maxlength="500" placeholder="e.g. Q3 report — sign by Friday" />
  </label>`;
}

function passwordRow(): string {
  return `<label class="fld">Password <span class="hint">optional</span>
    <input id="password" type="password" placeholder="leave blank for none" autocomplete="off" />
  </label>`;
}

function checkRow(id: string, label: string): string {
  return `<label class="setting-row"><input type="checkbox" id="${id}" /><span class="setting-label">${label}</span></label>`;
}

function settingsBlock(): string {
  const s = state.status!;
  const shellChk = s.shellInstalled ? 'checked' : '';
  const autoChk = s.autostartEnabled ? 'checked' : '';
  const autoDis = s.canReceive ? '' : 'disabled';
  return `<details class="settings"${state.settingsOpen ? ' open' : ''}>
    <summary class="settings-summary">Settings</summary>
    <div class="settings-body">
      <label class="setting-row"><input type="checkbox" id="set-shell" ${shellChk} /><span class="setting-label">Right-click Share menu</span></label>
      <label class="setting-row${s.canReceive ? '' : ' is-disabled'}"><input type="checkbox" id="set-autostart" ${autoChk} ${autoDis} /><span class="setting-label">Auto-receive files at login${
        s.canReceive ? '' : '<span class="setting-help">Sign in to enable this.</span>'
      }</span></label>
      ${s.loggedIn ? `<button class="btn-mini" id="open-trust">Manage who can send to my devices →</button>` : ''}
    </div>
  </details>`;
}

function renderTrust() {
  const s = state.status!;
  root.innerHTML = `
    <div class="modal">
      <header class="modal-head">
        <div class="brand"><button class="btn-mini" id="trust-back">←</button> Device access</div>
        <div class="who">${escapeHtml(s.email)}</div>
      </header>
      <div class="hint">Choose which of your devices a contact may send files to. Applies when your inbound mode is "approvals".</div>
      <div class="fld">Contact email
        <div style="display:flex; gap:6px;">
          <input id="trust-email" type="text" placeholder="alice@example.com" value="${escapeHtml(state.trustEmail)}" style="flex:1;" />
          <button class="btn-mini" id="trust-load">Load</button>
        </div>
      </div>
      ${state.trustError ? `<div class="banner-err">${escapeHtml(state.trustError)}</div>` : ''}
      <section class="panel" id="trust-panel">${trustDeviceList()}</section>
    </div>`;
  wireTrust();
}

function trustDeviceList(): string {
  if (state.trustDevices === null) {
    return `<div class="soon-note">Enter a contact's email and press Load.</div>`;
  }
  if (!state.trustDevices.length) {
    return `<div class="soon-note">You have no devices with a key yet.</div>`;
  }
  return state.trustDevices
    .map((d) => {
      const dis = d.hasKey ? '' : 'disabled';
      const cur = d.current ? ' (this device)' : '';
      const noKey = d.hasKey ? '' : '<span class="setting-help">No key yet.</span>';
      return `<label class="setting-row${d.hasKey ? '' : ' is-disabled'}"><input type="checkbox" class="trust-dev" data-id="${d.sessionId}" ${
        d.exposed ? 'checked' : ''
      } ${dis} /><span class="setting-label">${escapeHtml(d.label)}${cur}${noKey}</span></label>`;
    })
    .join('');
}

function canShare(): boolean {
  return !!state.status?.loggedIn && state.paths.length > 0 && state.target !== 'network';
}

// shareDisabledReason explains, in the footer, why Share is disabled so the
// control isn't a dead end. Order mirrors canShare()'s checks.
function shareDisabledReason(): string {
  if (!state.status?.loggedIn) return 'Sign in to share.';
  if (state.paths.length === 0) return 'Add a file — drop or paste one above — to share.';
  if (state.target === 'network') return 'Network sharing is coming soon.';
  return '';
}

function wire() {
  root.querySelectorAll<HTMLButtonElement>('.tab').forEach((b) => {
    b.addEventListener('click', async () => {
      const t = b.dataset.tab as Target;
      state.target = t;
      render();
      if (t === 'device' && !state.devices.length && !state.status?.isApiToken) {
        try {
          state.devices = (await backend().ListDevices()) || [];
        } catch (e) {
          state.devices = [];
        }
        if (state.target === 'device') render();
      }
    });
  });
  const dsel = root.querySelector<HTMLSelectElement>('#device-select');
  if (dsel) {
    state.selectedDevice = dsel.value;
    dsel.addEventListener('change', () => (state.selectedDevice = dsel.value));
  }
  root.querySelector<HTMLButtonElement>('#share-btn')?.addEventListener('click', doShare);
  root.querySelector<HTMLButtonElement>('#sign-in')?.addEventListener('click', signIn);
  root.querySelector<HTMLButtonElement>('#reopen-login')?.addEventListener('click', () => backend().BeginLogin());
  root.querySelector<HTMLButtonElement>('#theme-toggle')?.addEventListener('click', toggleTheme);
  root.querySelector<HTMLButtonElement>('#logout-btn')?.addEventListener('click', logout);
  root.querySelector<HTMLButtonElement>('#apply-update')?.addEventListener('click', async (e) => {
    const btn = e.currentTarget as HTMLButtonElement;
    btn.disabled = true;
    btn.textContent = 'Updating…';
    try {
      await backend().ApplyUpdate();
    } catch {
      btn.disabled = false;
      btn.textContent = 'Install';
    }
  });
  root.querySelectorAll<HTMLButtonElement>('.chip-x').forEach((b) =>
    b.addEventListener('click', () => {
      state.paths.splice(Number(b.dataset.i), 1);
      render();
    })
  );
  root.querySelector<HTMLDetailsElement>('.settings')?.addEventListener('toggle', (e) => {
    state.settingsOpen = (e.target as HTMLDetailsElement).open;
  });
  wireToggle('set-shell', (on) => backend().SetShellIntegration(on));
  wireToggle('set-autostart', (on) => backend().SetAutostart(on));
  root.querySelector<HTMLButtonElement>('#open-trust')?.addEventListener('click', () => {
    state.view = 'trust';
    render();
  });
}

function wireTrust() {
  root.querySelector<HTMLButtonElement>('#trust-back')?.addEventListener('click', () => {
    state.view = 'share';
    state.settingsOpen = true;
    render();
  });
  const emailEl = root.querySelector<HTMLInputElement>('#trust-email');
  const load = async () => {
    const email = emailEl?.value.trim() || '';
    if (!email) return;
    state.trustEmail = email;
    state.trustError = '';
    try {
      state.trustDevices = (await backend().DeviceAccessForContact(email)) || [];
    } catch (e) {
      state.trustDevices = null;
      state.trustError = String(e);
    }
    render();
  };
  root.querySelector<HTMLButtonElement>('#trust-load')?.addEventListener('click', load);
  emailEl?.addEventListener('keydown', (e) => {
    if ((e as KeyboardEvent).key === 'Enter') load();
  });
  root.querySelectorAll<HTMLInputElement>('.trust-dev').forEach((el) =>
    el.addEventListener('change', async () => {
      try {
        await backend().SetDeviceAccess(state.trustEmail, el.dataset.id || '', el.checked);
      } catch (e) {
        el.checked = !el.checked;
        state.trustError = String(e);
        render();
      }
    })
  );
}

// wireToggle calls the backend on change and, on failure, reverts the checkbox
// and refreshes status so the UI never lies about the actual state.
function wireToggle(id: string, fn: (on: boolean) => Promise<void>) {
  const el = root.querySelector<HTMLInputElement>('#' + id);
  el?.addEventListener('change', async () => {
    try {
      await fn(el.checked);
    } catch (e) {
      el.checked = !el.checked;
    }
    try {
      state.status = await backend().Status();
    } catch {}
  });
}

async function signIn() {
  state.loginPhase = 'waiting';
  state.loginError = '';
  try {
    state.loginInfo = await backend().BeginLogin();
    render();
    const status = await backend().CompleteLogin();
    state.status = status;
    state.devices = [];
    state.loginPhase = 'idle';
    state.loginInfo = null;
    render();
  } catch (e) {
    state.loginPhase = 'error';
    state.loginError = String(e);
    state.loginInfo = null;
    render();
  }
}

async function doShare() {
  const btn = root.querySelector<HTMLButtonElement>('#share-btn')!;
  btn.disabled = true;
  btn.textContent = 'Sharing…';
  try {
    const req = buildRequest();
    const outcomes = await backend().Share(req);
    renderResults(outcomes);
  } catch (e) {
    renderResults([{ path: '', ok: false, error: String(e) }]);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Share';
  }
}

function buildRequest(): ShareRequest {
  const req: ShareRequest = { paths: state.paths, target: state.target };
  const val = (id: string) => root.querySelector<HTMLInputElement>('#' + id)?.value?.trim() || '';
  const checked = (id: string) => !!root.querySelector<HTMLInputElement>('#' + id)?.checked;
  const expires = root.querySelector<HTMLSelectElement>('#expires')?.value || '';
  if (expires === 'keep') req.keep = true;
  else if (expires) req.expires = expires;
  req.password = val('password') || undefined;
  req.oneTime = checked('one-time');
  req.note = val('note') || undefined;
  if (state.target === 'private') {
    req.recipients = val('recipients')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    req.allowReshare = checked('allow-reshare');
  }
  if (state.target === 'device') {
    const sel = root.querySelector<HTMLSelectElement>('#device-select');
    req.deviceId = sel?.value || '';
    const opt = sel?.selectedOptions[0];
    req.devicePub = opt?.getAttribute('data-pub') || '';
  }
  return req;
}

function renderResults(outcomes: ShareOutcome[]) {
  const box = root.querySelector<HTMLDivElement>('#results')!;
  box.innerHTML = outcomes
    .map((o) => {
      const name = o.path ? escapeHtml(basename(o.path)) : '';
      if (!o.ok) {
        return `<div class="res err"><span class="res-name">${name}</span><span class="res-msg">${escapeHtml(
          o.error || 'failed'
        )}</span></div>`;
      }
      if (o.link) {
        return `<div class="res ok">
          <span class="res-name">${name}</span>
          <input class="link" readonly value="${escapeHtml(o.link)}" />
          <button class="btn-copy" data-link="${escapeHtml(o.link)}">Copy</button>
        </div>`;
      }
      return `<div class="res ok"><span class="res-name">${name}</span><span class="res-msg">Sent ✓</span></div>`;
    })
    .join('');
  box.querySelectorAll<HTMLButtonElement>('.btn-copy').forEach((b) =>
    b.addEventListener('click', () => {
      copy(b.dataset.link || '');
      b.textContent = 'Copied';
      setTimeout(() => (b.textContent = 'Copy'), 1200);
    })
  );
}

function copy(text: string) {
  const rt = (window as any).runtime;
  if (rt?.ClipboardSetText) rt.ClipboardSetText(text);
  else navigator.clipboard?.writeText(text).catch(() => {});
}

function basename(p: string): string {
  const parts = p.split(/[\\/]/);
  return parts[parts.length - 1] || p;
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] as string)
  );
}

// --- Paste / drop to share ---------------------------------------------------

function addPaths(paths: string[]) {
  let changed = false;
  for (const p of paths) {
    if (p && !state.paths.includes(p)) {
      state.paths.push(p);
      changed = true;
    }
  }
  if (changed) {
    if (state.view !== 'share') state.view = 'share';
    render();
  }
}

let listenersReady = false;
function setupInputListeners() {
  if (listenersReady) return;
  listenersReady = true;
  document.addEventListener('paste', onPaste);
  (window as any).runtime?.EventsOn?.('files-dropped', (paths: string[]) => addPaths(paths || []));
}

async function onPaste(e: ClipboardEvent) {
  if (!state.status?.loggedIn || state.view !== 'share') return;
  // Don't hijack pastes into text fields (email, password, recipients).
  if ((e.target as HTMLElement)?.closest?.('input, textarea')) return;
  const cd = e.clipboardData;
  if (!cd) return;
  for (const item of Array.from(cd.items)) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const blob = item.getAsFile();
      if (blob) {
        e.preventDefault();
        try {
          addPaths([await backend().AddPasted(extFromMime(item.type), await blobToBase64(blob))]);
        } catch {}
        return;
      }
    }
  }
  const text = cd.getData('text');
  if (text && text.trim()) {
    e.preventDefault();
    try {
      addPaths([await backend().AddPasted(looksLikeMarkdown(text) ? 'md' : 'txt', utf8ToBase64(text))]);
    } catch {}
  }
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => {
      const s = String(r.result);
      resolve(s.slice(s.indexOf(',') + 1));
    };
    r.onerror = () => reject(new Error('read failed'));
    r.readAsDataURL(blob);
  });
}

function utf8ToBase64(s: string): string {
  return btoa(unescape(encodeURIComponent(s)));
}

function extFromMime(mime: string): string {
  return (
    { 'image/png': 'png', 'image/jpeg': 'jpg', 'image/gif': 'gif', 'image/webp': 'webp', 'image/bmp': 'bmp' } as Record<
      string,
      string
    >
  )[mime] || 'png';
}

function looksLikeMarkdown(t: string): boolean {
  return /(^|\n)\s{0,3}(#{1,6}\s|[-*+]\s|\d+\.\s|>\s|```)/.test(t) || /\[[^\]]+\]\([^)]+\)/.test(t);
}

boot();
