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
// A file that has arrived and is waiting in staging until the user says where it
// goes. Deliberately not written to a default folder behind their back.
type Incoming = { id: string; name: string; size: number; from: string; at: string; file: string };

type UpdateInfo = { available: boolean; current: string; latest: string; assetUrl: string; assetName: string; page: string; channel: string; prerelease: boolean };
type LanPeer = {
  name: string; addr: string; dest: string; code: string; mode: string;
  fingerprint: string; isBroadcast: boolean; fileName: string; fileSize: number;
};
type LanRequest = { id: string; from: string; name: string; size: number; fingerprint: string; senderName: string; code: string; action: string; trusted?: boolean };
type TrustedDevice = { fingerprint: string; name: string; mode: 'ask' | 'auto' };
type TrustChallenge = { challengeId: string; factor: string; sentTo: string; verifyCode: string; safetyNumber: string; expiresIn: number };
type TrustPrompt = TrustChallenge & { fingerprint: string; name: string; mode: string; error: string; busy: boolean };
type ClipSuggestion = { kind: 'image' | 'text' | 'none'; preview: string; ext: string };
type Activity = { kind: string; peer: string; name: string; size: number; ts: number; link?: string };
type BcConn = { fingerprint: string; name: string; peer: string; sent: number; total: number; done: boolean; err: string };
type BroadcastState = { active: boolean; name: string; size: number; access: string; downloading: BcConn[]; completed: BcConn[] };
type DownloadResult = { name: string; fingerprint: string; from: string; trusted: boolean };

interface AppBackend {
  Status(): Promise<Status>;
  PendingPaths(): Promise<string[]>;
  PickFiles(): Promise<string[]>;
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
  IsStoreManaged(): Promise<boolean>;
  UpdateChannel(): Promise<string>;
  BuildVersion(): Promise<string>;
  IncomingList(): Promise<Incoming[]>;
  IncomingFolder(): Promise<string>;
  SetIncomingFolder(dir: string): Promise<void>;
  ChooseIncomingFolder(): Promise<string>;
  SaveIncoming(id: string): Promise<string>;
  DiscardIncoming(id: string): Promise<void>;
  SetUpdateChannel(channel: string): Promise<void>;
  LanSend(paths: string[], dest: string, password: string): Promise<ShareOutcome[]>;
  LanBrowse(): Promise<LanPeer[]>;
  SetDiscoverable(on: boolean): Promise<void>;
  RespondLanRequest(id: string, accept: boolean): Promise<void>;
  TrustDevice(fingerprint: string, name: string, mode: string): Promise<TrustChallenge>;
  VerifyTrust(challengeId: string, code: string): Promise<void>;
  SetTrustMode(fingerprint: string, mode: string): Promise<void>;
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
  trustPrompt: null as TrustPrompt | null, // ADR-034 code entry after 'Trust this device'
  activity: [] as Activity[],
  trusted: [] as TrustedDevice[],
  bc: null as BroadcastState | null, // live broadcast
  discCode: '' as string,
  discSafety: '' as string, // this device's safety number (compare before trusting it from elsewhere)
  clip: null as ClipSuggestion | null,
  theme: 'dark' as 'dark' | 'light',
  update: null as UpdateInfo | null,
  storeManaged: false as boolean, // Microsoft Store build/install -> updater hidden
  updateChannel: 'stable' as string, // 'stable' | 'beta'; shared with the CLI via config.json

  incoming: [] as Incoming[], // staged arrivals awaiting a save location
  incomingFolder: '' as string, // remembered destination; '' means ask each time
  pendingRemember: '' as string, // folder just used, offered as the default once
  shaiOpen: false as boolean, // the coming-soon panel behind the header launcher
  // The device chosen to send to. Picking one SELECTS it rather than sending
  // immediately, so the button can name it truthfully and there is a moment to
  // notice you picked the wrong machine before the file leaves.
  picked: null as { dest: string; name: string } | null,
  scanInterval: 60 as number,
  buildVersion: '' as string,
  // share modal
  dest: 'nearby' as Dest,
  // Which half of the send flow is showing. Derived from dest so the two can
  // never disagree; switching modes picks that half's default destination.
  sendMode: 'device' as 'device' | 'link',
  bcAccess: 'approve' as 'all' | 'trusted' | 'approve',
  optionsOpen: false as boolean,
  netDest: '' as string,
  // login
  loginPhase: 'idle' as 'idle' | 'waiting' | 'error',
  loginInfo: null as LoginInfo | null,
  loginError: '' as string,
  // download confirm overlay
  dl: null as (LanPeer & { trust?: boolean }) | null,
  // cloud share result (persistent copyable link)
  shareResult: null as { name: string; link: string; kind: string } | null,
};

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
    state.storeManaged = await backend().IsStoreManaged().catch(() => false);
    state.updateChannel = await backend().UpdateChannel().catch(() => 'stable');
    state.buildVersion = await backend().BuildVersion().catch(() => '');
    state.incomingFolder = await backend().IncomingFolder().catch(() => '');
    await refreshIncoming();
    state.bc = await backend().BroadcastStats().catch(() => null);
    if (state.bc && !state.bc.active) state.bc = null;
    // Opened via the Share verb with files -> jump straight to the Share modal.
    if (state.paths.length) state.view = 'share';
    render();
    setupListeners();
    refreshActivity();
    loadTrusted();
    if (!state.storeManaged) checkForUpdate();
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
  // Update sits beside the account because both answer the same question: is this
  // app of mine in good order? The dot IS the notification, so nothing appears or
  // disappears and the row never reflows (design rule: no layout shift).
  const upd = state.storeManaged
    ? ''
    : `<button class="icon-btn upd-btn${state.update?.available ? ' has-dot' : ''}" id="check-update" aria-label="${state.update?.available ? 'Install the new version of Share2Us' : 'Check for a new version of Share2Us'}" title="${state.update?.available ? 'A new version of Share2Us is ready to install' : 'Check for a new version of Share2Us'}"><svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M8 2v7"/><path d="M5 6.5 8 9.5l3-3"/><path d="M2.5 11.5v1a1.5 1.5 0 0 0 1.5 1.5h8a1.5 1.5 0 0 0 1.5-1.5v-1"/></svg></button>`;
  return `<header class="modal-head">
    ${
      s.loggedIn
        ? `<span class="who" title="${escapeHtml(s.email)}">${escapeHtml(s.email)}</span>`
        : `<button class="btn-hdr" id="login-btn">Login</button>`
    }
    <div class="head-actions">
      <button class="icon-btn shai-btn" id="shai-open" title="Shai — coming soon">✦</button>
      ${upd}
    </div>
  </header>`;
}



// Persistent status strip along the bottom. Discoverability lives here rather
// than inside a collapsed Settings block, because it decides whether anyone can
// find this machine and hiding it produced a "nearby share is not working"
// report that was nothing of the kind. The theme toggle sits at the far left:
// it is set once and then never touched, so it does not deserve header space.
function statusStrip(): string {
  const on = !!state.status?.discoverable;
  const code = on && state.discCode ? ` · code ${escapeHtml(state.discCode)}` : '';
  const near = state.peers.filter((p) => !p.isBroadcast).length;
  return `<div class="status-strip">
    <button class="strip-theme" id="theme-toggle" title="Toggle light/dark">${state.theme === 'dark' ? '☀' : '☾'}</button>
    <span class="strip-dot${on ? '' : ' off'}"></span>
    <span class="strip-txt">${on ? `Discoverable${near ? ` · ${near} nearby` : ''}${code}` : 'Not discoverable'}</span>
    <span class="strip-sp"></span>
    <button class="strip-link" id="open-settings">Settings</button>
  </div>`;
}

// Shai's shell. The agent, chat and voice are not built (todo I); this says so
// plainly and describes what it will do, including the part people are right to
// ask about first: what it will never do without permission.
//
// Drawn as unmistakably unavailable rather than dressed up as live, because a
// control that looks working and does nothing gets reported as a bug.
function shaiPanel(): string {
  if (!state.shaiOpen) return '';
  return `<div class="shai-panel" id="shai-panel">
    <div class="shai-head">
      <span class="shai-av">✦</span>
      <b>Shai</b>
      <span class="shai-soon">Coming soon</span>
      <button class="ib" id="shai-close" title="Close" style="margin-left:auto">✕</button>
    </div>
    <div class="shai-body">
      Ask for the outcome and Shai does the steps, instead of you finding the screen:
      <ul>
        <li><b>“Send this folder to my laptop”</b></li>
        <li><b>“Make a link that expires tomorrow”</b></li>
        <li><b>“Who downloaded the report?”</b></li>
      </ul>
      Anything that cannot be undone, such as revoking a link, deleting a share or
      trusting a device, it asks you to confirm first. It can never do more than
      you can.
    </div>
    <div class="shai-ask">Ask Shai…<span class="shai-mic">🎙</span></div>
  </div>`;
}

// ---- Home ------------------------------------------------------------------


// Build stamp in the window footer. Always rendered, even before the value
// arrives, so it can never appear late and reflow the frame (design rule: no
// layout shift). Click copies it, because the point of showing it is to be able
// to quote it in a bug report.
function buildStrip(): string {
  const v = state.buildVersion;
  return `<div class="build-strip" id="build-strip" title="${v ? 'Click to copy' : ''}">${
    v ? `version <span class="build-ver">${escapeHtml(v)}</span>` : '&nbsp;'
  }</div>`;
}

function renderHome(): void {
  root.innerHTML = `<div class="modal">
    ${header()}
    ${updateBanner()}
    ${loginProgress()}
    <div class="home">
      <button class="share-cta" id="open-share"><span class="plus">+</span> Share a file</button>
      <div class="feed-scroll">${sectionNearby()}${sectionIncoming()}${sectionRecent()}</div>
      ${settingsBlock()}
    </div>
    ${state.requests.length ? requestOverlay(state.requests[0]) : ''}
    ${state.trustPrompt && !state.requests.length ? trustCodeOverlay(state.trustPrompt) : ''}
    ${state.dl ? downloadOverlay(state.dl) : ''}
    ${state.shareResult ? shareResultOverlay(state.shareResult) : ''}
    ${shaiPanel()}
    ${statusStrip()}
    ${buildStrip()}
  </div>`;
  wire();
}

// Home is three separate questions, not one list: who can I reach, what has
// arrived that I have not filed, and what happened lately. They used to share a
// single "Activity & nearby" feed, which answered none of them well.
function sectionNearby(): string {
  const rows: string[] = [];
  for (const p of state.peers.filter((x) => x.isBroadcast)) rows.push(bcastRow(p));
  if (state.bc && state.bc.active) rows.push(liveRow(state.bc));
  for (const p of state.peers.filter((x) => !x.isBroadcast)) rows.push(nearbyRow(p));
  const body = rows.length
    ? rows.join('')
    : `<div class="empty">No devices found. A device appears here only while Share2Us is open on it <b>and</b> its “Discoverable on local network” setting is on. Press ↻ to scan again.</div>`;
  return `<div class="sec-head"><b>Nearby</b><button class="refresh" id="nearby-find" aria-label="Scan for nearby devices again" title="Scan for nearby devices again (automatic every ${state.scanInterval || 60}s). This does not update the app.">↻</button></div>${body}`;
}

// Only rendered when something is waiting: an empty section would be a permanent
// reminder of nothing.
function sectionIncoming(): string {
  // The offer to remember a folder must outlive the list. Saving the LAST
  // waiting file empties it, and an early return here made the offer disappear
  // at exactly the moment it was earned.
  const remember = state.pendingRemember
    ? `<div class="warn-line">Always save received files to <b>${escapeHtml(state.pendingRemember)}</b>?
         <button class="btn-mini" id="remember-folder">Always save here</button>
         <button class="btn-mini" id="remember-dismiss" style="background:transparent;color:var(--text-2)">Not now</button></div>`
    : '';
  if (!state.incoming.length) {
    return remember ? `<div class="sec-head"><b>Incoming</b></div>${remember}` : '';
  }
  const rows = state.incoming
    .map(
      (f) => `<div class="item">
      <div class="ico">📥</div>
      <div class="line"><b>${escapeHtml(f.name)}</b> <span class="meta">· ${fmtBytes(f.size)} · from ${escapeHtml(f.from)}</span></div>
      <div class="acts"><button class="ib save-incoming" data-id="${escapeHtml(f.id)}" title="Save to this device">⤓</button></div>
    </div>`,
    )
    .join('');
  return `<div class="sec-head"><b>Incoming</b><span class="meta">waiting to be saved</span></div>${rows}${remember}`;
}

// Capped at five. The full history lives in the portal rather than being rebuilt
// here, so this stays a glance and not a second product.
function sectionRecent(): string {
  const all = state.activity;
  if (!all.length) return '';
  const rows = all.slice(0, 5).map(logRow).join('');
  const more = all.length > 5
    ? `<div class="item log"><div class="line"><button class="strip-link" id="open-history">Show all in the portal ↗</button></div></div>`
    : '';
  return `<div class="sec-head"><b>Recent</b></div>${rows}${more}`;
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
  const now = (bc.downloading || []).length, done = (bc.completed || []).length;
  return `<div class="item" id="live-row">
    <div class="ico out">📡</div>
    <div class="line"><b>Broadcasting</b> ${escapeHtml(bc.name)} <span class="meta">· ${now} now · ${done} done</span> <span class="tag live">live</span></div>
    <div class="acts"><button class="ib" id="bc-stop" title="Stop broadcasting">◼</button></div>
  </div>`;
}

function logRow(a: Activity): string {
  if (a.kind === 'link') {
    return `<div class="item log">
      <div class="ico done">🔗</div>
      <div class="line">Shared <b>${escapeHtml(a.name)}</b> <span class="meta">· ${escapeHtml(a.peer)} · ${ago(a.ts)}</span></div>
      ${a.link ? `<div class="acts"><button class="ib copy-link" data-link="${escapeHtml(a.link)}" title="Copy link">⧉</button></div>` : ''}
    </div>`;
  }
  const verb: Record<string, string> = { sent: 'Sent', received: 'Received', downloaded: 'Downloaded', broadcast: 'Broadcast to' };
  const who = a.peer ? ` ${a.kind === 'sent' ? 'to' : a.kind === 'broadcast' ? '' : 'from'} ${escapeHtml(a.peer)}` : '';
  return `<div class="item log">
    <div class="ico done">✓</div>
    <div class="line">${verb[a.kind] || a.kind} <b>${escapeHtml(a.name)}</b>${who} <span class="meta">· ${ago(a.ts)}</span></div>
  </div>`;
}

function shareResultOverlay(r: { name: string; link: string; kind: string }): string {
  const title = r.kind === 'private' ? 'Private link ready' : 'Public link ready';
  return `<div class="overlay"><div class="overlay-card">
    <div class="overlay-title">${title}</div>
    <div class="overlay-body"><b>${escapeHtml(r.name)}</b> is shared. The link is on your clipboard:</div>
    <input class="link-field" id="share-link" type="text" readonly value="${escapeHtml(r.link)}" />
    <div class="overlay-actions"><button class="btn-hdr" id="share-done">Done</button><button class="btn-accept" id="share-copy">Copy link</button></div>
  </div></div>`;
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
      ${sendModeTabs()}
      <div class="dest">${destPicker(s.loggedIn)}</div>
    </div>
    <footer class="modal-foot">
      ${footerReason() ? `<div class="foot-reason">${escapeHtml(footerReason())}</div>` : ''}
      <button class="btn-primary" id="primary-btn" ${canPrimary() ? '' : 'disabled'}>${escapeHtml(primaryLabel())}</button>
    </footer>
    ${shaiPanel()}
    ${statusStrip()}
    ${buildStrip()}
  </div>`;
  wire();
}

// Two paths, because that is the question people actually ask: am I handing this
// to someone who is here, or making a URL? Four co-equal radio cards mixed direct
// transfers with hosted links and made both harder to find.
function sendModeTabs(): string {
  const tab = (m: 'device' | 'link', label: string) =>
    `<button class="seg-btn${state.sendMode === m ? ' active' : ''}" data-send-mode="${m}">${label}</button>`;
  return `<div class="seg">${tab('device', 'To a device')}${tab('link', 'Create a link')}</div>`;
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
    <div style="font-size:12px;color:var(--text-2)">Send straight to a device on your LAN</div>
    ${
      state.status?.discoverable
        ? ''
        : `<div class="warn-line">This device is not discoverable, so other devices cannot see it or send to you.
             <button class="btn-mini" id="dest-make-disc">Make discoverable</button></div>`
    }
    ${
      state.peers.filter((p) => !p.isBroadcast).length
        ? state.peers.filter((p) => !p.isBroadcast).map((p) => `<div class="mini-dev${state.picked?.dest === p.dest ? ' picked' : ''}"><span class="n"><b>${escapeHtml(p.name)}</b> · ${escapeHtml(p.addr)}</span>${p.code ? `<span class="tag code" title="Verify code. Check it matches what that device shows, so you know it is really them. It is not something you type in here.">${escapeHtml(p.code)}</span>` : ''}<button class="ib${state.picked?.dest === p.dest ? ' on' : ''} pick-dev" data-dest="${escapeHtml(p.dest)}" data-name="${escapeHtml(p.name)}" title="${state.picked?.dest === p.dest ? 'Selected' : 'Select this device'}" style="margin-left:4px">${state.picked?.dest === p.dest ? '✓' : '→'}</button></div>`).join('')
        : `<div class="hint">No devices found. A device appears here only while Share2Us is open on it <b>and</b> its “Discoverable on local network” setting is on — turn that on over there, then press ↻ on Home. Or type its address below, which also works when the two devices are on different networks and cannot see each other.</div>`
    }
    <div class="or-line"><span>or enter its address</span></div>
    <div class="addr-row">
      <input id="net-dest" type="text" spellcheck="false" autocapitalize="off"
             placeholder="192.168.1.5  or  s2u://…"
             title="The device's address, or the s2u:// link it shows. The 6-digit verify code is for checking identity, not for finding a device."
             value="${escapeHtml(state.netDest)}" />
      <button id="net-dest-use" class="btn-mini" ${state.netDest.trim() ? '' : 'disabled'}
              title="Use this address as the destination">Use</button>
    </div>`;
  const bcBody = `
    <div style="font-size:12px;color:var(--text-2)">Who can download</div>
    <div class="modes">
      ${bcMode('all', 'Allow all', 'anyone nearby')}
      ${bcMode('trusted', 'Trusted only', 'pick trusted devices')}
      ${bcMode('approve', 'Approve each', 'you allow every download')}
    </div>`;

  if (state.sendMode === 'device') {
    return (
      opt('nearby', 'A device on this network', guest, nearbyBody) +
      opt('broadcast', 'Everyone nearby', guest, bcBody)
    );
  }
  // Link options (expiry, password, one-time, note) only mean something here, so
  // they live on this path instead of sitting under every destination.
  const linkBody = `
    <details class="opt-card"${state.optionsOpen ? ' open' : ''} style="margin-top:9px">
      <summary class="opt-summary">＋ Options<span class="opt-hint">note, expiry, password</span></summary>
      <div class="opt-body">${noteRow()}${expiryRow()}${checkRow('one-time', 'One-time (delete after first download)')}${passwordRow()}</div>
    </details>`;
  return (
    opt('public', 'Anyone with the link', loggedIn ? '' : need, linkBody) +
    opt('private', 'Only these people', loggedIn ? '' : need, linkBody)
  );
}


function bcMode(m: string, label: string, sub: string): string {
  return `<div class="mode ${state.bcAccess === m ? 'on' : ''}" data-bc-mode="${m}"><span class="r"></span>${label} <small>— ${sub}</small></div>`;
}

// ---- Broadcast detail ------------------------------------------------------

function renderBroadcast(): void {
  const bc = state.bc;
  if (!bc) { state.view = 'home'; return render(); }
  // Guard: a just-started broadcast has no connections yet, and Go marshals a nil
  // slice as null — so these can be undefined.
  const downloading = bc.downloading || [];
  const completed = bc.completed || [];
  const sent = completed.reduce((n, c) => n + c.total, 0) + downloading.reduce((n, c) => n + c.sent, 0);
  root.innerHTML = `<div class="modal">
    <header class="modal-head">
      <div class="brand"><button class="btn-mini" id="bc-back">←</button> Broadcast</div>
      <div class="head-actions"><span class="tag live">live</span><button class="ib" id="bc-stop" title="Stop broadcasting" style="margin-left:8px">◼</button></div>
    </header>
    <div class="modal-body" style="padding:14px 18px 24px">
      <div class="item" style="margin-bottom:6px"><div class="ico out">📡</div><div class="line"><b>${escapeHtml(bc.name)}</b> <span class="meta">· ${fmtBytes(bc.size)} · ${escapeHtml(accessLabel(bc.access))}</span></div></div>
      <div class="stat-strip" style="margin:0 0 16px 38px"><b>${downloading.length}</b>&nbsp;downloading · <b>${completed.length}</b>&nbsp;done · <b>${fmtBytes(sent)}</b>&nbsp;sent</div>
      ${downloading.length ? `<div class="grp-label">Downloading now · <span class="n">${downloading.length}</span></div>${downloading.map(connRow).join('')}` : ''}
      ${completed.length ? `<div class="grp-label" style="margin-top:18px">Downloaded · <span class="n">${completed.length}</span></div>${completed.map(doneRow).join('')}` : ''}
      ${!downloading.length && !completed.length ? `<div class="empty">Waiting for someone to download… they'll see it when they scan nearby.</div>` : ''}
    </div>
    ${shaiPanel()}
    ${statusStrip()}
    ${buildStrip()}
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
  // Three shapes, same slot: trusted (ask mode), untrusted with an identity, anonymous.
  const trustBox = r.trusted
    ? `<div class="hint">Trusted device (identity verified) — no code to compare. It asks each time; change that under Settings → Trusted devices.</div>`
    : r.fingerprint
    ? `<label class="chk-trust"><input type="checkbox" id="req-trust"> Trust this device (skip the code next time)</label>
       <label class="chk-trust trust-mode"><span>Then:</span>
         <select id="req-trust-mode">
           <option value="ask" selected>Ask before each transfer (recommended)</option>
           <option value="auto">Save its files automatically</option>
         </select></label>
       <div class="hint">Ask keeps a one-tap approval per file, so nothing lands without you seeing it. Auto is for your own devices.</div>
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
  if (state.storeManaged) return '';
  const u = state.update;
  if (!u?.available) return '';
  const kind = u.prerelease ? 'Beta update available' : 'Update available';
  return `<div class="update-bar"><span>${kind} — <strong>v${escapeHtml(u.latest)}</strong></span><button class="btn-mini" id="apply-update">Install</button></div>`;
}
// ADR-034: the second factor. The transfer has already been answered; this only
// decides whether the device becomes trusted.
function trustCodeOverlay(p: TrustPrompt): string {
  const how = p.factor === 'totp'
    ? 'Enter the 6-digit code from your authenticator app.'
    : `We emailed a 6-digit code to <b>${escapeHtml(p.sentTo)}</b>.`;
  const safety = p.safetyNumber
    ? `<div class="hint">Safety number of this device: <b>${escapeHtml(p.safetyNumber)}</b>. Compare it with its own screen (Settings, or <code>s2u lan id</code>). If it differs, choose Not now — someone may be impersonating it.</div>`
    : '';
  const what = p.mode === 'auto' ? 'its files will be saved automatically' : 'it will still ask before each transfer, without a code';
  return `<div class="overlay"><div class="overlay-card">
    <div class="overlay-title">Confirm trusting ${escapeHtml(p.name || p.fingerprint.slice(0, 10))}</div>
    <div class="overlay-body">${safety}<div class="hint">${how}</div><div class="hint">Once confirmed, ${what}.</div>
      <input id="trust-code" class="code-input" inputmode="numeric" autocomplete="one-time-code" maxlength="7" placeholder="123 456" ${p.busy ? 'disabled' : ''} />
      <div class="hint trust-error" style="min-height:1.2em">${escapeHtml(p.error)}</div>
    </div>
    <div class="overlay-actions"><button class="btn-hdr" id="trust-cancel" ${p.busy ? 'disabled' : ''}>Not now</button><button class="btn-accept" id="trust-verify" ${p.busy ? 'disabled' : ''}>${p.busy ? 'Checking…' : 'Confirm'}</button></div>
  </div></div>`;
}

// startTrust opens the challenge and shows the code prompt. Errors (not signed
// in, API token, offline) surface as a toast; nothing is trusted.
async function startTrust(fingerprint: string, name: string, mode: string) {
  try {
    const ch = await backend().TrustDevice(fingerprint, name, mode);
    state.trustPrompt = { ...ch, fingerprint, name, mode, error: '', busy: false };
    render();
    root.querySelector<HTMLInputElement>('#trust-code')?.focus();
  } catch (e) { toast('Cannot trust this device: ' + String(e)); }
}

async function submitTrustCode() {
  const p = state.trustPrompt; if (!p || p.busy) return;
  const code = (root.querySelector<HTMLInputElement>('#trust-code')?.value || '').replace(/\s+/g, '');
  if (!code) { p.error = 'Enter the code.'; render(); return; }
  p.busy = true; p.error = ''; render();
  try {
    await backend().VerifyTrust(p.challengeId, code);
    state.trustPrompt = null; render();
    toast(`Trusted ${p.name || 'device'}${p.mode === 'auto' ? ' — files save automatically' : ''}`);
    loadTrusted();
  } catch (e) {
    p.busy = false; p.error = String(e).replace(/^Error:\s*/, ''); render();
    root.querySelector<HTMLInputElement>('#trust-code')?.focus();
  }
}

function loginProgress(): string {
  if (state.loginPhase === 'waiting') {
    const info = state.loginInfo;
    if (!info) {
      // Show feedback immediately, before BeginLogin returns, so a slow or
      // unreachable sign-in server never looks like "nothing happened".
      return `<div class="banner">Starting sign-in… <span class="hint">opening your browser</span></div>`;
    }
    return `<div class="banner">Approve this device in your browser${info.userCode ? ` — code <code>${escapeHtml(info.userCode)}</code>` : ''}. <span class="hint">Waiting…</span>${info.verificationUrl ? `<button class="btn-mini" id="reopen-login">Reopen page</button>` : ''}</div>`;
  }
  if (state.loginPhase === 'error' && state.loginError) return `<div class="banner-err">${escapeHtml(state.loginError)}</div>`;
  return '';
}
function filesBlock(): string {
  if (!state.paths.length) {
    return `<div class="canvas" id="canvas"><div class="canvas-ico">⬍</div><div class="canvas-title">Choose files, drop them here, or paste with Ctrl+V</div><div class="canvas-sub">Screenshots, images and text work too.</div><button class="btn-choose" id="pick-files">Choose files</button>${clipChip()}</div>`;
  }
  return `<div class="files">${state.paths.map((p, i) => `<div class="file-chip" title="${escapeHtml(p)}">${escapeHtml(basename(p))}<button class="chip-x" data-i="${i}" title="Remove">×</button></div>`).join('')}<button class="files-add" id="pick-files">＋ Add files</button></div>`;
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
    <summary class="settings-summary" aria-hidden="true" tabindex="-1">Settings</summary>
    <div class="settings-body">
      <label class="setting-row"><input type="checkbox" id="set-discoverable" ${s.discoverable ? 'checked' : ''} /><span class="setting-label">Discoverable on local network<span class="setting-help">Nearby devices can send you files — trusted ones land automatically, others ask.</span></span></label>
      <div class="chk2">Broadcast scan interval <select id="scan-interval">${[15, 30, 60, 120, 0].map((v) => `<option value="${v}" ${state.scanInterval === v ? 'selected' : ''}>${v === 0 ? 'manual only' : 'every ' + v + 's'}</option>`).join('')}</select></div>
      <label class="setting-row"><input type="checkbox" id="set-shell" ${s.shellInstalled ? 'checked' : ''} /><span class="setting-label">Right-click Share menu</span></label>
      <label class="setting-row${s.canReceive ? '' : ' is-disabled'}"><input type="checkbox" id="set-autostart" ${s.autostartEnabled ? 'checked' : ''} ${s.canReceive ? '' : 'disabled'} /><span class="setting-label">Start Share2Us at login<span class="setting-help">So it is already running to receive files. Being found by other devices also needs “Discoverable on local network” above.</span></span></label>
      <label class="setting-row${state.storeManaged ? ' is-disabled' : ''}"><input type="checkbox" id="set-beta" ${state.updateChannel === 'beta' ? 'checked' : ''} ${state.storeManaged ? 'disabled' : ''} /><span class="setting-label">Get beta builds<span class="setting-help">${state.storeManaged ? 'The Microsoft Store manages updates for this install.' : 'Pre-release builds before they reach everyone. Also switches the s2u command line on this machine.'}</span></span></label>
      ${trustedBlock()}
      ${state.activity.length ? `<button class="btn-mini" id="clear-activity">Clear activity log</button>` : ''}
      <div class="setting-row" style="justify-content:space-between">
        <span class="setting-label">Received files${state.incomingFolder ? '' : ' · you are asked each time'}<span class="setting-help">${state.incomingFolder ? escapeHtml(state.incomingFolder) : 'Nothing is saved anywhere until you choose.'}</span></span>
        <span style="display:flex;gap:6px;flex:none">
          <button class="btn-hdr" id="change-folder">Change</button>
          ${state.incomingFolder ? `<button class="btn-hdr" id="clear-folder">Ask each time</button>` : ''}
        </span>
      </div>
      ${s.loggedIn ? `<div class="setting-row" style="justify-content:space-between"><span class="setting-label">Signed in as ${escapeHtml(s.email)}</span><button class="btn-hdr" id="logout-btn">Log out</button></div>` : ''}
    </div>
  </details>`;
}
function trustedBlock(): string {
  if (!state.trusted.length) return '';
  return `<div class="trusted-list"><div class="setting-label">Trusted devices <span class="hint">on your account; trusting or switching to "auto" asks for a verification code</span></div>${state.trusted.map((d) => `<div class="trusted-row"><span class="trusted-name" title="${escapeHtml(d.fingerprint)}">${escapeHtml(d.name || d.fingerprint.slice(0, 10))}</span><select class="trusted-mode" data-fp="${escapeHtml(d.fingerprint)}" title="What happens when this device sends a file"><option value="ask" ${d.mode === 'auto' ? '' : 'selected'}>Ask each time</option><option value="auto" ${d.mode === 'auto' ? 'selected' : ''}>Save automatically</option></select><button class="btn-mini trusted-revoke" data-fp="${escapeHtml(d.fingerprint)}">Revoke</button></div>`).join('')}</div>`;
}

// ---- Primary button (share modal) ------------------------------------------

function primaryLabel(): string {
  const n = state.paths.length;
  const files = `${n} file${n === 1 ? '' : 's'}`;
  if (state.dest === 'broadcast') return n ? `Broadcast ${files} to everyone nearby` : 'Start broadcast';
  if (state.dest === 'nearby') {
    // Only ever names a device the user actually selected. A typed address counts
    // as a choice too; what it will not do is name whichever device happened to
    // answer the scan first.
    const target = state.picked?.name || state.netDest.trim();
    return n && target ? `Send ${files} to ${target}` : `Send ${files}`;
  }
  return n ? `Create link for ${files}` : 'Create link';
}

function canPrimary(): boolean {
  if (!state.paths.length) return false;
  if (state.dest === 'public' || state.dest === 'private') return !!state.status?.loggedIn;
  // A direct send needs a destination. The button stays present and disabled
  // rather than appearing once one is chosen, so nothing reflows.
  if (state.dest === 'nearby') return !!(state.picked || state.netDest.trim());
  return true;
}
function footerReason(): string {
  if (!state.paths.length) return 'Add a file above to share.';
  if ((state.dest === 'public' || state.dest === 'private') && !state.status?.loggedIn) return 'Login to share to the cloud.';
  if (state.dest === 'nearby' && !state.picked && !state.netDest.trim()) return 'Pick a device above, or enter its address.';
  return '';
}
async function onPrimary() {
  if (state.dest === 'broadcast') return startBroadcast();
  if (state.dest === 'nearby') {
    const typed = (root.querySelector<HTMLInputElement>('#net-dest')?.value || '').trim();
    const dest = typed || state.picked?.dest || '';
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
  state.picked = null;
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
  const kind = state.dest;
  const name = basename(state.paths[0] || '');
  const btn = root.querySelector<HTMLButtonElement>('#primary-btn')!;
  btn.disabled = true; btn.textContent = 'Sharing…';
  try {
    const out = await backend().Share(req);
    const failed = out.find((o) => !o.ok);
    if (failed) { btn.disabled = false; btn.textContent = primaryLabel(); toast(failed.error || 'Share failed'); return; }
    const link = out.find((o) => o.ok && o.link)?.link;
    state.view = 'home'; state.paths = [];
    await refreshActivity();
    // Keep the link visible + on the clipboard instead of silently closing.
    if (link) { copy(link); state.shareResult = { name, link, kind }; }
    else { toast('Shared'); }
    render();
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
    await refreshActivity(); loadTrusted();
    if (wantTrust && res.fingerprint) startTrust(res.fingerprint, p.name, 'ask');
  } catch (e) { toast('Download failed: ' + String(e)); }
}

async function stopBroadcast() {
  try { await backend().StopBroadcast(); } catch { /* ignore */ }
  state.bc = null; state.view = 'home'; render();
}

// ---- Data refresh ----------------------------------------------------------

async function refreshIncoming() { try { state.incoming = (await backend().IncomingList()) || []; } catch { /* */ } }
async function refreshActivity() { try { state.activity = (await backend().ActivityLog()) || []; } catch { /* */ } }
async function loadTrusted() { try { state.trusted = (await backend().ListTrusted()) || []; render(); } catch { /* */ } }
let scanTimer = 0;
async function findNearby(quiet = false) {
  if (state.browsing) return;
  state.browsing = true; if (!quiet) render();
  try { state.peers = (await backend().LanBrowse()) || []; } catch { /* keep last list */ }
  state.browsing = false;
  // Repaint where the result is actually on screen. Home always; the share modal
  // too when its device list is showing, because that list arrives AFTER the
  // modal does when the app is opened straight into Share from the file manager
  // — without this the user is asked to pick a device from an empty list.
  //
  // Never while the address box has focus: a repaint mid-keystroke would take the
  // caret away, which is exactly the "yanked out from under you" this guard was
  // written to prevent.
  const typing = root.querySelector('#net-dest') === document.activeElement;
  if (state.view === 'home' || (state.view === 'share' && state.dest === 'nearby' && !typing)) render();
}

// startScanTimer re-scans for nearby devices/broadcasts every scanInterval
// seconds (0 = manual refresh only). The tick is quiet — no scanning flash.
function startScanTimer() {
  if (scanTimer) { clearInterval(scanTimer); scanTimer = 0; }
  const sec = state.scanInterval;
  if (!sec || sec <= 0) return;
  scanTimer = window.setInterval(() => { if (state.view === 'home') findNearby(true); }, sec * 1000);
}
async function checkForUpdate() { if (state.storeManaged) return; try { const info = await backend().CheckUpdate(); if (info?.available) { state.update = info; render(); } } catch { /* */ } }
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
  root.querySelector('#pick-files')?.addEventListener('click', (e) => { e.stopPropagation(); pickFiles(); });
  root.querySelector('#canvas')?.addEventListener('click', pickFiles);
  root.querySelector('#share-back')?.addEventListener('click', () => { state.view = 'home'; render(); });
  root.querySelector('#nearby-find')?.addEventListener('click', () => findNearby());
  root.querySelector('#clip-add')?.addEventListener('click', addClipboard);
  root.querySelector('#primary-btn')?.addEventListener('click', onPrimary);
  root.querySelector('#bc-back')?.addEventListener('click', () => { state.view = 'home'; render(); });
  root.querySelectorAll('#bc-stop').forEach((b) => b.addEventListener('click', stopBroadcast));
  root.querySelector('#live-row')?.addEventListener('click', (e) => { if (!(e.target as HTMLElement).closest('#bc-stop')) { state.view = 'broadcast'; render(); } });
  on('.chip-x', 'click', (e) => { state.paths.splice(Number((e.currentTarget as HTMLElement).dataset.i), 1); render(); });
  // Picking a device selects it; the send happens from the primary button. The
  // old behaviour fired the transfer straight from this row, which left no
  // moment to notice you had picked the wrong machine.
  root.querySelectorAll<HTMLElement>('.pick-dev').forEach((el) =>
    el.addEventListener('click', (e) => {
      e.stopPropagation();
      const dest = el.dataset.dest || '';
      state.picked = state.picked?.dest === dest ? null : { dest, name: el.dataset.name || dest };
      render();
    }),
  );
  on('.send-to', 'click', (e) => sendTo((e.currentTarget as HTMLElement).dataset.dest || ''));
  on('.dl-btn', 'click', (e) => { const fp = (e.currentTarget as HTMLElement).dataset.fp; const p = state.peers.find((x) => x.isBroadcast && x.fingerprint === fp); if (p) { state.dl = p; render(); } });
  on('.dest-opt', 'click', (e) => { state.dest = (e.currentTarget as HTMLElement).dataset.destOpt as Dest; render(); });
  on('.dest-opt input, .dest-opt .send-to, .dest-opt .pick-dev, .dest-opt .mode, .dest-opt .addr-row', 'click', (e) => e.stopPropagation());
  on('.mode', 'click', (e) => { state.bcAccess = (e.currentTarget as HTMLElement).dataset.bcMode as any; render(); });
  const nd = root.querySelector<HTMLInputElement>('#net-dest');
  const ndUse = root.querySelector<HTMLButtonElement>('#net-dest-use');
  // Enable the action in place rather than re-rendering: a render on every
  // keystroke would take the focus out of the field being typed into.
  nd?.addEventListener('input', () => {
    state.netDest = nd.value;
    if (ndUse) ndUse.disabled = !nd.value.trim();
  });
  // A typed address had no visible way to submit it. It was only ever consumed
  // by the primary button at the far end of the form, so the field read as
  // inert. Enter and an explicit Use both confirm it as the destination.
  const useTyped = () => {
    const dest = (nd?.value || '').trim();
    if (!dest) return;
    state.netDest = dest;
    state.picked = { dest, name: dest };
    render();
  };
  ndUse?.addEventListener('click', (e) => { e.stopPropagation(); useTyped(); });
  nd?.addEventListener('keydown', (e) => { if ((e as KeyboardEvent).key === 'Enter') { e.preventDefault(); useTyped(); } });
  root.querySelector<HTMLDetailsElement>('.opt-card')?.addEventListener('toggle', (e) => (state.optionsOpen = (e.target as HTMLDetailsElement).open));
  // request approval overlay
  root.querySelector('#req-reject')?.addEventListener('click', () => respondRequest(false));
  root.querySelector('#req-accept')?.addEventListener('click', async () => {
    const r = state.requests[0];
    const wantTrust = !!(r && root.querySelector<HTMLInputElement>('#req-trust')?.checked && r.fingerprint);
    const mode = root.querySelector<HTMLSelectElement>('#req-trust-mode')?.value === 'auto' ? 'auto' : 'ask';
    // Answer the sender first (it is waiting), then collect the second factor.
    await respondRequest(true);
    if (wantTrust && r) startTrust(r.fingerprint, r.senderName || r.from, mode);
  });
  // download confirm overlay
  root.querySelector('#dl-cancel')?.addEventListener('click', () => { state.dl = null; render(); });
  root.querySelector('#dl-go')?.addEventListener('click', doDownload);
  // share-result overlay (persistent copyable link)
  root.querySelector('#share-done')?.addEventListener('click', () => { state.shareResult = null; render(); });
  root.querySelector('#share-copy')?.addEventListener('click', () => {
    const f = root.querySelector<HTMLInputElement>('#share-link');
    if (f) { f.select(); copy(f.value); toast('Link copied'); }
  });
  root.querySelector<HTMLInputElement>('#share-link')?.addEventListener('focus', (e) => (e.currentTarget as HTMLInputElement).select());
  on('.copy-link', 'click', (e) => { copy((e.currentTarget as HTMLElement).dataset.link || ''); toast('Link copied'); });
  on('#build-strip', 'click', () => { if (state.buildVersion) { copy(state.buildVersion); toast('Version copied'); } });
  // settings
  // One tap from the send flow, so the user never has to go hunting in Settings
  // for a thing they were just told they need.
  on('#dest-make-disc', 'click', async () => {
    try {
      await backend().SetDiscoverable(true);
      if (state.status) state.status.discoverable = true;
      toast('Discoverable — nearby devices can now see this one');
      render();
      findNearby();
    } catch (e) { toast(String(e)); }
  });
  // Settings is one click from the strip now, wherever the user is looking.
  // Full history is the portal's job; this window shows the last few.
  // Switching halves also moves the selection, so the picker is never showing a
  // destination that belongs to the other tab.
  root.querySelectorAll<HTMLElement>('[data-send-mode]').forEach((el) =>
    el.addEventListener('click', () => {
      const m = el.dataset.sendMode as 'device' | 'link';
      if (state.sendMode === m) return;
      state.sendMode = m;
      state.dest = m === 'device' ? 'nearby' : 'public';
      render();
    }),
  );
  // Save a waiting arrival. The native dialog cannot carry a "remember this"
  // checkbox, so the offer comes after the save, once, and only while no folder
  // is set — asking again every time would be nagging.
  root.querySelectorAll<HTMLElement>('.save-incoming').forEach((el) =>
    el.addEventListener('click', async () => {
      const id = el.dataset.id || '';
      try {
        const folder = await backend().SaveIncoming(id);
        if (!folder) return; // cancelled: the file is still waiting
        await refreshIncoming();
        if (!state.incomingFolder) {
          state.pendingRemember = folder;
        } else {
          toast('Saved');
        }
        render();
      } catch (e) { toast(String(e)); }
    }),
  );
  on('#remember-folder', 'click', async () => {
    const dir = state.pendingRemember;
    state.pendingRemember = '';
    try { await backend().SetIncomingFolder(dir); state.incomingFolder = dir; toast('Received files will be saved there'); }
    catch (e) { toast(String(e)); }
    render();
  });
  on('#remember-dismiss', 'click', () => { state.pendingRemember = ''; render(); });
  on('#change-folder', 'click', async () => {
    try {
      const dir = await backend().ChooseIncomingFolder();
      if (dir) { state.incomingFolder = dir; toast('Received files will be saved there'); render(); }
    } catch (e) { toast(String(e)); }
  });
  on('#clear-folder', 'click', async () => {
    try { await backend().SetIncomingFolder(''); state.incomingFolder = ''; toast('You will be asked each time'); render(); }
    catch (e) { toast(String(e)); }
  });
  on('#open-history', 'click', () => {
    const rt = (window as any).runtime;
    rt?.BrowserOpenURL?.('https://portal.share2.us/activity');
  });
  on('#open-settings', 'click', () => {
    const d = root.querySelector<HTMLDetailsElement>('details.settings');
    if (d) { d.open = true; d.scrollIntoView({ behavior: 'smooth', block: 'nearest' }); }
  });
  on('#shai-open', 'click', () => { state.shaiOpen = !state.shaiOpen; render(); });
  on('#shai-close', 'click', () => { state.shaiOpen = false; render(); });
  const disc = root.querySelector<HTMLInputElement>('#set-discoverable');
  disc?.addEventListener('change', async () => { try { await backend().SetDiscoverable(disc.checked); if (state.status) state.status.discoverable = disc.checked; if (!disc.checked) state.discCode = ''; render(); } catch { disc.checked = !disc.checked; } });
  const si = root.querySelector<HTMLSelectElement>('#scan-interval');
  si?.addEventListener('change', async () => { state.scanInterval = Number(si.value); try { await backend().SetScanInterval(state.scanInterval); } catch { /* */ } startScanTimer(); });
  wireToggle('set-shell', (o) => backend().SetShellIntegration(o));
  wireToggle('set-autostart', (o) => backend().SetAutostart(o));
  wireToggle('set-beta', async (o) => { await backend().SetUpdateChannel(o ? 'beta' : 'stable'); state.updateChannel = o ? 'beta' : 'stable'; state.update = null; render(); checkForUpdate(); });
  on('.trusted-revoke', 'click', async (e) => { try { await backend().UntrustDevice((e.currentTarget as HTMLElement).dataset.fp || ''); } catch (err) { toast(String(err)); } loadTrusted(); });
  on('.trusted-mode', 'change', async (e) => {
    const el = e.currentTarget as HTMLSelectElement; const fp = el.dataset.fp || '';
    if (el.value === 'auto') { const d = state.trusted.find((t) => t.fingerprint === fp); startTrust(fp, d?.name || '', 'auto'); loadTrusted(); return; } // widening: needs the code
    try { await backend().SetTrustMode(fp, 'ask'); } catch (err) { toast(String(err)); } loadTrusted();
  });
  root.querySelector('#trust-cancel')?.addEventListener('click', () => { state.trustPrompt = null; render(); });
  root.querySelector('#trust-verify')?.addEventListener('click', submitTrustCode);
  root.querySelector('#trust-code')?.addEventListener('keydown', (e) => { if ((e as KeyboardEvent).key === 'Enter') submitTrustCode(); });
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

function withTimeout<T>(p: Promise<T>, ms: number, msg: string): Promise<T> {
  return Promise.race([p, new Promise<T>((_, rej) => setTimeout(() => rej(new Error(msg)), ms))]);
}
async function signIn() {
  state.loginPhase = 'waiting'; state.loginError = ''; state.loginInfo = null; render();
  try {
    // Bound BeginLogin: if the sign-in server is unreachable this fails visibly
    // instead of hanging silently. (CompleteLogin is NOT bounded — it waits for
    // the user to approve in the browser, up to the backend's 10-minute window.)
    state.loginInfo = await withTimeout(backend().BeginLogin(), 20000, 'Could not reach the sign-in server. Check your connection and try again.');
    render();
    state.status = await backend().CompleteLogin();
    state.loginPhase = 'idle'; state.loginInfo = null; render();
  } catch (e) { state.loginPhase = 'error'; state.loginError = String(e).replace(/^Error:\s*/, ''); state.loginInfo = null; render(); }
}
async function logout() {
  let err: unknown = null;
  try { await backend().Logout(); } catch (e) { err = e; }
  try { state.status = await backend().Status(); } catch { /* */ }
  state.view = 'home'; render();
  if (err) toast('Logout failed: ' + String(err));
  else if (state.status?.loggedIn) toast('Could not clear the session — please try again');
  else toast('Signed out');
}

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

async function pickFiles() {
  try {
    const paths = await backend().PickFiles();
    if (paths && paths.length) { addPaths(paths); state.view = 'share'; render(); }
  } catch (e) { toast(String(e)); }
}

let listenersReady = false;
function setupListeners() {
  if (listenersReady) return; listenersReady = true;
  document.addEventListener('paste', onPaste);
  // Stop the WebView from navigating to (opening) a file dropped outside a Wails
  // drop target — that's what made drops "open like a web browser" and left the
  // app unreachable. Real file paths still arrive via 'files-dropped' below.
  ['dragenter', 'dragover', 'drop'].forEach((ev) =>
    window.addEventListener(ev, (e) => { e.preventDefault(); e.stopPropagation(); }, false));
  // Escape closes an open overlay, or backs a modal out to Home.
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    if (state.shareResult) { state.shareResult = null; render(); return; }
    if (state.dl) { state.dl = null; render(); return; }
    if (state.requests.length) { respondRequest(false); return; }
    if (state.view !== 'home') { state.view = 'home'; render(); }
  });
  const rt = (window as any).runtime;
  // THIS CALL IS WHAT MAKES DROPPING WORK AT ALL. Wails registers its own
  // dragover/dragleave/drop listeners only inside the FRONTEND runtime's
  // OnFileDrop; the Go-side runtime.OnFileDrop (app.go) merely subscribes to the
  // "wails:file-drop" event that those listeners cause to be emitted. With no
  // frontend call, nothing was ever listening, no drop was ever resolved, and
  // the Go callback could never fire — which is exactly what "drag and drop does
  // nothing, but Choose files works" looked like.
  // Passing true keeps Wails' drop-target gating, which our CSS now satisfies
  // (see --wails-drop-target in style.css).
  rt?.OnFileDrop?.((_x: number, _y: number, paths: string[]) => {
    addPaths(paths || []); state.view = 'share'; render();
  }, true);
  // Kept: the Go side emits this for the same drop. addPaths dedupes, so the two
  // paths cannot double-add a file.
  rt?.EventsOn?.('files-dropped', (paths: string[]) => { addPaths(paths || []); state.view = 'share'; render(); });
  rt?.EventsOn?.('lan-request', (r: any) => {
    if (!r?.id) return;
    state.requests.push({ id: String(r.id), from: String(r.from || ''), name: String(r.name || 'file'), size: Number(r.size) || 0, fingerprint: String(r.fingerprint || ''), senderName: String(r.senderName || ''), code: String(r.code || ''), action: String(r.action || 'send') });
    render();
  });
  rt?.EventsOn?.('lan-recv-done', () => { refreshActivity().then(render); });
  rt?.EventsOn?.('incoming-changed', () => { refreshIncoming().then(render); });
  rt?.EventsOn?.('lan-discoverable', (d: any) => { if (!d?.error) { state.discCode = String(d?.code || ''); state.discSafety = String(d?.safety || ''); render(); } });
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
