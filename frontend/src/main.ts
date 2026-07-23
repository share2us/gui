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
type LanListen = { address: string; port: number; passphrase: string; pairing: string; destDir: string };
type ClipSuggestion = { kind: 'image' | 'text' | 'none'; preview: string; ext: string };

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
  LanSend(paths: string[], dest: string, password: string): Promise<ShareOutcome[]>;
  LanStartReceive(): Promise<LanListen>;
  LanStopReceive(): Promise<void>;
  ClipboardSuggestion(): Promise<ClipSuggestion>;
  AddClipboard(kind: string): Promise<string>;
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
  optionsOpen: false as boolean,
  // Local-network (LAN / guest) sharing.
  netMode: 'send' as 'send' | 'receive',
  netDest: '' as string,
  netListen: null as LanListen | null,
  netReceiving: false as boolean,
  netStatus: '' as string,
  // Clipboard suggestion (Windows backend read).
  clip: null as ClipSuggestion | null,
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
    checkClipboard();
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
  // Guests can only use Local network sharing; force that destination.
  if (!s.loggedIn && state.target !== 'network') state.target = 'network';
  root.innerHTML = `
    <div class="modal">
      <header class="modal-head">
        <div class="brand">${BRAND_SVG}<span>Share2Us</span></div>
        <div class="head-actions">
          <button class="icon-btn" id="theme-toggle" title="Toggle light/dark">${state.theme === 'dark' ? '☀' : '☾'}</button>
          <button class="icon-btn" id="check-update" title="Check for updates">⟳</button>
          ${
            s.loggedIn
              ? `<span class="who" title="${escapeHtml(s.email)}">${escapeHtml(s.email)}</span><button class="btn-hdr" id="logout-btn">Logout</button>`
              : `<button class="btn-hdr" id="login-btn">Login</button>`
          }
        </div>
      </header>
      ${updateBanner()}
      ${loginProgress()}
      <div class="modal-body">
        ${filesBlock()}
        ${
          s.loggedIn
            ? `<nav class="tabs">
          ${tab('public', 'Public link')}
          ${tab('private', 'Private (email)')}
          ${tab('device', 'Send to device')}
          ${tab('network', 'Local network')}
        </nav>`
            : `<div class="section-label">Local network share <span class="pill">no account needed</span></div>`
        }
        <section class="panel" id="panel">${panel()}</section>
        <div class="results" id="results"></div>
        ${settingsBlock()}
      </div>
      <footer class="modal-foot">
        ${canPrimary() ? '' : `<div class="foot-reason">${escapeHtml(primaryDisabledReason())}</div>`}
        <button class="btn-primary" id="primary-btn" ${
          canPrimary() ? '' : `disabled title="${escapeHtml(primaryDisabledReason())}"`
        }>${escapeHtml(primaryLabel())}</button>
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

// loginProgress renders only the transient states of the header Login flow
// (waiting for browser approval, or an error). The idle "sign in to share"
// prompt was removed — logging in is initiated from the header Login button.
function loginProgress(): string {
  if (state.loginPhase === 'waiting' && state.loginInfo) {
    const info = state.loginInfo;
    return `<div class="banner">
      Approve this device in your browser${info.userCode ? ` — code <code>${escapeHtml(info.userCode)}</code>` : ''}.
      <span class="hint">Waiting for approval…</span>
      ${info.verificationUrl ? `<button class="btn-mini" id="reopen-login">Reopen page</button>` : ''}
    </div>`;
  }
  if (state.loginPhase === 'error' && state.loginError) {
    return `<div class="banner-err">${escapeHtml(state.loginError)}</div>`;
  }
  return '';
}

function filesBlock(): string {
  if (!state.paths.length) {
    return `<div class="canvas" id="canvas">
      <div class="canvas-ico">⬍</div>
      <div class="canvas-title">Drop a file here, or paste with Ctrl+V</div>
      <div class="canvas-sub">Screenshots, images, and text work too — or right-click a file in your file manager → s2u → Share.</div>
      ${clipChip()}
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
      return optionsCard('public');
    case 'private':
      return `
        <label class="fld">Recipient emails <span class="hint">comma-separated</span>
          <input id="recipients" type="text" placeholder="alice@example.com, bob@example.com" />
        </label>
        ${optionsCard('private')}`;
    case 'device':
      return devicePanel();
    case 'network':
      return networkPanel();
  }
}

// optionsCard hides the optional share settings (note, expiry, one-time /
// reshare, password) behind a card the user opens only when they need them, so
// the default flow is just: pick files, pick destination, Share.
function optionsCard(target: Target): string {
  const extra =
    target === 'public'
      ? checkRow('one-time', 'One-time (delete after first download)')
      : checkRow('allow-reshare', 'Allow recipients to reshare');
  return `<details class="opt-card"${state.optionsOpen ? ' open' : ''}>
    <summary class="opt-summary">＋ Options<span class="opt-hint">note, expiry, password</span></summary>
    <div class="opt-body">
      ${noteRow()}
      ${expiryRow()}
      ${extra}
      ${passwordRow()}
    </div>
  </details>`;
}

// networkPanel is the account-free LAN transfer UI: a Send / Receive segmented
// control over cli-core lanshare.
function networkPanel(): string {
  const seg = `<div class="seg">
    <button class="seg-btn ${state.netMode === 'send' ? 'active' : ''}" data-net="send">Send</button>
    <button class="seg-btn ${state.netMode === 'receive' ? 'active' : ''}" data-net="receive">Receive</button>
  </div>`;
  return seg + (state.netMode === 'receive' ? lanReceivePanel() : lanSendPanel());
}

function lanSendPanel(): string {
  return `
    <label class="fld">Receiver code or address <span class="hint">paste their s2u:// code, or the IP they show</span>
      <input id="net-dest" type="text" placeholder="s2u://…  or  192.168.1.5" value="${escapeHtml(state.netDest)}" />
    </label>
    <label class="fld">Password <span class="hint">only if their code has none</span>
      <input id="net-pass" type="password" autocomplete="off" placeholder="leave blank if the code includes it" />
    </label>
    <div class="hint">Sent straight to the other device, end-to-end encrypted. No account needed.</div>`;
}

function lanReceivePanel(): string {
  if (!state.netReceiving || !state.netListen) {
    return `<div class="soon-note">Receive a file from a nearby device with no account — it lands in your Downloads.</div>
      <div class="hint">Press <b>Start receiving</b> below, then share the code it shows with the sender.</div>`;
  }
  const l = state.netListen;
  return `
    <div class="hint">Give the sender this code (or your address + passphrase). Incoming files land in your Downloads.</div>
    <label class="fld">Your code <span class="hint">they paste this into "Receiver code"</span>
      <div class="copy-row">
        <input class="link" readonly value="${escapeHtml(l.pairing)}" />
        <button class="btn-copy" data-copy="${escapeHtml(l.pairing)}">Copy</button>
      </div>
    </label>
    <div class="lan-kv"><span>Address</span><b class="mono">${escapeHtml(l.address)}</b></div>
    <div class="lan-kv"><span>Passphrase</span><b class="mono">${escapeHtml(l.passphrase)}</b></div>
    <div class="lan-status" id="lan-status">${escapeHtml(state.netStatus || 'Waiting for a sender…')}</div>`;
}

// clipChip offers a one-click share of whatever is on the clipboard.
function clipChip(): string {
  const c = state.clip;
  if (!c || c.kind === 'none') return '';
  const label = c.kind === 'image' ? '🖼 Share clipboard image' : '📄 Share copied text';
  const prev = c.kind === 'text' && c.preview ? `<div class="clip-prev">${escapeHtml(c.preview)}</div>` : '';
  return `<div class="clip-suggest"><button class="clip-chip" id="clip-add">${label}</button>${prev}</div>`;
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

// shareDisabledReason explains why a cloud Share is disabled (guests don't see
// the cloud tabs, so this is a safety net).
function shareDisabledReason(): string {
  if (!state.status?.loggedIn) return 'Login to share to the cloud.';
  if (state.paths.length === 0) return 'Add a file — drop or paste one above — to share.';
  return '';
}

// The footer primary button is context-aware: Share (cloud), Send (LAN send), or
// Start/Stop receiving (LAN receive).
function primaryLabel(): string {
  if (state.target === 'network') {
    if (state.netMode === 'receive') return state.netReceiving ? 'Stop receiving' : 'Start receiving';
    return 'Send';
  }
  return 'Share';
}
function canPrimary(): boolean {
  if (state.target === 'network') {
    if (state.netMode === 'receive') return true; // start / stop is always actionable
    return state.paths.length > 0; // send needs files (address is checked on click)
  }
  return canShare();
}
function primaryDisabledReason(): string {
  if (state.target === 'network') {
    if (state.netMode === 'send' && state.paths.length === 0) return 'Add a file — drop or paste one above — to send.';
    return '';
  }
  return shareDisabledReason();
}

// onPrimary dispatches the footer button to the right action for the context.
async function onPrimary() {
  if (state.target === 'network') {
    if (state.netMode === 'receive') return state.netReceiving ? stopReceive() : startReceive();
    return lanSend();
  }
  return doShare();
}

async function lanSend() {
  const dest = (root.querySelector<HTMLInputElement>('#net-dest')?.value || '').trim();
  state.netDest = dest;
  const pass = root.querySelector<HTMLInputElement>('#net-pass')?.value || '';
  if (!dest) {
    root.querySelector<HTMLInputElement>('#net-dest')?.focus();
    renderResults([{ path: '', ok: false, error: 'Enter the receiver’s code or address first.' }]);
    return;
  }
  const btn = root.querySelector<HTMLButtonElement>('#primary-btn')!;
  btn.disabled = true;
  btn.textContent = 'Sending…';
  try {
    const outcomes = await backend().LanSend(state.paths, dest, pass);
    renderResults(outcomes);
  } catch (e) {
    renderResults([{ path: '', ok: false, error: String(e) }]);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Send';
  }
}

async function startReceive() {
  const btn = root.querySelector<HTMLButtonElement>('#primary-btn')!;
  btn.disabled = true;
  btn.textContent = 'Starting…';
  try {
    const l = await backend().LanStartReceive();
    state.netListen = l;
    state.netReceiving = true;
    state.netStatus = 'Waiting for a sender…';
    render();
  } catch (e) {
    state.netReceiving = false;
    state.netListen = null;
    render();
    renderResults([{ path: '', ok: false, error: String(e) }]);
  }
}

async function stopReceive() {
  try {
    await backend().LanStopReceive();
  } catch {
    /* ignore */
  }
  state.netReceiving = false;
  state.netListen = null;
  state.netStatus = '';
  render();
}

function fmtBytes(n?: number): string {
  if (!n || n <= 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
}

async function checkClipboard() {
  try {
    const c = await backend().ClipboardSuggestion();
    const changed = JSON.stringify(c) !== JSON.stringify(state.clip);
    state.clip = c && c.kind !== 'none' ? c : null;
    if (changed && !state.paths.length && state.view === 'share') render();
  } catch {
    /* clipboard unavailable (non-Windows) — the Ctrl+V path still works */
  }
}

async function addClipboard() {
  const c = state.clip;
  if (!c || c.kind === 'none') return;
  try {
    addPaths([await backend().AddClipboard(c.kind)]);
  } catch (e) {
    renderResults([{ path: '', ok: false, error: String(e) }]);
  }
}

// manualUpdateCheck runs an on-demand update check from the header button and
// either reveals the update banner or briefly confirms the app is up to date.
async function manualUpdateCheck() {
  const btn = root.querySelector<HTMLButtonElement>('#check-update');
  if (!btn) return;
  btn.disabled = true;
  const prev = btn.textContent;
  btn.textContent = '…';
  try {
    const info = await backend().CheckUpdate();
    if (info?.available) {
      state.update = info;
      render();
      return;
    }
    btn.textContent = '✓';
    btn.title = 'Up to date';
    setTimeout(() => {
      btn.textContent = prev;
      btn.title = 'Check for updates';
      btn.disabled = false;
    }, 1500);
  } catch {
    btn.textContent = prev;
    btn.disabled = false;
  }
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
  root.querySelector<HTMLButtonElement>('#primary-btn')?.addEventListener('click', onPrimary);
  root.querySelector<HTMLButtonElement>('#login-btn')?.addEventListener('click', signIn);
  root.querySelector<HTMLButtonElement>('#reopen-login')?.addEventListener('click', () => backend().BeginLogin());
  root.querySelector<HTMLButtonElement>('#theme-toggle')?.addEventListener('click', toggleTheme);
  root.querySelector<HTMLButtonElement>('#check-update')?.addEventListener('click', manualUpdateCheck);
  root.querySelector<HTMLButtonElement>('#logout-btn')?.addEventListener('click', logout);
  root.querySelector<HTMLButtonElement>('#clip-add')?.addEventListener('click', addClipboard);
  // LAN Send/Receive segmented control.
  root.querySelectorAll<HTMLButtonElement>('.seg-btn').forEach((b) =>
    b.addEventListener('click', () => {
      const m = b.dataset.net as 'send' | 'receive';
      if (m && m !== state.netMode) {
        state.netMode = m;
        render();
      }
    })
  );
  // Preserve the LAN destination across re-renders.
  const nd = root.querySelector<HTMLInputElement>('#net-dest');
  nd?.addEventListener('input', () => (state.netDest = nd.value));
  // Copy buttons that carry their text inline (e.g. the receiver code).
  root.querySelectorAll<HTMLButtonElement>('.btn-copy[data-copy]').forEach((b) =>
    b.addEventListener('click', () => {
      copy(b.dataset.copy || '');
      const t = b.textContent;
      b.textContent = 'Copied';
      setTimeout(() => (b.textContent = t), 1200);
    })
  );
  // Advanced options card open/close.
  root.querySelector<HTMLDetailsElement>('.opt-card')?.addEventListener('toggle', (e) => {
    state.optionsOpen = (e.target as HTMLDetailsElement).open;
  });
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
  const btn = root.querySelector<HTMLButtonElement>('#primary-btn')!;
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
  const rt = (window as any).runtime;
  rt?.EventsOn?.('files-dropped', (paths: string[]) => addPaths(paths || []));
  // LAN receive progress + completion.
  rt?.EventsOn?.('lan-recv-progress', (p: any) => {
    state.netStatus =
      p?.total > 0
        ? `Receiving… ${fmtBytes(p.received)} / ${fmtBytes(p.total)}`
        : `Receiving… ${fmtBytes(p?.received)}`;
    const el = root.querySelector('#lan-status');
    if (el) el.textContent = state.netStatus;
  });
  rt?.EventsOn?.('lan-recv-done', (d: any) => {
    state.netReceiving = false;
    state.netListen = null;
    state.netStatus = '';
    render();
    if (d?.error) {
      renderResults([{ path: '', ok: false, error: String(d.error) }]);
    } else {
      const box = root.querySelector<HTMLDivElement>('#results');
      if (box) {
        box.innerHTML = `<div class="res ok"><span class="res-name">${escapeHtml(
          String(d?.name || 'file')
        )}</span><span class="res-msg">Received → Downloads</span></div>`;
      }
    }
  });
  // LAN send progress -> primary button label.
  rt?.EventsOn?.('lan-send-progress', (p: any) => {
    if (state.target !== 'network' || state.netMode !== 'send') return;
    const pct = p?.total > 0 ? Math.floor((p.sent / p.total) * 100) : 0;
    const btn = root.querySelector<HTMLButtonElement>('#primary-btn');
    if (btn) btn.textContent = `Sending… ${pct}%`;
  });
  // Re-check the clipboard when the window regains focus.
  window.addEventListener('focus', () => {
    checkClipboard();
  });
}

async function onPaste(e: ClipboardEvent) {
  if (state.view !== 'share') return;
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
