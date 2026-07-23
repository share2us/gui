package main

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/share2us/gui/internal/autostart"
	"github.com/share2us/gui/internal/clip"
	"github.com/share2us/gui/internal/core"
	"github.com/share2us/gui/internal/lan"
	"github.com/share2us/gui/internal/lanid"
	"github.com/share2us/gui/internal/receiver"
	"github.com/share2us/gui/internal/shell"
	"github.com/share2us/gui/internal/update"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound backend. Every exported method is callable from the
// frontend modal; the heavy lifting lives in internal/core so it stays testable
// off Windows.
type App struct {
	ctx context.Context

	// pending holds the file/folder paths passed on the command line by the
	// Explorer "Share" verb (share2us-windows.exe share "<path>" ...).
	pending []string

	mu     sync.Mutex
	client *core.Client // lazily loaded from the saved credential

	loginMu sync.Mutex
	login   *core.LoginSession // an in-progress device-code login

	lanMu   sync.Mutex
	lanRecv *lan.Receiver // an active one-shot local-network receiver, if any

	discMu       sync.Mutex
	discRecv     *lan.Receiver          // persistent discoverable serve loop, if on
	discoverable bool                   // whether we are advertising + serving
	reqs         map[string]chan bool   // pending approval prompts, by id
	reqSeq       uint64
	pendingByIP  map[string]int         // in-flight approval prompts per source IP
	pendingTotal int                    // in-flight approval prompts overall
	cooldown     map[string]time.Time   // per-IP auto-reject-until after a decline
}

// Approval anti-spam limits (a peer must not be able to flood the receiver).
const (
	maxPendingPerIP   = 1
	maxPendingTotal   = 3
	declineCooldown   = 30 * time.Second
	approvalWaitLimit = 60 * time.Second
)

// NewApp constructs the app with the paths selected in Explorer (may be empty).
func NewApp(pending []string) *App {
	return &App{pending: pending}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Native file drop: forward dropped file paths to the modal.
	wailsRuntime.OnFileDrop(ctx, func(_, _ int, paths []string) {
		if len(paths) > 0 {
			wailsRuntime.EventsEmit(ctx, "files-dropped", paths)
		}
	})
	go cleanupOldTemps() // remove staged paste/update temp dirs left by prior runs
}

// cleanupOldTemps best-effort removes leftover Share2Us temp dirs (staged pastes
// and downloaded updates) from previous runs. It only ever touches our own
// prefixed dirs, and the current run's files are created after startup, so they
// are never affected.
func cleanupOldTemps() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && (strings.HasPrefix(e.Name(), "share2us-paste-") || strings.HasPrefix(e.Name(), "share2us-update-")) {
			_ = os.RemoveAll(filepath.Join(os.TempDir(), e.Name()))
		}
	}
}

// maxPasteBytes caps clipboard content written to a temp file.
const maxPasteBytes = 64 << 20 // 64 MiB

// AddPasted writes base64-encoded clipboard content (an image or text pasted into
// the window) to a uniquely-named temp file and returns its path, so it can be
// shared like any other file. ext is the extension without the dot (png/txt/md).
func (a *App) AddPasted(ext, dataB64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return "", errors.New("could not read clipboard content")
	}
	if len(raw) == 0 {
		return "", errors.New("clipboard is empty")
	}
	if len(raw) > maxPasteBytes {
		return "", errors.New("clipboard content is too large")
	}
	return writeTempShare(raw, ext)
}

// ClipboardSuggestion reports shareable content currently on the OS clipboard so
// the UI can offer a one-click chip. Kind is "none" when there is nothing
// shareable (or on platforms without a backend clipboard read).
func (a *App) ClipboardSuggestion() clip.Suggestion {
	s, err := clip.Peek()
	if err != nil {
		return clip.Suggestion{Kind: "none"}
	}
	return s
}

// AddClipboard stages the current clipboard content of kind ("image"|"text") to
// a temp file and returns its path, so a copied screenshot or snippet can be
// shared with one click.
func (a *App) AddClipboard(kind string) (string, error) {
	raw, ext, err := clip.Read(kind)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", errors.New("clipboard is empty")
	}
	if len(raw) > maxPasteBytes {
		return "", errors.New("clipboard content is too large")
	}
	return writeTempShare(raw, ext)
}

// writeTempShare writes raw to a uniquely-named temp file (ext without the dot)
// and returns its path. Shared by the browser paste path and the clipboard read.
func writeTempShare(raw []byte, ext string) (string, error) {
	dir, err := os.MkdirTemp("", "share2us-paste-")
	if err != nil {
		return "", err
	}
	name := "pasted-" + time.Now().Format("20060102-150405") + sanitizeExt(ext)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// sanitizeExt returns a safe ".ext" (default .bin) from a caller-supplied hint.
func sanitizeExt(ext string) string {
	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "txt", "md", "csv", "json", "log":
		return "." + ext
	default:
		return ".bin"
	}
}

// clientOrErr lazily loads the authenticated client from the saved login.
func (a *App) clientOrErr() (*core.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client != nil {
		return a.client, nil
	}
	c, err := core.Load()
	if err != nil {
		return nil, err
	}
	a.client = c
	return c, nil
}

// Status is what the modal asks for first: what can this session do right now?
type Status struct {
	LoggedIn         bool   `json:"loggedIn"`
	Email            string `json:"email"`
	IsAPIToken       bool   `json:"isApiToken"`
	CanReceive       bool   `json:"canReceive"`
	ShellInstalled   bool   `json:"shellInstalled"`
	AutostartEnabled bool   `json:"autostartEnabled"`
	Discoverable     bool   `json:"discoverable"`
}

// Status reports login and capability state for the UI.
func (a *App) Status() Status {
	s := Status{
		ShellInstalled:   shell.Installed(),
		AutostartEnabled: autostart.Enabled(),
	}
	a.discMu.Lock()
	s.Discoverable = a.discoverable
	a.discMu.Unlock()
	c, err := a.clientOrErr()
	if err != nil {
		return s
	}
	s.LoggedIn = true
	s.Email = c.Email()
	s.IsAPIToken = c.IsAPIToken()
	s.CanReceive = c.HasDeviceKey()
	return s
}

// SetAutostart enables/disables launching the background receiver at login.
func (a *App) SetAutostart(on bool) error {
	if on {
		return autostart.Enable("")
	}
	return autostart.Disable()
}

// SetShellIntegration adds/removes the file-manager right-click entry.
func (a *App) SetShellIntegration(on bool) error {
	if on {
		return shell.Install("")
	}
	return shell.Uninstall()
}

// PendingPaths returns the paths Explorer passed to the Share verb.
func (a *App) PendingPaths() []string { return a.pending }

// ListDevices returns the account's own devices for the "send to device" picker.
func (a *App) ListDevices() ([]core.Device, error) {
	c, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	return c.Devices(a.ctx)
}

// ShareRequest is the modal's submit payload. Target selects the destination:
// "public" | "private" | "device" | "contact".
type ShareRequest struct {
	Paths        []string `json:"paths"`
	Target       string   `json:"target"`
	Recipients   []string `json:"recipients"`
	Email        string   `json:"email"`
	DeviceID     string   `json:"deviceId"`
	DevicePub    string   `json:"devicePub"`
	Password     string   `json:"password"`
	OneTime      bool     `json:"oneTime"`
	Expires      string   `json:"expires"`
	Keep         bool     `json:"keep"`
	AllowReshare bool     `json:"allowReshare"`
	Note         string   `json:"note"`
}

// ShareOutcome is one path's result (the modal renders a row per path).
type ShareOutcome struct {
	Path     string `json:"path"`
	OK       bool   `json:"ok"`
	Link     string `json:"link,omitempty"`
	PublicID string `json:"publicId,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Share dispatches each path to the chosen destination and returns per-path
// outcomes. Errors are captured per row so one failure does not abort the rest.
func (a *App) Share(req ShareRequest) []ShareOutcome {
	c, err := a.clientOrErr()
	if err != nil {
		return failAll(req.Paths, err)
	}
	outcomes := make([]ShareOutcome, 0, len(req.Paths))
	for _, p := range req.Paths {
		outcomes = append(outcomes, a.shareOne(c, req, p))
	}
	return outcomes
}

func (a *App) shareOne(c *core.Client, req ShareRequest, path string) ShareOutcome {
	var (
		res core.Result
		err error
	)
	switch req.Target {
	case "public", "private":
		vis := core.Public
		if req.Target == "private" {
			vis = core.Private
		}
		var allow *bool
		if vis == core.Private && req.AllowReshare {
			allow = &req.AllowReshare
		}
		res, err = c.ShareLink(a.ctx, core.LinkRequest{
			Path:         path,
			Visibility:   vis,
			Recipients:   req.Recipients,
			Password:     req.Password,
			OneTime:      req.OneTime,
			Expires:      req.Expires,
			Keep:         req.Keep,
			AllowReshare: allow,
			Note:         req.Note,
		})
	case "device":
		res, err = c.SendToDevice(a.ctx, path, req.DeviceID, req.DevicePub)
	case "contact":
		res, err = c.SendToContact(a.ctx, path, req.Email)
	default:
		return ShareOutcome{Path: path, Error: "unknown target: " + req.Target}
	}
	if err != nil {
		return ShareOutcome{Path: path, Error: err.Error()}
	}
	return ShareOutcome{Path: path, OK: true, Link: res.Link, PublicID: res.PublicID}
}

// ---- Local network (LAN, account-free / guest) ------------------------------

// LanSend streams each path directly to a nearby receiver over the local network
// (no account, end-to-end encrypted). dest is the receiver's code (an s2u://
// pairing string) or a plain host / host:port; password is used only when the
// code does not already carry one. Progress is emitted as "lan-send-progress".
func (a *App) LanSend(paths []string, dest, password string) []ShareOutcome {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return failAll(paths, errors.New("enter the receiver's code or address"))
	}
	out := make([]ShareOutcome, 0, len(paths))
	for _, p := range paths {
		path := p
		err := lan.SendOne(a.ctx, path, dest, password, func(sent, total int64) {
			wailsRuntime.EventsEmit(a.ctx, "lan-send-progress", map[string]any{
				"path": path, "sent": sent, "total": total,
			})
		})
		if err != nil {
			out = append(out, ShareOutcome{Path: path, Error: err.Error()})
		} else {
			out = append(out, ShareOutcome{Path: path, OK: true})
		}
	}
	return out
}

// LanStartReceive opens a background receiver and returns the details a sender
// needs (address, passphrase, one-paste code). It emits "lan-recv-progress" as
// bytes arrive and "lan-recv-done" ({name,path,bytes,from} or {error}) when a
// file lands or the receiver stops. Receiving lands files in Downloads.
func (a *App) LanStartReceive() (lan.Listen, error) {
	a.lanMu.Lock()
	if a.lanRecv != nil {
		a.lanRecv.Stop()
	}
	a.lanMu.Unlock()

	ready := make(chan lan.Listen, 1)
	errc := make(chan error, 1)
	r := lan.StartReceive(a.ctx, receiver.DownloadsDir(),
		func(l lan.Listen) {
			select {
			case ready <- l:
			default:
			}
		},
		func(rec, total int64) {
			wailsRuntime.EventsEmit(a.ctx, "lan-recv-progress", map[string]any{"received": rec, "total": total})
		},
		func(res *lan.Result, err error) {
			if err != nil {
				select {
				case errc <- err: // surfaces a pre-listen failure to the caller
				default:
				}
				wailsRuntime.EventsEmit(a.ctx, "lan-recv-done", map[string]any{"error": err.Error()})
				return
			}
			wailsRuntime.EventsEmit(a.ctx, "lan-recv-done", map[string]any{
				"name": res.Name, "path": res.Path, "bytes": res.Bytes, "from": res.From,
			})
			_ = beeep.Notify("Share2Us", "Received "+res.Name+" from "+res.From, "")
		})
	a.lanMu.Lock()
	a.lanRecv = r
	a.lanMu.Unlock()

	select {
	case l := <-ready:
		return l, nil
	case e := <-errc:
		return lan.Listen{}, e
	case <-time.After(8 * time.Second):
		r.Stop()
		return lan.Listen{}, errors.New("could not start the local receiver (is the port free?)")
	}
}

// LanStopReceive cancels an active local-network receiver.
func (a *App) LanStopReceive() {
	a.lanMu.Lock()
	r := a.lanRecv
	a.lanRecv = nil
	a.lanMu.Unlock()
	r.Stop()
}

// LanBrowse lists nearby Share2Us devices that are currently discoverable, for
// the "nearby devices" picker in the Send flow.
func (a *App) LanBrowse() ([]lan.Peer, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
	defer cancel()
	return lan.Browse(ctx, 1500*time.Millisecond)
}

// SetDiscoverable turns this device's discoverable receiver on or off. While on,
// the device advertises on the local network and every incoming transfer raises
// a "lan-request" approval prompt (answered by RespondLanRequest); accepted files
// land in Downloads with a toast. Being discoverable is opt-in — off by default.
func (a *App) SetDiscoverable(on bool) error {
	a.discMu.Lock()
	defer a.discMu.Unlock()
	if !on {
		if a.discRecv != nil {
			a.discRecv.Stop()
			a.discRecv = nil
		}
		a.discoverable = false
		return nil
	}
	if a.discRecv != nil {
		return nil // already discoverable
	}
	if a.reqs == nil {
		a.reqs = make(map[string]chan bool)
	}
	name, _ := os.Hostname()
	if name == "" {
		name = "Share2Us"
	}
	a.discRecv = lan.Serve(a.ctx, name, receiver.DownloadsDir(),
		func(l lan.Listen) {
			wailsRuntime.EventsEmit(a.ctx, "lan-discoverable", map[string]any{"address": l.Address, "name": name, "code": l.Code})
		},
		a.approveRequest,
		func(res lan.Result) {
			wailsRuntime.EventsEmit(a.ctx, "lan-recv-done", map[string]any{
				"name": res.Name, "path": res.Path, "bytes": res.Bytes, "from": res.From,
			})
			_ = beeep.Notify("Share2Us", "Received "+res.Name+" from "+res.From, "")
		},
		func(err error) {
			wailsRuntime.EventsEmit(a.ctx, "lan-discoverable", map[string]any{"error": err.Error()})
		})
	a.discoverable = true
	return nil
}

// approveRequest decides an inbound transfer. A device the receiver has trusted
// (by its verified key fingerprint) bypasses the verify code and the anti-spam
// caps: "auto" trust lands silently, "ask" trust still prompts (labelled
// trusted). An untrusted / anonymous sender is subject to the anti-spam limits
// (per-IP cap, global cap, post-decline cooldown) and then the normal prompt,
// which also offers "Accept & trust". Runs concurrently (one goroutine per
// inbound connection).
func (a *App) approveRequest(r lan.Request) bool {
	if r.Fingerprint != "" {
		if td, ok := lanid.Lookup(r.Fingerprint); ok {
			if td.Mode == "auto" {
				return true // trusted + auto: lands silently; OnReceived toasts + logs
			}
			return a.promptApproval(r, true) // trusted + ask: prompt, no caps
		}
	}

	// Untrusted / anonymous: enforce anti-spam limits before prompting.
	a.discMu.Lock()
	if a.pendingByIP == nil {
		a.pendingByIP = make(map[string]int)
		a.cooldown = make(map[string]time.Time)
	}
	if until, ok := a.cooldown[r.From]; ok {
		if time.Now().Before(until) {
			a.discMu.Unlock()
			return false
		}
		delete(a.cooldown, r.From)
	}
	if a.pendingByIP[r.From] >= maxPendingPerIP || a.pendingTotal >= maxPendingTotal {
		a.discMu.Unlock()
		return false
	}
	a.pendingByIP[r.From]++
	a.pendingTotal++
	a.discMu.Unlock()

	ok := a.promptApproval(r, false)

	a.discMu.Lock()
	if a.pendingByIP[r.From] > 0 {
		a.pendingByIP[r.From]--
		if a.pendingByIP[r.From] == 0 {
			delete(a.pendingByIP, r.From)
		}
	}
	if a.pendingTotal > 0 {
		a.pendingTotal--
	}
	if !ok {
		a.cooldown[r.From] = time.Now().Add(declineCooldown) // back off a declining/ignored peer
	}
	a.discMu.Unlock()
	return ok
}

// promptApproval raises a "lan-request" prompt and blocks until RespondLanRequest
// (or a timeout / shutdown). trusted marks the prompt as coming from an
// already-trusted device in the UI.
func (a *App) promptApproval(r lan.Request, trusted bool) bool {
	a.discMu.Lock()
	if a.reqs == nil {
		a.reqs = make(map[string]chan bool)
	}
	a.reqSeq++
	id := "req" + strconv.FormatUint(a.reqSeq, 10)
	ch := make(chan bool, 1)
	a.reqs[id] = ch
	a.discMu.Unlock()

	wailsRuntime.EventsEmit(a.ctx, "lan-request", map[string]any{
		"id": id, "from": r.From, "name": r.Name, "size": r.Size,
		"fingerprint": r.Fingerprint, "senderName": r.SenderName, "code": r.Code, "trusted": trusted,
	})

	ok := false
	select {
	case ok = <-ch:
	case <-time.After(approvalWaitLimit):
	case <-a.ctx.Done():
	}

	a.discMu.Lock()
	delete(a.reqs, id)
	a.discMu.Unlock()
	return ok
}

// TrustDevice adds a sender (by verified key fingerprint) to the trusted list
// with a mode ("ask" or "auto"). Called from the approval prompt's
// "Accept & trust".
func (a *App) TrustDevice(fingerprint, name, mode string) error {
	return lanid.Trust(fingerprint, name, mode)
}

// UntrustDevice revokes trust for a device.
func (a *App) UntrustDevice(fingerprint string) error {
	return lanid.Untrust(fingerprint)
}

// ListTrusted returns the trusted devices for the Settings management list.
func (a *App) ListTrusted() []lanid.TrustedDevice {
	return lanid.List()
}

// RespondLanRequest answers a pending "lan-request" prompt (accept or reject).
func (a *App) RespondLanRequest(id string, accept bool) {
	a.discMu.Lock()
	ch := a.reqs[id]
	delete(a.reqs, id)
	a.discMu.Unlock()
	if ch != nil {
		ch <- accept
	}
}

// LoginInfo is returned to the modal so it can show the code / verification page.
type LoginInfo struct {
	UserCode        string `json:"userCode"`
	VerificationURL string `json:"verificationUrl"`
	VerificationURI string `json:"verificationUri"`
}

// BeginLogin starts the device-code flow and opens the verification page in the
// user's browser. The modal then calls CompleteLogin to wait for approval.
func (a *App) BeginLogin() (LoginInfo, error) {
	sess, err := core.StartLogin(a.ctx, "")
	if err != nil {
		return LoginInfo{}, err
	}
	a.loginMu.Lock()
	a.login = sess
	a.loginMu.Unlock()
	if sess.VerificationURL != "" {
		wailsRuntime.BrowserOpenURL(a.ctx, sess.VerificationURL)
	}
	return LoginInfo{
		UserCode:        sess.UserCode,
		VerificationURL: sess.VerificationURL,
		VerificationURI: sess.VerificationURI,
	}, nil
}

// CompleteLogin blocks until the pending login is approved (or times out), saves
// the credential, and returns the refreshed Status.
func (a *App) CompleteLogin() (Status, error) {
	a.loginMu.Lock()
	sess := a.login
	a.loginMu.Unlock()
	if sess == nil {
		return Status{}, errors.New("no login in progress")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()
	c, err := sess.Wait(ctx)
	if err != nil {
		return Status{}, err
	}
	a.mu.Lock()
	a.client = c
	a.mu.Unlock()
	a.loginMu.Lock()
	a.login = nil
	a.loginMu.Unlock()
	return a.Status(), nil
}

// DeviceAccess is one of the caller's devices annotated with whether a given
// contact may target it (approvals-mode exposure).
type DeviceAccess struct {
	SessionID string `json:"sessionId"`
	Label     string `json:"label"`
	Current   bool   `json:"current"`
	HasKey    bool   `json:"hasKey"`
	Exposed   bool   `json:"exposed"`
}

// DeviceAccessForContact returns the caller's own devices with each device's
// exposure state for the given contact email (for the trust screen).
func (a *App) DeviceAccessForContact(email string) ([]DeviceAccess, error) {
	c, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	devices, err := c.Devices(a.ctx)
	if err != nil {
		return nil, err
	}
	exposed, err := c.ExposedDevices(a.ctx, email)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(exposed))
	for _, id := range exposed {
		set[id] = true
	}
	out := make([]DeviceAccess, 0, len(devices))
	for _, d := range devices {
		out = append(out, DeviceAccess{
			SessionID: d.SessionID,
			Label:     d.Label,
			Current:   d.Current,
			HasKey:    d.HasKey,
			Exposed:   set[d.SessionID],
		})
	}
	return out, nil
}

// SetDeviceAccess exposes (on) or revokes (off) one of the caller's devices for a
// contact.
func (a *App) SetDeviceAccess(email, sessionID string, exposed bool) error {
	c, err := a.clientOrErr()
	if err != nil {
		return err
	}
	if exposed {
		return c.ExposeDevice(a.ctx, email, sessionID)
	}
	return c.UnexposeDevice(a.ctx, email, sessionID)
}

// CheckUpdate reports whether a newer Share2Us release is available for this OS.
func (a *App) CheckUpdate() update.Info {
	info, err := update.Check(a.ctx, buildVersion)
	if err != nil {
		return update.Info{Current: buildVersion}
	}
	return info
}

// ApplyUpdate downloads and launches the update. On Windows it runs the installer
// and quits so it can replace the running app; elsewhere it opens the release page
// (self-replacing a running GUI is unreliable cross-platform).
func (a *App) ApplyUpdate() error {
	info, err := update.Check(a.ctx, buildVersion)
	if err != nil {
		return err
	}
	if !info.Available {
		return nil
	}
	if runtime.GOOS == "windows" && info.AssetURL != "" {
		path, err := downloadTemp(a.ctx, info.AssetURL, info.AssetName)
		if err != nil {
			return err
		}
		// Never execute the downloaded installer unless it carries a Valid
		// Authenticode signature from Share2.us (fail-closed). This is the last
		// line of defence if the download were somehow tampered.
		if err := update.VerifySignature(path); err != nil {
			return err
		}
		if err := exec.Command(path).Start(); err != nil {
			return err
		}
		wailsRuntime.Quit(a.ctx) // let the installer replace the running app
		return nil
	}
	if info.Page != "" {
		wailsRuntime.BrowserOpenURL(a.ctx, info.Page)
	}
	return nil
}

// allowedUpdateHost restricts update downloads (and every redirect hop) to
// GitHub hosts, so a tampered release URL or a redirect can never point the
// auto-updater at an arbitrary server.
func allowedUpdateHost(h string) bool {
	h = strings.ToLower(h)
	return h == "github.com" || h == "api.github.com" || h == "githubusercontent.com" ||
		strings.HasSuffix(h, ".githubusercontent.com") || strings.HasSuffix(h, ".github.com")
}

// downloadTemp fetches rawURL into a uniquely-named temp file and returns its
// path. It requires HTTPS + a GitHub host for the URL and for every redirect hop
// (no scheme downgrade, no off-host redirect).
func downloadTemp(ctx context.Context, rawURL, name string) (string, error) {
	if u, perr := neturl.Parse(rawURL); perr != nil || u.Scheme != "https" || !allowedUpdateHost(u.Hostname()) {
		return "", errors.New("refusing to download the update from an unexpected URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if r.URL.Scheme != "https" {
				return errors.New("refusing an insecure (non-HTTPS) update redirect")
			}
			if !allowedUpdateHost(r.URL.Hostname()) {
				return errors.New("refusing an update redirect to an unexpected host")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("download failed")
	}
	dir, err := os.MkdirTemp("", "share2us-update-")
	if err != nil {
		return "", err
	}
	if name == "" {
		name = "Share2Us-update"
	}
	path := filepath.Join(dir, filepath.Base(name))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return path, nil
}

// Logout deletes the saved login (shared with the CLI) and clears the cached
// client so the modal drops back to the signed-out state.
func (a *App) Logout() error {
	err := core.Logout()
	a.mu.Lock()
	a.client = nil
	a.mu.Unlock()
	return err
}

// InstallShell registers the Explorer right-click integration (Windows only).
func (a *App) InstallShell() error { return shell.Install("") }

// UninstallShell removes it.
func (a *App) UninstallShell() error { return shell.Uninstall() }

func failAll(paths []string, err error) []ShareOutcome {
	out := make([]ShareOutcome, 0, len(paths))
	for _, p := range paths {
		out = append(out, ShareOutcome{Path: p, Error: err.Error()})
	}
	return out
}
