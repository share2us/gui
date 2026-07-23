import './style.css';

// Backend is window.go.main.App (Wails). Types mirror app.go.
type Status = {
  loggedIn: boolean;
  email: string;
  isApiToken: boolean;
  canReceive: boolean;
  shellInstalled: boolean;
  autostartEnabled: boolean;
  discoverable: boolean;
};
type ShareRequest = {
  paths: string[];
  target: string;
  recipients?: string[];
  password?: string;
  oneTime?: boolean;
  expires?: string;
  keep?: boolean;
  allowReshare?: boolean;
  note?: string;
};
type ShareOutcome = { path: string; ok: boolean; link?: string; error?: string };
type LoginInfo = { userCode: string; verificationUrl: string; verificationUri: string };
type UpdateInfo = { available: boolean; current: string; latest: string; assetUrl: string; assetName: string; page: string };
type LanPeer = {
  name: string; addr: string; dest: string; code: string; mode: string;
  fingerprint: string; isBroadcast: boolean; fileName: string; fileSize: number;
};
type LanRequest = { id: string; from: string; name: string; size: number; fingerprint: string; senderName: string; code: string; action: string };
type TrustedDevice = { fingerprint: string; name: string };
type ClipSuggestion = { kind: 'image' | 'text' | 'none'; preview: string; ext: string };
type Activity = { kind: string; peer: string; name: string; size: number; ts: number };
type BcConn = { fingerprint: string; name: string; peer: string; sent: number; total: number; done: boolean; err: string };
type BroadcastState = { active: boolean; name: string; size: number; access: string; downloading: BcConn[]; completed: BcConn[] };
type DownloadResult = { name: string; fingerprint: string; from: string; trusted: boolean };

interface AppBackend {
  Status(): Promise<Status>;
  PendingPaths(): Promise<string[]>;
  Share(req: ShareRequest): Promise<ShareOutcome[]>;
  BeginLogin(): Promise<LoginInfo>;
  CompleteLogin(): Promise<Status>;
  SetAutostart(on: boolean): Promise<void>;
  SetShellIntegration(on: boolean): Promise<void>;
  Logout(): Promise<void>;
  AddPasted(ext: string, dataB64: string): Promise<string>;
  ClipboardSuggestion(): Promise<ClipSuggestion>;
  AddClipboard(kind: string): Promise<string>;
  CheckUpdate(): Promise<UpdateInfo>;
  ApplyUpdate(): Promise<void>;
  LanSend(paths: string[], dest: string, password: string): Promise<ShareOutcome[]>;
  LanBrowse(): Promise<LanPeer[]>;
  SetDiscoverable(on: boolean): Promise<void>;
  RespondLanRequest(id: string, accept: boolean): Promise<void>;
  TrustDevice(fingerprint: string, name: string): Promise<void>;
  UntrustDevice(fingerprint: string): Promise<void>;
  ListTrusted(): Promise<TrustedDevice[]>;
  StartBroadcast(path: string, access: string): Promise<BroadcastState>;
  StopBroadcast(): Promise<void>;
  BroadcastStats(): Promise<BroadcastState>;
  LanDownload(addr: string, fingerprint: string, name: string, size: number): Promise<DownloadResult>;
  ActivityLog(): Promise<Activity[]>;
  ClearActivity(): Promise<void>;
  GetScanInterval(): Promise<number>;
  SetScanInterval(sec: number): Promise<void>;
}
const backend = (): AppBackend => (window as any).go.main.App;

type View = 'home' | 'share' | 'broadcast';
type Dest = 'nearby' | 'broadcast' | 'public' | 'private';

const state = {
  view: 'home' as View,
  paths: [] as string[],
  status: null as Status | null,
  peers: [] as LanPeer[],
  browsing: false as boolean,
  requests: [] as LanRequest[],
  activity: [] as Activity[],
  trusted: [] as TrustedDevice[],
  bc: null as BroadcastState | null, // live broadcast
  discCode: '' as string,
  clip: null as ClipSuggestion | null,
  theme: 'dark' as 'dark' | 'light',
  update: null as UpdateInfo | null,
  scanInterval: 60 as number,
  // share modal
  dest: 'nearby' as Dest,
  bcAccess: 'approve' as 'all' | 'trusted' | 'approve',
  optionsOpen: false as boolean,
  netDest: '' as string,
  // login
  loginPhase: 'idle' as 'idle' | 'waiting' | 'error',
  loginInfo: null as LoginInfo | null,
  loginError: '' as string,
  // download confirm overlay
  dl: null as (LanPeer & { trust?: boolean }) | null,
};

const BRAND_SVG =
  `<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#2fd6b6"/><stop offset="1" stop-color="#17a68c"/></linearGradient></defs><rect width="24" height="24" rx="6" fill="url(#g)"/><text x="12" y="16.5" font-family="system-ui,sans-serif" font-size="10" font-weight="700" text-anchor="middle" fill="#0b2b26">S2U</text></svg>`;

const root = document.querySelector<HTMLDivElement>('#app')!;

function applyTheme() { document.documentElement.setAttribute('data-theme', state.theme); }
function initTheme() { state.theme = localStorage.getItem('s2u-theme') === 'light' ? 'light' : 'dark'; applyTheme(); }
function toggleTheme() { state.theme = state.theme === 'dark' ? 'light' : 'dark'; localStorage.setItem('s2u-theme', state.theme); applyTheme(); render(); }

async function boot() {
  initTheme();
  root.innerHTML = `<div class="loading">Loading…</div>`;
  try {
    const [status, paths] = await Promise.all([backend().Status(), backend().PendingPaths()]);
    state.status = status;
    state.paths = paths || [];
    state.scanInterval = await backend().GetScanInterval().catch(() => 60);
    state.bc = await backend().BroadcastStats().catch(() => null);
    if (state.bc && !state.bc.active) state.bc = null;
    // Opened via the Share verb with files -> jump straight to the Share modal.
    if (state.paths.length) state.view = 'share';
    render();
    setupListeners();
    refreshActivity();
    loadTrusted();
    checkForUpdate();
    checkClipboard();
    findNearby(); // populate nearby devices/broadcasts on open
    startScanTimer();
  } catch (e) {
    root.innerHTML = `<div class="error-box">Could not start: ${escapeHtml(String(e))}</div>`;
  }
}

function render(): void {
  if (state.view === 'share') return renderShare();
  if (state.view === 'broadcast') return renderBroadcast();
  return renderHome();
}

// ---- Header ----------------------------------------------------------------

function header(): string {
  const s = state.status!;
  return `<header class="modal-head">
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
  </header>`;
}

function discBanner(): string {
  if (!state.status?.discoverable) return '';
  const code = state.discCode ? `<b>Verify code ${escapeHtml(state.discCode)}</b>` : 'starting…';
  return `<div class="disc-bar">📡 Discoverable — nearby devices can send you files. ${code}</div>`;
}

// ---- Home ------------------------------------------------------------------

function renderHome(): void {
  root.innerHTML = `<div class="modal">
    ${header()}
    ${updateBanner()}
    ${discBanner()}
    ${loginProgress()}
    <div class="home">
      <button class="share-cta" id="open-share"><span class="plus">+</span> Share a file</button>
      <div class="sec-head"><b>Activity &amp; nearby</b><button class="refresh" id="nearby-find" title="Refresh (auto every ${state.scanInterval || 60}s)">↻</button></div>
      <div class="feed-scroll">${feed()}</div>
      ${settingsBlock()}
    </div>
    ${state.requests.length ? requestOverlay(state.requests[0]) : ''}
    ${state.dl ? downloadOverlay(state.dl) : ''}
  </div>`;
  wire();
}

function feed(): string {
  const rows: string[] = [];
  // Incoming approval prompts that aren't shown as an overlay yet still queue;
  // the head is the overlay. Broadcasts + your live broadcast + log below.
  for (const p of state.peers.filter((x) => x.isBroadcast)) {
    rows.push(bcastRow(p));
  }
  if (state.bc && state.bc.active) rows.push(liveRow(state.bc));
  for (const p of state.peers.filter((x) => !x.isBroadcast)) {
    rows.push(nearbyRow(p));
  }
  for (const a of state.activity) rows.push(logRow(a));
  if (!rows.length) {
    return `<div class="empty">Nothing yet. Press ↻ to scan for nearby devices, or + Share a file.</div>`;
  }
  return rows.join('');
}

function bcastRow(p: LanPeer): string {
  return `<div class="item warn">
    <div class="ico bc">📡</div>
    <div class="line"><b>${escapeHtml(p.name)}</b> · ${escapeHtml(p.fileName)} <span class="meta">${fmtBytes(p.fileSize)}</span></div>
    <div class="acts"><span class="warn-ic" title="Unverified until you download">⚠</span><button class="ib on dl-btn" data-fp="${escapeHtml(p.fingerprint)}" title="Download">↓</button></div>
  </div>`;
}

function nearbyRow(p: LanPeer): string {
  return `<div class="item">
    <div class="ico rx">📡</div>
    <div class="line"><b>${escapeHtml(p.name)}</b> <span class="meta">${escapeHtml(p.addr)}</span>${
      p.code ? ` <span class="tag code">${escapeHtml(p.code)}</span>` : ''
    }</div>
    <div class="acts"><button class="ib on send-to" data-dest="${escapeHtml(p.dest)}" title="Send to this device">→</button></div>
  </div>`;
}

function liveRow(bc: BroadcastState): string {
  const now = bc.downloading.length, done = bc.completed.length;
  return `<div class="item" id="live-row">
    <div class="ico out">📡</div>
    <div class="line"><b>Broadcasting</b> ${escapeHtml(bc.name)} <span class="meta">· ${now} now · ${done} done</span> <span class="tag live">live</span></div>
    <div class="acts"><button class="ib" id="bc-stop" title="Stop broadcasting">◼</button></div>
  </div>`;
}

function logRow(a: Activity): string {
  const verb: Record<string, string> = { sent: 'Sent', received: 'Received', downloaded: 'Downloaded', broadcast: 'Broadcast to' };
  const who = a.peer ? ` ${a.kind === 'sent' ? 'to' : a.kind === 'broadcast' ? '' : 'from'} ${escapeHtml(a.peer)}` : '';
  return `<div class="item log">
    <div class="ico done">✓</div>
    <div class="line">${verb[a.kind] || a.kind} <b>${escapeHtml(a.name)}</b>${who} <span class="meta">· ${ago(a.ts)}</span></div>
  </div>`;
}

// ---- Share modal -----------------------------------------------------------

function renderShare(): void {
  const s = state.status!;
  root.innerHTML = `<div class="modal">
    <header class="modal-head">
      <div class="brand"><button class="btn-mini" id="share-back">←</button> Share</div>
      <div class="head-actions"><span class="who" style="color:var(--muted)">${state.paths.length} file${state.paths.length === 1 ? '' : 's'}</span></div>
    </header>
    ${loginProgress()}
    <div class="modal-body" style="padding:14px 18px 24px">
      ${filesBlock()}
      <details class="opt-card"${state.optionsOpen ? ' open' : ''}>
        <summary class="opt-summary">＋ Options<span class="opt-hint">note, expiry, password</span></summary>
        <div class="opt-body">${noteRow()}${expiryRow()}${checkRow('one-time', 'One-time (delete after first download)')}${passwordRow()}</div>
      </details>
      <div class="fld" style="gap:9px">Send to<div class="dest">${destPicker(s.loggedIn)}</div></div>
    </div>
    <footer class="modal-foot">
      ${footerReason() ? `<div class="foot-reason">${escapeHtml(footerReason())}</div>` : ''}
      <button class="btn-primary" id="primary-btn" ${canPrimary() ? '' : 'disabled'}>${escapeHtml(primaryLabel())}</button>
    </footer>
  </div>`;
  wire();
}

function destPicker(loggedIn: boolean): string {
  const opt = (d: Dest, title: string, badge: string, body = '') => `
    <div class="dest-opt ${state.dest === d ? 'sel' : ''}" data-dest-opt="${d}">
      <div class="dest-top"><span class="dot"></span>${title}${badge}</div>
      ${state.dest === d && body ? `<div class="dest-body">${body}</div>` : ''}
    </div>`;
  const guest = `<span class="free">guest</span>`;
  const need = `<span class="need">login required</span>`;
  const nearbyBody = `
    <div style="font-size:12px;color:var(--muted)">Send straight to a device on your LAN</div>
    ${
      state.peers.filter((p) => !p.isBroadcast).length
        ? state.peers.filter((p) => !p.isBroadcast).map((p) => `<div class="mini-dev"><span class="n"><b>${escapeHtml(p.name)}</b> · ${escapeHtml(p.addr)}</span>${p.code ? `<span class="tag code">${escapeHtml(p.code)}</span>` : ''}<button class="ib on send-to" data-dest="${escapeHtml(p.dest)}" title="Send" style="margin-left:4px">→</button></div>`).join('')
        : `<div class="hint">No devices found. Press ↻ on Home, or use a code below.</div>`
    }
    <div class="or-line"><span>or a code</span></div>
    <input id="net-dest" type="text" placeholder="s2u://…  or  192.168.1.5" value="${escapeHtml(state.netDest)}" />`;
  const bcBody = `
    <div style="font-size:12px;color:var(--muted)">Who can download</div>
    <div class="modes">
      ${bcMode('all', 'Allow all', 'anyone nearby')}
      ${bcMode('trusted', 'Trusted only', 'pick trusted devices')}
      ${bcMode('approve', 'Approve each', 'you allow every download')}
    </div>`;
  return (
    opt('nearby', 'Nearby device — direct', guest, nearbyBody) +
    opt('broadcast', 'Broadcast to everyone nearby', guest, bcBody) +
    opt('public', 'Public link', loggedIn ? '' : need) +
    opt('private', 'Private (email)', loggedIn ? '' : need)
  );
}

function bcMode(m: string, label: string, sub: string): string {
  return `<div class="mode ${state.bcAccess === m ? 'on' : ''}" data-bc-mode="${m}"><span class="r"></span>${label} <small>— ${sub}</small></div>`;
}

// ---- Broadcast detail ------------------------------------------------------

function renderBroadcast(): void {
  const bc = state.bc;
  if (!bc) { state.view = 'home'; return render(); }
  const sent = bc.completed.reduce((n, c) => n + c.total, 0) + bc.downloading.reduce((n, c) => n + c.sent, 0);
  root.innerHTML = `<div class="modal">
    <header class="modal-head">
      <div class="brand"><button class="btn-mini" id="bc-back">←</button> Broadcast</div>
      <div class="head-actions"><span class="tag live">live</span><button class="ib" id="bc-stop" title="Stop broadcasting" style="margin-left:8px">◼</button></div>
    </header>
    <div class="modal-body" style="padding:14px 18px 24px">
      <div class="item" style="margin-bottom:6px"><div class="ico out">📡</div><div class="line"><b>${escapeHtml(bc.name)}</b> <span class="meta">· ${fmtBytes(bc.size)} · ${escapeHtml(accessLabel(bc.access))}</span></div></div>
      <div class="stat-strip" style="margin:0 0 16px 38px"><b>${bc.downloading.length}</b>&nbsp;downloading · <b>${bc.completed.length}</b>&nbsp;done · <b>${fmtBytes(sent)}</b>&nbsp;sent</div>
      ${bc.downloading.length ? `<div class="grp-label">Downloading now · <span class="n">${bc.downloading.length}</span></div>${bc.downloading.map(connRow).join('')}` : ''}
      ${bc.completed.length ? `<div class="grp-label" style="margin-top:18px">Downloaded · <span class="n">${bc.completed.length}</span></div>${bc.completed.map(doneRow).join('')}` : ''}
      ${!bc.downloading.length && !bc.completed.length ? `<div class="empty">Waiting for someone to download… share the file's presence — they'll see it when they scan.</div>` : ''}
    </div>
  </div>`;
  wire();
}

function connRow(c: BcConn): string {
  const pct = c.total > 0 ? Math.floor((c.sent / c.total) * 100) : 0;
  return `<div class="conn"><span class="cn"><b>${escapeHtml(c.name || 'a device')}</b>${c.peer ? ` · ${escapeHtml(c.peer)}` : ''}</span><span class="bar"><span style="width:${pct}%"></span></span><span class="pct">${pct}%</span></div>`;
}
function doneRow(c: BcConn): string {
  return `<div class="conn"><span class="cn"><b>${escapeHtml(c.name || 'a device')}</b>${c.peer ? ` · ${escapeHtml(c.peer)}` : ''}</span><span class="when">done ✓</span></div>`;
}
function accessLabel(a: string): string { return a === 'all' ? 'Allow all' : a === 'trusted' ? 'Trusted only' : 'Approve each'; }

// ---- Overlays --------------------------------------------------------------

function requestOverlay(r: LanRequest): string {
  const who = escapeHtml(r.senderName || r.from);
  const verb = r.action === 'download' ? 'wants to download' : 'wants to send you';
  const trustBox = r.fingerprint
    ? `<label class="chk-trust"><input type="checkbox" id="req-trust"> Trust this device — accept automatically from now on</label>
       ${r.code ? `<div class="hint">Device code <b>${escapeHtml(r.code)}</b> — confirm it matches their screen.</div>` : ''}`
    : `<div class="hint">This device has no verified identity — it can't be trusted.</div>`;
  return `<div class="overlay"><div class="overlay-card">
    <div class="overlay-title">${r.action === 'download' ? 'Download request' : 'Incoming file'}</div>
    <div class="overlay-body"><b>${escapeHtml(r.name)}</b> <span class="hint">(${fmtBytes(r.size)})</span><div class="hint">${who} ${verb}</div></div>
    ${trustBox}
    <div class="overlay-actions"><button class="btn-hdr" id="req-reject">Decline</button><button class="btn-accept" id="req-accept">Accept</button></div>
  </div></div>`;
}

function downloadOverlay(p: LanPeer & { trust?: boolean }): string {
  return `<div class="overlay"><div class="overlay-card">
    <div class="overlay-title">Download from ${escapeHtml(p.name)}?</div>
    <div class="overlay-body"><b>${escapeHtml(p.fileName)}</b> <span class="hint">(${fmtBytes(p.fileSize)})</span></div>
    <div class="warn-line">⚠ You haven't trusted this device. Only download files from people you know — a downloaded file could be harmful.</div>
    <label class="chk-trust"><input type="checkbox" id="dl-trust"> Trust this device — auto-accept its files from now on</label>
    <div class="overlay-actions"><button class="btn-hdr" id="dl-cancel">Cancel</button><button class="btn-accept" id="dl-go">Download</button></div>
  </div></div>`;
}

// ---- Shared bits -----------------------------------------------------------

function updateBanner(): string {
  const u = state.update;
  if (!u?.available) return '';
  return `<div class="update-bar"><span>Update available — <strong>v${escapeHtml(u.latest)}</strong></span><button class="btn-mini" id="apply-update">Install</button></div>`;
}
function loginProgress(): string {
  if (state.loginPhase === 'waiting' && state.loginInfo) {
    const info = state.loginInfo;
    return `<div class="banner">Approve this device in your browser${info.userCode ? ` — code <code>${escapeHtml(info.userCode)}</code>` : ''}. <span class="hint">Waiting…</span>${info.verificationUrl ? `<button class="btn-mini" id="reopen-login">Reopen page</button>` : ''}</div>`;
  }
  if (state.loginPhase === 'error' && state.loginError) return `<div class="banner-err">${escapeHtml(state.loginError)}</div>`;
  return '';
}
function filesBlock(): string {
  if (!state.paths.length) {
    return `<div class="canvas" id="canvas"><div class="canvas-ico">⬍</div><div class="canvas-title">Drop a file here, or paste with Ctrl+V</div><div class="canvas-sub">Screenshots, images and text work too.</div>${clipChip()}</div>`;
  }
  return `<div class="files">${state.paths.map((p, i) => `<div class="file-chip" title="${escapeHtml(p)}">${escapeHtml(basename(p))}<button class="chip-x" data-i="${i}" title="Remove">×</button></div>`).join('')}<span class="files-add">＋ paste / drop</span></div>`;
}
function clipChip(): string {
  const c = state.clip;
  if (!c || c.kind === 'none') return '';
  const label = c.kind === 'image' ? '🖼 Share clipboard image' : '📄 Share copied text';
  const prev = c.kind === 'text' && c.preview ? `<div class="clip-prev">${escapeHtml(c.preview)}</div>` : '';
  return `<div class="clip-suggest"><button class="clip-chip" id="clip-add">${label}</button>${prev}</div>`;
}
function noteRow(): string { return `<label class="fld">Note <span class="hint">shown to viewers, optional</span><input id="note" type="text" maxlength="500" placeholder="e.g. Q3 report — sign by Friday" /></label>`; }
function expiryRow(): string { return `<label class="fld">Expires<select id="expires"><option value="">Default</option><option value="1h">1 hour</option><option value="1d">1 day</option><option value="7d">7 days</option><option value="30d">30 days</option><option value="keep">Keep (no expiry)</option></select></label>`; }
function passwordRow(): string { return `<label class="fld">Password <span class="hint">optional</span><input id="password" type="password" placeholder="leave blank for none" autocomplete="off" /></label>`; }
function checkRow(id: string, label: string): string { return `<label class="setting-row"><input type="checkbox" id="${id}" /><span class="setting-label">${label}</span></label>`; }

function settingsBlock(): string {
  const s = state.status!;
  return `<details class="settings"${'' /* closed by default */}>
    <summary class="settings-summary">Settings</summary>
    <div class="settings-body">
      <label class="setting-row"><input type="checkbox" id="set-discoverable" ${s.discoverable ? 'checked' : ''} /><span class="setting-label">Discoverable on local network<span class="setting-help">Nearby devices can send you files — trusted ones land automatically, others ask.</span></span></label>
      <div class="chk2">Broadcast scan interval <select id="scan-interval">${[15, 30, 60, 120, 0].map((v) => `<option value="${v}" ${state.scanInterval === v ? 'selected' : ''}>${v === 0 ? 'manual only' : 'every ' + v + 's'}</option>`).join('')}</select></div>
      <label class="setting-row"><input type="checkbox" id="set-shell" ${s.shellInstalled ? 'checked' : ''} /><span class="setting-label">Right-click Share menu</span></label>
      <label class="setting-row${s.canReceive ? '' : ' is-disabled'}"><input type="checkbox" id="set-autostart" ${s.autostartEnabled ? 'checked' : ''} ${s.canReceive ? '' : 'disabled'} /><span class="setting-label">Auto-receive files at login</span></label>
      ${trustedBlock()}
      ${state.activity.length ? `<button class="btn-mini" id="clear-activity">Clear activity log</button>` : ''}
    </div>
  </details>`;
}
function trustedBlock(): string {
  if (!state.trusted.length) return '';
  return `<div class="trusted-list"><div class="setting-label">Trusted devices <span class="hint">auto-accept, no code</span></div>${state.trusted.map((d) => `<div class="trusted-row"><span class="trusted-name" title="${escapeHtml(d.fingerprint)}">${escapeHtml(d.name || d.fingerprint.slice(0, 10))}</span><button class="btn-mini trusted-revoke" data-fp="${escapeHtml(d.fingerprint)}">Revoke</button></div>`).join('')}</div>`;
}

// ---- Primary button (share modal) ------------------------------------------

function primaryLabel(): string {
  if (state.dest === 'broadcast') return 'Start broadcast';
  if (state.dest === 'nearby') return 'Send';
  return 'Create link';
}
function canPrimary(): boolean {
  if (!state.paths.length) return false;
  if (state.dest === 'public' || state.dest === 'private') return !!state.status?.loggedIn;
  return true;
}
function footerReason(): string {
  if (!state.paths.length) return 'Add a file above to share.';
  if ((state.dest === 'public' || state.dest === 'private') && !state.status?.loggedIn) return 'Login to share to the cloud.';
  if (state.dest === 'nearby') return 'Pick a device above, or enter a code and press Send.';
  return '';
}
async function onPrimary() {
  if (state.dest === 'broadcast') return startBroadcast();
  if (state.dest === 'nearby') {
    const dest = (root.querySelector<HTMLInputElement>('#net-dest')?.value || '').trim();
    if (!dest) { root.querySelector<HTMLInputElement>('#net-dest')?.focus(); return; }
    return sendTo(dest);
  }
  return doShare();
}

async function sendTo(dest: string) {
  const outcomes = await backend().LanSend(state.paths, dest, '').catch((e) => [{ path: '', ok: false, error: String(e) } as ShareOutcome]);
  const ok = outcomes.every((o) => o.ok);
  state.view = 'home';
  state.paths = [];
  await refreshActivity();
  render();
  if (!ok) toast(outcomes.find((o) => !o.ok)?.error || 'Send failed');
}

async function startBroadcast() {
  const path = state.paths[0];
  if (!path) return;
  try {
    state.bc = await backend().StartBroadcast(path, state.bcAccess);
    state.view = 'broadcast';
    state.paths = [];
    render();
  } catch (e) {
    toast(String(e));
  }
}

async function doShare() {
  const val = (id: string) => root.querySelector<HTMLInputElement>('#' + id)?.value?.trim() || '';
  const checked = (id: string) => !!root.querySelector<HTMLInputElement>('#' + id)?.checked;
  const req: ShareRequest = { paths: state.paths, target: state.dest };
  const expires = root.querySelector<HTMLSelectElement>('#expires')?.value || '';
  if (expires === 'keep') req.keep = true; else if (expires) req.expires = expires;
  req.password = val('password') || undefined;
  req.oneTime = checked('one-time');
  req.note = val('note') || undefined;
  if (state.dest === 'private') req.recipients = val('recipients').split(',').map((x) => x.trim()).filter(Boolean);
  const btn = root.querySelector<HTMLButtonElement>('#primary-btn')!;
  btn.disabled = true; btn.textContent = 'Sharing…';
  try {
    const out = await backend().Share(req);
    const link = out.find((o) => o.ok && o.link)?.link;
    if (link) { copy(link); toast('Link copied to clipboard'); }
    state.view = 'home'; state.paths = [];
    await refreshActivity(); render();
  } catch (e) { btn.disabled = false; btn.textContent = primaryLabel(); toast(String(e)); }
}

// ---- Downloads + broadcast lifecycle ---------------------------------------

async function doDownload() {
  const p = state.dl;
  if (!p) return;
  const wantTrust = !!root.querySelector<HTMLInputElement>('#dl-trust')?.checked;
  state.dl = null; render();
  toast('Downloading ' + p.fileName + '…');
  try {
    const res = await backend().LanDownload(p.addr, p.fingerprint, p.fileName, p.fileSize);
    if (wantTrust && res.fingerprint) await backend().TrustDevice(res.fingerprint, p.name).catch(() => {});
    await refreshActivity(); loadTrusted();
  } catch (e) { toast('Download failed: ' + String(e)); }
}

async function stopBroadcast() {
  try { await backend().StopBroadcast(); } catch { /* ignore */ }
  state.bc = null; state.view = 'home'; render();
}

// ---- Data refresh ----------------------------------------------------------

async function refreshActivity() { try { state.activity = (await backend().ActivityLog()) || []; } catch { /* */ } }
async function loadTrusted() { try { state.trusted = (await backend().ListTrusted()) || []; render(); } catch { /* */ } }
let scanTimer = 0;
async function findNearby(quiet = false) {
  if (state.browsing) return;
  state.browsing = true; if (!quiet) render();
  try { state.peers = (await backend().LanBrowse()) || []; } catch { /* keep last list */ }
  state.browsing = false;
  // Only repaint the feed when we're actually looking at it, so a background
  // scan never yanks an open modal out from under the user.
  if (state.view === 'home') render();
}

// startScanTimer re-scans for nearby devices/broadcasts every scanInterval
// seconds (0 = manual refresh only). The tick is quiet — no scanning flash.
function startScanTimer() {
  if (scanTimer) { clearInterval(scanTimer); scanTimer = 0; }
  const sec = state.scanInterval;
  if (!sec || sec <= 0) return;
  scanTimer = window.setInterval(() => { if (state.view === 'home') findNearby(true); }, sec * 1000);
}
async function checkForUpdate() { try { const info = await backend().CheckUpdate(); if (info?.available) { state.update = info; render(); } } catch { /* */ } }
async function checkClipboard() {
  try { const c = await backend().ClipboardSuggestion(); state.clip = c && c.kind !== 'none' ? c : null; render(); } catch { /* */ }
}
async function addClipboard() { const c = state.clip; if (!c) return; try { addPaths([await backend().AddClipboard(c.kind)]); state.view = 'share'; render(); } catch (e) { toast(String(e)); } }

// ---- Wiring ----------------------------------------------------------------

function wire() {
  const on = (sel: string, ev: string, fn: (e: Event) => void) => root.querySelectorAll(sel).forEach((el) => el.addEventListener(ev, fn));
  root.querySelector('#theme-toggle')?.addEventListener('click', toggleTheme);
  root.querySelector('#check-update')?.addEventListener('click', manualUpdateCheck);
  root.querySelector('#login-btn')?.addEventListener('click', signIn);
  root.querySelector('#reopen-login')?.addEventListener('click', () => backend().BeginLogin());
  root.querySelector('#logout-btn')?.addEventListener('click', logout);
  root.querySelector('#apply-update')?.addEventListener('click', applyUpdate);
  root.querySelector('#open-share')?.addEventListener('click', () => { state.view = 'share'; render(); });
  root.querySelector('#share-back')?.addEventListener('click', () => { state.view = 'home'; render(); });
  root.querySelector('#nearby-find')?.addEventListener('click', () => findNearby());
  root.querySelector('#clip-add')?.addEventListener('click', addClipboard);
  root.querySelector('#primary-btn')?.addEventListener('click', onPrimary);
  root.querySelector('#bc-back')?.addEventListener('click', () => { state.view = 'home'; render(); });
  root.querySelectorAll('#bc-stop').forEach((b) => b.addEventListener('click', stopBroadcast));
  root.querySelector('#live-row')?.addEventListener('click', (e) => { if (!(e.target as HTMLElement).closest('#bc-stop')) { state.view = 'broadcast'; render(); } });
  on('.chip-x', 'click', (e) => { state.paths.splice(Number((e.currentTarget as HTMLElement).dataset.i), 1); render(); });
  on('.send-to', 'click', (e) => sendTo((e.currentTarget as HTMLElement).dataset.dest || ''));
  on('.dl-btn', 'click', (e) => { const fp = (e.currentTarget as HTMLElement).dataset.fp; const p = state.peers.find((x) => x.isBroadcast && x.fingerprint === fp); if (p) { state.dl = p; render(); } });
  on('.dest-opt', 'click', (e) => { state.dest = (e.currentTarget as HTMLElement).dataset.destOpt as Dest; render(); });
  on('.dest-opt input, .dest-opt .send-to, .dest-opt .mode', 'click', (e) => e.stopPropagation());
  on('.mode', 'click', (e) => { state.bcAccess = (e.currentTarget as HTMLElement).dataset.bcMode as any; render(); });
  const nd = root.querySelector<HTMLInputElement>('#net-dest');
  nd?.addEventListener('input', () => (state.netDest = nd.value));
  root.querySelector<HTMLDetailsElement>('.opt-card')?.addEventListener('toggle', (e) => (state.optionsOpen = (e.target as HTMLDetailsElement).open));
  // request approval overlay
  root.querySelector('#req-reject')?.addEventListener('click', () => respondRequest(false));
  root.querySelector('#req-accept')?.addEventListener('click', async () => {
    const r = state.requests[0];
    if (r && root.querySelector<HTMLInputElement>('#req-trust')?.checked && r.fingerprint) { try { await backend().TrustDevice(r.fingerprint, r.senderName || r.from); loadTrusted(); } catch { /* */ } }
    respondRequest(true);
  });
  // download confirm overlay
  root.querySelector('#dl-cancel')?.addEventListener('click', () => { state.dl = null; render(); });
  root.querySelector('#dl-go')?.addEventListener('click', doDownload);
  // settings
  const disc = root.querySelector<HTMLInputElement>('#set-discoverable');
  disc?.addEventListener('change', async () => { try { await backend().SetDiscoverable(disc.checked); if (state.status) state.status.discoverable = disc.checked; if (!disc.checked) state.discCode = ''; render(); } catch { disc.checked = !disc.checked; } });
  const si = root.querySelector<HTMLSelectElement>('#scan-interval');
  si?.addEventListener('change', async () => { state.scanInterval = Number(si.value); try { await backend().SetScanInterval(state.scanInterval); } catch { /* */ } startScanTimer(); });
  wireToggle('set-shell', (o) => backend().SetShellIntegration(o));
  wireToggle('set-autostart', (o) => backend().SetAutostart(o));
  on('.trusted-revoke', 'click', async (e) => { try { await backend().UntrustDevice((e.currentTarget as HTMLElement).dataset.fp || ''); } catch { /* */ } loadTrusted(); });
  root.querySelector('#clear-activity')?.addEventListener('click', async () => { try { await backend().ClearActivity(); } catch { /* */ } state.activity = []; render(); });
}

function wireToggle(id: string, fn: (on: boolean) => Promise<void>) {
  const el = root.querySelector<HTMLInputElement>('#' + id);
  el?.addEventListener('change', async () => {
    try { await fn(el.checked); } catch { el.checked = !el.checked; }
    try { state.status = await backend().Status(); } catch { /* */ }
  });
}

async function respondRequest(accept: boolean) {
  const r = state.requests.shift();
  render();
  if (r) { try { await backend().RespondLanRequest(r.id, accept); } catch { /* */ } }
}

async function signIn() {
  state.loginPhase = 'waiting'; state.loginError = '';
  try {
    state.loginInfo = await backend().BeginLogin(); render();
    state.status = await backend().CompleteLogin();
    state.loginPhase = 'idle'; state.loginInfo = null; render();
  } catch (e) { state.loginPhase = 'error'; state.loginError = String(e); state.loginInfo = null; render(); }
}
async function logout() { try { await backend().Logout(); } catch { /* */ } try { state.status = await backend().Status(); } catch { /* */ } state.view = 'home'; render(); }

async function applyUpdate(e: Event) {
  const btn = e.currentTarget as HTMLButtonElement; btn.disabled = true; btn.textContent = 'Updating…';
  try { await backend().ApplyUpdate(); } catch { btn.disabled = false; btn.textContent = 'Install'; }
}
async function manualUpdateCheck(e: Event) {
  const btn = e.currentTarget as HTMLButtonElement; btn.disabled = true; const prev = btn.textContent; btn.textContent = '…';
  try {
    const info = await backend().CheckUpdate();
    if (info?.available) { state.update = info; render(); return; }
    btn.textContent = '✓'; setTimeout(() => { btn.textContent = prev; btn.disabled = false; }, 1500);
  } catch { btn.textContent = prev; btn.disabled = false; }
}

// ---- Events + input --------------------------------------------------------

let listenersReady = false;
function setupListeners() {
  if (listenersReady) return; listenersReady = true;
  document.addEventListener('paste', onPaste);
  const rt = (window as any).runtime;
  rt?.EventsOn?.('files-dropped', (paths: string[]) => { addPaths(paths || []); state.view = 'share'; render(); });
  rt?.EventsOn?.('lan-request', (r: any) => {
    if (!r?.id) return;
    state.requests.push({ id: String(r.id), from: String(r.from || ''), name: String(r.name || 'file'), size: Number(r.size) || 0, fingerprint: String(r.fingerprint || ''), senderName: String(r.senderName || ''), code: String(r.code || ''), action: String(r.action || 'send') });
    render();
  });
  rt?.EventsOn?.('lan-recv-done', () => { refreshActivity().then(render); });
  rt?.EventsOn?.('lan-discoverable', (d: any) => { if (!d?.error) { state.discCode = String(d?.code || ''); render(); } });
  rt?.EventsOn?.('lan-bc-conn', async () => { try { state.bc = await backend().BroadcastStats(); if (state.view === 'broadcast' || (state.view === 'home')) render(); } catch { /* */ } });
  window.addEventListener('focus', () => checkClipboard());
}

function addPaths(paths: string[]) {
  let changed = false;
  for (const p of paths) if (p && !state.paths.includes(p)) { state.paths.push(p); changed = true; }
  if (changed) render();
}

async function onPaste(e: ClipboardEvent) {
  if (state.view !== 'home' && state.view !== 'share') return;
  if ((e.target as HTMLElement)?.closest?.('input, textarea')) return;
  const cd = e.clipboardData; if (!cd) return;
  for (const item of Array.from(cd.items)) {
    if (item.kind === 'file' && item.type.startsWith('image/')) {
      const blob = item.getAsFile();
      if (blob) { e.preventDefault(); try { addPaths([await backend().AddPasted(extFromMime(item.type), await blobToBase64(blob))]); state.view = 'share'; render(); } catch { /* */ } return; }
    }
  }
  const text = cd.getData('text');
  if (text && text.trim()) { e.preventDefault(); try { addPaths([await backend().AddPasted(looksLikeMarkdown(text) ? 'md' : 'txt', utf8ToBase64(text))]); state.view = 'share'; render(); } catch { /* */ } }
}

// ---- Tiny helpers ----------------------------------------------------------

function toast(msg: string) {
  const t = document.createElement('div');
  t.className = 'toast'; t.textContent = msg;
  document.body.appendChild(t);
  setTimeout(() => t.classList.add('in'));
  setTimeout(() => { t.classList.remove('in'); setTimeout(() => t.remove(), 300); }, 2600);
}
function fmtBytes(n?: number): string {
  if (!n || n <= 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB']; let i = 0, v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(v < 10 && i > 0 ? 1 : 0)} ${u[i]}`;
}
function ago(ts: number): string {
  if (!ts) return '';
  const s = Math.max(0, Math.floor(Date.now() / 1000 - ts));
  if (s < 60) return 'just now';
  if (s < 3600) return Math.floor(s / 60) + 'm ago';
  if (s < 86400) return Math.floor(s / 3600) + 'h ago';
  return Math.floor(s / 86400) + 'd ago';
}
function copy(text: string) { const rt = (window as any).runtime; if (rt?.ClipboardSetText) rt.ClipboardSetText(text); else navigator.clipboard?.writeText(text).catch(() => {}); }
function basename(p: string): string { const parts = p.split(/[\\/]/); return parts[parts.length - 1] || p; }
function escapeHtml(s: string): string { return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c] as string)); }
function blobToBase64(blob: Blob): Promise<string> { return new Promise((res, rej) => { const r = new FileReader(); r.onload = () => { const s = String(r.result); res(s.slice(s.indexOf(',') + 1)); }; r.onerror = () => rej(new Error('read failed')); r.readAsDataURL(blob); }); }
function utf8ToBase64(s: string): string { return btoa(unescape(encodeURIComponent(s))); }
function extFromMime(mime: string): string { return ({ 'image/png': 'png', 'image/jpeg': 'jpg', 'image/gif': 'gif', 'image/webp': 'webp', 'image/bmp': 'bmp' } as Record<string, string>)[mime] || 'png'; }
function looksLikeMarkdown(t: string): boolean { return /(^|\n)\s{0,3}(#{1,6}\s|[-*+]\s|\d+\.\s|>\s|```)/.test(t) || /\[[^\]]+\]\([^)]+\)/.test(t); }

boot();
