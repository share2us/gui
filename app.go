// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/share2us/cli-core/lanid"
	"github.com/share2us/cli-core/lanshare"
	"github.com/share2us/gui/internal/alias"
	"github.com/share2us/gui/internal/autostart"
	"github.com/share2us/gui/internal/clip"
	"github.com/share2us/gui/internal/core"
	"github.com/share2us/gui/internal/incoming"
	"github.com/share2us/gui/internal/knownpeers"
	"github.com/share2us/gui/internal/lan"
	"github.com/share2us/gui/internal/netprofile"
	"github.com/share2us/gui/internal/prefs"
	"github.com/share2us/gui/internal/receiver"
	"github.com/share2us/gui/internal/shell"
	"github.com/share2us/gui/internal/update"

	clicore "github.com/share2us/cli-core"
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
	knownHosts   map[string]bool      // addresses seen as devices, for the cheap re-probe
	lastDeepScan time.Time            // when the subnet was last swept
	discRecv     *lan.Receiver        // persistent discoverable serve loop, if on
	discoverable bool                 // whether we are advertising + serving
	reqs         map[string]chan bool // pending approval prompts, by id
	reqSeq       uint64
	pendingByIP  map[string]int       // in-flight approval prompts per source IP
	pendingTotal int                  // in-flight approval prompts overall
	cooldown     map[string]time.Time // per-IP auto-reject-until after a decline

	bcMu        sync.Mutex
	broadcaster *lan.Broadcaster      // active broadcast, if any
	bcName      string                // broadcast file name
	bcSize      int64                 // broadcast file size
	bcAccess    string                // all | trusted | approve
	bcActive    map[string]lan.BcConn // downloads in progress, keyed by peer IP
	bcDone      []lan.BcConn          // completed downloads
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
	a.refreshWindowTitle()
	// Honour what the user last chose. Without this the app came up invisible
	// every time, which is indistinguishable from the feature being broken — and
	// is what the installer's "Receive files sent to this device" promised.
	if prefs.Load().Discoverable {
		go func() {
			if err := a.SetDiscoverable(true); err != nil {
				wailsRuntime.EventsEmit(a.ctx, "lan-discoverable", map[string]any{"error": err.Error()})
			}
		}()
	}
	go func() {
		// Arrivals nobody filed do not live forever. The count is announced
		// rather than the files vanishing quietly, because a file someone sent
		// you disappearing without a word is worse than the clutter it saves.
		if n := incoming.Sweep(7 * 24 * time.Hour); n > 0 {
			a.notifyArrival(
				fmt.Sprintf("%d unsaved file%s removed", n, map[bool]string{true: "", false: "s"}[n == 1]),
				"They had been waiting over a week. Save them sooner to keep them.",
			)
			wailsRuntime.EventsEmit(a.ctx, "incoming-changed", nil)
		}
	}()
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

// PickFiles opens the native file picker and returns the selected paths (empty if
// the user cancels). Backs the "Choose files" button so sharing works without
// drag-and-drop.
func (a *App) PickFiles() ([]string, error) {
	paths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Choose files to share",
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// ListDevices returns the account's own devices for the "send to device" picker.
func (a *App) ListDevices() ([]core.Device, error) {
	c, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	return c.Devices(a.ctx)
}

// ShareRequest is the modal's submit payload. Target selects the destination:
// "public" | "private" | "only-me" | "device" | "contact".
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
	case "public", "private", "only-me":
		vis := core.Public
		switch req.Target {
		case "private":
			vis = core.Private
		case "only-me":
			vis = core.OnlyMe
		}
		var allow *bool
		if vis == core.Private && req.AllowReshare {
			allow = &req.AllowReshare
		}
		// "Only me" has no recipients by definition; never forward any the modal
		// may still be holding from a switch between destinations.
		recipients := req.Recipients
		if vis == core.OnlyMe {
			recipients = nil
		}
		res, err = c.ShareLink(a.ctx, core.LinkRequest{
			Path:         path,
			Visibility:   vis,
			Recipients:   recipients,
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
	// Record the share in the activity log (metadata only). Cloud link shares
	// keep the URL so the home feed can re-copy it.
	var sz int64
	if fi, serr := os.Stat(path); serr == nil {
		sz = fi.Size()
	}
	switch req.Target {
	case "public":
		lanid.ActivityAppend(lanid.ActivityEntry{Kind: "link", Name: filepath.Base(path), Size: sz, Peer: "public link", Link: res.Link})
	case "private":
		lanid.ActivityAppend(lanid.ActivityEntry{Kind: "link", Name: filepath.Base(path), Size: sz, Peer: "private link", Link: res.Link})
	case "only-me":
		lanid.ActivityAppend(lanid.ActivityEntry{Kind: "link", Name: filepath.Base(path), Size: sz, Peer: "only me", Link: res.Link})
	case "device", "contact":
		lanid.ActivityAppend(lanid.ActivityEntry{Kind: "sent", Name: filepath.Base(path), Size: sz, Peer: firstNonEmptyStr(req.Email, "a device")})
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
		return failAll(paths, errors.New("enter the receiver's address"))
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
			var sz int64
			if fi, serr := os.Stat(path); serr == nil {
				sz = fi.Size()
			}
			lanid.ActivityAppend(lanid.ActivityEntry{Kind: "sent", Name: filepath.Base(path), Size: sz})
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
	r := lan.StartReceive(a.ctx, stageDir(),
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
//
// deep asks for a full sweep of the local subnet. That is a person pressing
// refresh; the timer that runs in the background passes false and re-probes only
// devices already seen, so the app never behaves like a port scanner on a loop.
// A sweep still happens unattended, but rarely enough to be unremarkable.
func (a *App) LanBrowse(deep bool) ([]lan.Peer, error) {
	a.discMu.Lock()
	if !deep && time.Since(a.lastDeepScan) > deepScanEvery {
		deep = true // a new device would otherwise never appear on its own
	}
	known := make([]string, 0, len(a.knownHosts))
	for h := range a.knownHosts {
		known = append(known, h)
	}
	if deep {
		a.lastDeepScan = time.Now()
	}
	a.discMu.Unlock()

	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	peers, err := lan.Browse(ctx, lan.BrowseOptions{Timeout: 1500 * time.Millisecond, Deep: deep, Known: known})
	if err != nil {
		return peers, err
	}
	// Remember where devices were found so the cheap pass can keep them listed.
	a.discMu.Lock()
	if a.knownHosts == nil {
		a.knownHosts = map[string]bool{}
	}
	for _, p := range peers {
		if h, _, e := net.SplitHostPort(p.Addr); e == nil && h != "" {
			a.knownHosts[h] = true
		}
	}
	a.discMu.Unlock()
	return peers, nil
}

// deepScanEvery bounds how long the app can go without a full sweep. Short
// enough that a device switched on is found without anyone pressing anything;
// long enough that it is not a recurring pattern on the network.
const deepScanEvery = 10 * time.Minute

// NetworkProfile reports how the operating system classifies this network, so
// the UI can explain the one failure that looks exactly like the app being
// broken: on a network Windows calls Public, firewall rules scoped to Private do
// not apply, and inbound discovery and transfers are dropped silently.
//
// The installer now allows the app on every profile, so this is a diagnosis for
// installs that predate that or had the rule removed — not the fix.
func (a *App) NetworkProfile() netprofile.Status { return netprofile.Current() }

// OpenNetworkSettings opens the operating system's own network settings, which
// is where the classification is changed. Deliberately not changed for the user:
// it needs administrator rights, and reclassifying a network as trusted is the
// user's decision about their surroundings, not something a file-sharing app
// should do on one click.
func (a *App) OpenNetworkSettings() error {
	return netprofile.OpenSettings()
}

// PeerCheck reports whether a nearby device is presenting the certificate it
// presented last time (W-M4).
//
// The fingerprint a device advertises over mDNS is attacker-choosable, and the
// sender pins exactly that, so "verified" can mean "verified as the impostor".
// The verify code is the defence, but only if somebody compares it, and asking on
// every send teaches people to click through. So the app asks once per device and
// then only when something changes.
//
// It records nothing — PeerRemember is called after the user accepts, never on
// sight, or an impostor seen once would be silently familiar the second time.
func (a *App) PeerCheck(name, fingerprint string) PeerCheckResult {
	status, previous := knownpeers.Check(name, fingerprint)
	return PeerCheckResult{
		Status:       string(status),
		Code:         lanshare.VerifyCode(fingerprint),
		PreviousCode: lanshare.VerifyCode(previous),
	}
}

// PeerCheckResult is what the UI needs to decide whether to ask.
type PeerCheckResult struct {
	// Status is new | same | changed | unknown.
	Status string `json:"status"`
	// Code is the 6-digit verify code for what the device is presenting NOW,
	// which is what the user compares against the device's own screen.
	Code string `json:"code"`
	// PreviousCode is what it presented last time, set only when Status is
	// "changed" — showing both is what makes the change legible.
	PreviousCode string `json:"previousCode"`
}

// PeerRemember records that the user accepted this device presenting this
// certificate. It grants no trust: ADR-034 keeps that server-signed and
// MFA-gated, and this only decides whether to ask again.
func (a *App) PeerRemember(name, fingerprint string) error {
	return knownpeers.Remember(name, fingerprint)
}

// PeerForget drops a device, so the next send is treated as first sight. The
// honest case behind a changed certificate is a reinstalled machine.
func (a *App) PeerForget(name string) error { return knownpeers.Forget(name) }

// LocalAddresses lists this machine's own IPv4 addresses, most LAN-reachable
// first. The status strip shows one; this is for the case a machine has several
// (Ethernet and Wi-Fi, or a VPN), where the person reading a device list needs to
// know which of them is theirs without going to look it up in the OS.
//
// It delegates to the receiver's own ranking rather than sorting again here.
// This used to sort private-before-public with no interface context, so on a
// machine with WSL or Hyper-V the strip showed 172.21.208.1 (private, and first
// in enumeration order) while the other laptop saw 192.168.10.218.
func (a *App) LocalAddresses() []string {
	return lan.RankedIPv4s()
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
		// Remember it. This used to live only in memory, so every launch started
		// invisible however the user had left it.
		_ = prefs.SetDiscoverable(false)
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
	a.discRecv = lan.Serve(a.ctx, name, stageDir(),
		func(l lan.Listen) {
			wailsRuntime.EventsEmit(a.ctx, "lan-discoverable", map[string]any{"address": l.Address, "name": name, "code": l.Code, "safety": lanid.SafetyNumber()})
		},
		a.approveRequest,
		func(res lan.Result) {
			lanid.ActivityAppend(lanid.ActivityEntry{Kind: "received", Peer: res.From, Name: res.Name, Size: res.Bytes})
			a.fileArrival(res)
		},
		func(err error) {
			wailsRuntime.EventsEmit(a.ctx, "lan-discoverable", map[string]any{"error": err.Error()})
		})
	a.discoverable = true
	_ = prefs.SetDiscoverable(true)
	return nil
}

// stageDir is where an arrival lands before the user has said what to do with
// it. Falling back to Downloads keeps a transfer working even if the staging
// directory cannot be created, because losing someone's file is worse than
// filing it somewhere they did not pick.
func stageDir() string {
	if d, err := incoming.StagePath(); err == nil {
		return d
	}
	return receiver.DownloadsDir()
}

// fileArrival decides what happens the moment a file lands.
//
// With a remembered folder it goes straight there and the notification says
// where, so the user can find it. Without one it waits in staging and the
// notification asks for a decision instead of announcing a location the user
// never chose. Both paths notify through here, so swapping the native
// notification for Firebase later is one substitution rather than a hunt
// through the receive paths.
func (a *App) fileArrival(res lan.Result) {
	folder := incoming.Folder()
	if folder != "" {
		// UniquePath, not the bare name: filing into a folder that already holds
		// that name used to rename straight over it, destroying the earlier file
		// with no prompt and no record. A second "report.pdf" now lands as
		// "report (1).pdf".
		dest := core.UniquePath(filepath.Join(folder, filepath.Base(res.Name)))
		if err := os.Rename(res.Path, dest); err == nil {
			// Still list it. A remembered folder means "stop asking me", not "hide
			// it from me" — the user must be able to send this one somewhere else
			// without going to Settings and switching back to "Ask each time".
			// Retention never deletes a filed arrival; it only stops listing it.
			_, _ = incoming.AddFiled(res.Name, res.From, dest, res.Bytes, folder)
			a.notifyArrival(res.Name+" saved", "From "+res.From+" · "+folder)
			wailsRuntime.EventsEmit(a.ctx, "lan-recv-done", map[string]any{
				"name": res.Name, "path": dest, "bytes": res.Bytes, "from": res.From,
			})
			wailsRuntime.EventsEmit(a.ctx, "incoming-changed", nil)
			return
		}
		// Could not file it where they asked; fall through so it waits rather
		// than disappearing.
	}
	// Stage, not Add: the arrival gets a name of its own inside staging so the
	// sender's name is free for the next transfer. Without this a second copy of
	// the same file name failed the whole transfer.
	it, err := incoming.Stage(res.Name, res.From, res.Path, res.Bytes)
	if err != nil {
		a.notifyArrival("Received "+res.Name, "From "+res.From)
	} else {
		a.notifyArrival("Received "+res.Name, "From "+res.From+" · choose where to save it")
		// Ask for the location NOW, rather than leaving a ⤓ button to be found.
		// Approving a transfer and then seeing no file is the confusing half of
		// this flow: the user had already said yes, so the app went quiet and the
		// file appeared to have gone nowhere until they noticed it still had to be
		// saved by hand. The dialog is the frontend's to open, so this only names
		// the arrival; cancelling it leaves the file waiting exactly as before.
		wailsRuntime.EventsEmit(a.ctx, "incoming-arrived", map[string]any{
			"id": it.ID, "name": it.Name, "from": it.From,
		})
	}
	wailsRuntime.EventsEmit(a.ctx, "incoming-changed", nil)
}

// notifyArrival is the single place an arrival is announced. Firebase Cloud
// Messaging replaces the body of this function later; nothing else needs to
// know.
func (a *App) notifyArrival(title, body string) {
	_ = beeep.Notify("Share2Us: "+title, body, "")
}

// IncomingList returns what has arrived and is still waiting to be filed.
func (a *App) IncomingList() []incoming.Item { return incoming.List() }

// IncomingFolder is the remembered save location, "" when the app should ask.
func (a *App) IncomingFolder() string { return incoming.Folder() }

// SetIncomingFolder picks the folder arrivals go to from now on. An empty string
// restores asking each time, so the choice is reversible.
func (a *App) SetIncomingFolder(dir string) error { return incoming.SetFolder(dir) }

// ChooseIncomingFolder opens the native folder picker and remembers the result.
func (a *App) ChooseIncomingFolder() (string, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Where should received files be saved?",
	})
	if err != nil || dir == "" {
		return "", err
	}
	return dir, incoming.SetFolder(dir)
}

// SaveIncoming asks where a waiting file should go and puts it there. Returns
// the folder it was saved into so the caller can offer to remember it.
func (a *App) SaveIncoming(id string) (string, error) {
	it, ok := incoming.Get(id)
	if !ok {
		return "", fmt.Errorf("that file is no longer waiting")
	}
	dest, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "Save received file",
		DefaultFilename: it.Name,
	})
	if err != nil {
		return "", err
	}
	if dest == "" {
		return "", nil // cancelled: the file stays put
	}
	if err := incoming.Claim(id, dest); err != nil {
		return "", err
	}
	wailsRuntime.EventsEmit(a.ctx, "incoming-changed", nil)
	return filepath.Dir(dest), nil
}

// DiscardIncoming deletes a waiting file the user does not want.
func (a *App) DiscardIncoming(id string) error {
	if err := incoming.Discard(id); err != nil {
		return err
	}
	wailsRuntime.EventsEmit(a.ctx, "incoming-changed", nil)
	return nil
}

// SweepIncoming deletes arrivals nobody filed in a week and reports the count,
// so they are never removed silently.
func (a *App) SweepIncoming() int { return incoming.Sweep(7 * 24 * time.Hour) }

// approveRequest decides an inbound transfer. A device the receiver has trusted
// (by its verified key fingerprint) bypasses the verify code and the anti-spam
// caps: "auto" trust lands silently, "ask" trust still prompts (labelled
// trusted). An untrusted / anonymous sender is subject to the anti-spam limits
// (per-IP cap, global cap, post-decline cooldown) and then the normal prompt,
// which also offers "Accept & trust". Runs concurrently (one goroutine per
// inbound connection).
func (a *App) approveRequest(r lan.Request) bool { return a.gate(r, "send") }

// approveDownload gates an inbound broadcast download (approve access mode).
func (a *App) approveDownload(r lan.Request) bool { return a.gate(r, "download") }

// gate decides an inbound transfer (send) or broadcast download. A trusted device
// (by verified key fingerprint) auto-accepts; otherwise the anti-spam limits apply
// and the user is prompted. action ("send"|"download") only affects prompt wording.
func (a *App) gate(r lan.Request, action string) bool {
	if r.Fingerprint != "" {
		if d, ok := lanid.Lookup(r.Fingerprint); ok {
			if d.AutoAccept() {
				return true // trusted + auto: lands silently (toast on arrival)
			}
			// trusted + ask: no verify code, no anti-spam caps, one tap to accept.
			r.Trusted = true
			return a.promptApproval(r, action)
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

	ok := a.promptApproval(r, action)

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
// (or a timeout / shutdown). action ("send"|"download") lets the UI word it.
func (a *App) promptApproval(r lan.Request, action string) bool {
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
		"fingerprint": r.Fingerprint, "senderName": r.SenderName, "code": r.Code, "action": action,
		"trusted": r.Trusted, "safetyNumber": r.SafetyNumber,
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

// TrustChallengeInfo is what the UI needs to collect the second factor.
type TrustChallengeInfo struct {
	ChallengeID  string `json:"challengeId"`
	Factor       string `json:"factor"` // "email" | "totp"
	SentTo       string `json:"sentTo"`
	VerifyCode   string `json:"verifyCode"`
	SafetyNumber string `json:"safetyNumber"` // compare with the other device before trusting
	ExpiresIn    int    `json:"expiresIn"`
}

// TrustDevice opens a trust challenge for a device (ADR-034). Nothing is trusted
// until VerifyTrust succeeds with the code the server delivered (email, or the
// user's authenticator once enrolled). Needs a signed-in interactive login;
// personal API tokens are refused, so an agent holding one cannot trust.
func (a *App) TrustDevice(fingerprint, name, mode string) (TrustChallengeInfo, error) {
	c, err := a.clientOrErr()
	if err != nil {
		return TrustChallengeInfo{}, errors.New("sign in to trust a device")
	}
	if c.IsAPIToken() {
		return TrustChallengeInfo{}, errors.New("trusting a device needs an interactive login, not an API token")
	}
	ch, err := c.TrustOpen(a.ctx, fingerprint, name, mode)
	if err != nil {
		return TrustChallengeInfo{}, err
	}
	return TrustChallengeInfo{ChallengeID: ch.ChallengeID, Factor: ch.Factor, SentTo: ch.SentTo, VerifyCode: ch.VerifyCode, SafetyNumber: lanshare.SafetyNumber(fingerprint), ExpiresIn: ch.ExpiresIn}, nil
}

// VerifyTrust submits the code for a challenge; on success the device is
// trusted on the account and the signed list is cached. A wrong code returns
// the server's message (the user may retry while attempts remain).
func (a *App) VerifyTrust(challengeID, code string) error {
	c, err := a.clientOrErr()
	if err != nil {
		return errors.New("sign in to trust a device")
	}
	if err := c.TrustVerify(a.ctx, challengeID, code); err != nil {
		var apiErr *clicore.APIError
		if errors.As(err, &apiErr) && apiErr.Message != "" {
			return errors.New(apiErr.Message)
		}
		return err
	}
	return nil
}

// SetTrustMode downgrades a device to "ask" through the account. "auto" widens
// trust and must go through TrustDevice/VerifyTrust; the UI routes it there.
func (a *App) SetTrustMode(fingerprint, mode string) error {
	if lanidNormalize(mode) == lanid.ModeAuto {
		return errors.New("switching to auto needs verification")
	}
	c, err := a.clientOrErr()
	if err != nil {
		return errors.New("sign in to change trusted devices")
	}
	return c.TrustSetMode(a.ctx, fingerprint, lanid.ModeAsk)
}

// UntrustDevice revokes trust for a device on the account.
func (a *App) UntrustDevice(fingerprint string) error {
	c, err := a.clientOrErr()
	if err != nil {
		return errors.New("sign in to change trusted devices")
	}
	return c.TrustRevoke(a.ctx, fingerprint)
}

func lanidNormalize(mode string) string {
	m, err := lanid.NormalizeMode(mode)
	if err != nil {
		return lanid.ModeAsk
	}
	return m
}

// TrustedDeviceView is a trusted device with its effective mode resolved
// (legacy records without a mode read as "ask").
type TrustedDeviceView struct {
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name"`
	Mode        string `json:"mode"`
}

// ListTrusted syncs the account's trusted devices (best-effort; offline keeps the
// verified cache) and returns them for the Settings list.
func (a *App) ListTrusted() []TrustedDeviceView {
	if c, err := a.clientOrErr(); err == nil && !c.IsAPIToken() {
		_ = c.TrustSync(a.ctx)
	}
	list := lanid.List()
	out := make([]TrustedDeviceView, 0, len(list))
	for _, d := range list {
		out = append(out, TrustedDeviceView{Fingerprint: d.Fingerprint, Name: d.Name, Mode: d.EffectiveMode()})
	}
	return out
}

// ---- broadcast (pull) ----

// BroadcastState is the live view of the current broadcast for the detail screen.
type BroadcastState struct {
	Active      bool         `json:"active"`
	Name        string       `json:"name"`
	Size        int64        `json:"size"`
	Access      string       `json:"access"`
	Downloading []lan.BcConn `json:"downloading"`
	Completed   []lan.BcConn `json:"completed"`
}

// StartBroadcast offers path to nearby devices (pull) with the given access mode
// ("all" | "trusted" | "approve", default approve). Replaces any current broadcast.
func (a *App) StartBroadcast(path, access string) (BroadcastState, error) {
	info, err := os.Stat(path)
	if err != nil {
		return BroadcastState{}, err
	}
	if access != "all" && access != "trusted" && access != "approve" {
		access = "approve"
	}
	a.bcMu.Lock()
	if a.broadcaster != nil {
		a.broadcaster.Stop()
	}
	a.bcActive = make(map[string]lan.BcConn)
	a.bcDone = nil
	a.bcName = filepath.Base(path)
	a.bcSize = info.Size()
	a.bcAccess = access
	a.bcMu.Unlock()

	b, err := lan.StartBroadcast(a.ctx, path, access, a.approveDownload, a.onBcConn, func(e error) {
		wailsRuntime.EventsEmit(a.ctx, "lan-bc-error", map[string]any{"error": e.Error()})
	})
	if err != nil {
		return BroadcastState{}, err
	}
	a.bcMu.Lock()
	a.broadcaster = b
	a.bcMu.Unlock()
	return a.broadcastState(), nil
}

// onBcConn tracks a broadcast connection (progress / completion) and forwards it.
func (a *App) onBcConn(c lan.BcConn) {
	a.bcMu.Lock()
	name, size := a.bcName, a.bcSize
	switch {
	case c.Done:
		delete(a.bcActive, c.Peer)
		a.bcDone = append(a.bcDone, c)
	case c.Err != "":
		delete(a.bcActive, c.Peer)
	default:
		if a.bcActive == nil {
			a.bcActive = make(map[string]lan.BcConn)
		}
		a.bcActive[c.Peer] = c
	}
	a.bcMu.Unlock()
	if c.Done {
		lanid.ActivityAppend(lanid.ActivityEntry{Kind: "broadcast", Peer: firstNonEmptyStr(c.Name, c.Peer), Name: name, Size: size})
	}
	wailsRuntime.EventsEmit(a.ctx, "lan-bc-conn", c)
}

// StopBroadcast ends the current broadcast.
func (a *App) StopBroadcast() {
	a.bcMu.Lock()
	b := a.broadcaster
	a.broadcaster = nil
	a.bcName, a.bcActive, a.bcDone = "", nil, nil
	a.bcMu.Unlock()
	b.Stop()
}

// BroadcastStats returns the current broadcast + its live connections.
func (a *App) BroadcastStats() BroadcastState { return a.broadcastState() }

func (a *App) broadcastState() BroadcastState {
	a.bcMu.Lock()
	defer a.bcMu.Unlock()
	// Non-nil slices so they marshal as [] not null (the frontend calls .reduce/
	// .map on them the moment a broadcast starts, before any connection exists).
	st := BroadcastState{
		Active:      a.broadcaster != nil,
		Name:        a.bcName,
		Size:        a.bcSize,
		Access:      a.bcAccess,
		Downloading: []lan.BcConn{},
		Completed:   []lan.BcConn{},
	}
	for _, c := range a.bcActive {
		st.Downloading = append(st.Downloading, c)
	}
	st.Completed = append(st.Completed, a.bcDone...)
	return st
}

// DownloadResult reports a completed broadcast download + the source identity so
// the UI can offer to trust it.
type DownloadResult struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	From        string `json:"from"`
	Trusted     bool   `json:"trusted"`
}

// LanDownload pulls a broadcast file (addr + cert fingerprint from the nearby
// list) into Downloads, resuming if it was interrupted before.
func (a *App) LanDownload(addr, fingerprint, name string, size int64) (DownloadResult, error) {
	res, fp, err := lan.Download(a.ctx, addr, fingerprint, name, size, receiver.DownloadsDir(),
		func(recv, total int64) {
			wailsRuntime.EventsEmit(a.ctx, "lan-dl-progress", map[string]any{"name": name, "received": recv, "total": total})
		})
	if err != nil {
		return DownloadResult{}, err
	}
	lanid.ActivityAppend(lanid.ActivityEntry{Kind: "downloaded", Peer: res.From, Name: res.Name, Size: size})
	_ = beeep.Notify("Share2Us", "Downloaded "+res.Name, "")
	_, trusted := lanid.Lookup(fp)
	return DownloadResult{Name: res.Name, Fingerprint: fp, From: res.From, Trusted: trusted}, nil
}

// ---- activity log + scan interval ----

// ActivityLog returns the recent transfer/broadcast log (newest first).
func (a *App) ActivityLog() []lanid.ActivityEntry { return lanid.ActivityList() }

// ClearActivity empties the activity log.
func (a *App) ClearActivity() { lanid.ActivityClear() }

// GetScanInterval / SetScanInterval control the broadcast-discovery scan cadence.
func (a *App) GetScanInterval() int        { return lanid.GetScanInterval() }
func (a *App) SetScanInterval(s int) error { return lanid.SetScanInterval(s) }

// firstNonEmptyStr returns a if non-empty, else b.
func firstNonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
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

// IsStoreManaged reports whether this copy is distributed via the Microsoft Store
// (built with -tags store, or running as an MSIX package). The frontend uses it to
// hide the update controls; when true the app never self-updates (Store policy
// 10.2.5 — the Store owns updates).
func (a *App) IsStoreManaged() bool {
	return update.IsStoreManaged()
}

// CheckUpdate reports whether a newer Share2Us release is available for this OS.
// Store-managed copies never self-update, so it always reports none available.
func (a *App) CheckUpdate() update.Info {
	channel := a.UpdateChannel()
	if update.IsStoreManaged() {
		return update.Info{Current: buildVersion, Channel: channel}
	}
	info, err := update.Check(a.ctx, buildVersion, channel)
	if err != nil {
		return update.Info{Current: buildVersion, Channel: channel}
	}
	return info
}

// UpdateChannel is the release channel this machine follows ("stable" or
// "beta"). It is the CLI's setting too: both read update_channel from the shared
// cli-core config.json, so `s2u update --channel beta` and this toggle are one
// machine-wide choice. Store-managed installs always report stable.
// BuildVersion is the release stamp this binary was built with, shown in the
// window footer so a user can report exactly what they are running. "dev" for a
// local build that CI never stamped.
func (a *App) BuildVersion() string { return buildVersion }

func (a *App) UpdateChannel() string {
	if update.IsStoreManaged() {
		return update.ChannelStable
	}
	cfg, err := clicore.LoadConfig()
	if err != nil {
		return update.ChannelStable
	}
	return update.NormalizeChannel(clicore.ResolveUpdateChannel(cfg))
}

// SetUpdateChannel saves the channel ("stable" or "beta") to the shared config.
// Stable is stored as the empty default so config.json stays minimal.
func (a *App) SetUpdateChannel(channel string) error {
	if update.IsStoreManaged() {
		return nil
	}
	cfg, err := clicore.LoadConfig()
	if err != nil {
		cfg = clicore.Config{}
	}
	switch update.NormalizeChannel(channel) {
	case update.ChannelBeta:
		cfg.UpdateChannel = clicore.UpdateChannelBeta
	default:
		cfg.UpdateChannel = ""
	}
	return clicore.SaveConfig(cfg)
}

// ApplyUpdate downloads and launches the update. On Windows it runs the installer
// and quits so it can replace the running app; elsewhere it opens the release page
// (self-replacing a running GUI is unreliable cross-platform).
func (a *App) ApplyUpdate() error {
	if update.IsStoreManaged() {
		return nil // the Microsoft Store applies updates; never self-update
	}
	info, err := update.Check(a.ctx, buildVersion, a.UpdateChannel())
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
		// Integrity gate. The checksum is the one that actually proves something
		// here: the Authenticode signature is made with a per-run self-signed cert
		// (see update.VerifySignature), so it can confirm the file is still signed
		// by us but never that the signer is the same publisher as last release.
		// Both are fail-closed; the download is discarded unless both pass.
		if err := update.VerifyChecksum(a.ctx, &http.Client{Timeout: 30 * time.Second}, path, info.SHA256URL); err != nil {
			_ = os.Remove(path)
			return err
		}
		if err := update.VerifySignature(path); err != nil {
			_ = os.Remove(path)
			return err
		}
		// ShellExecute (not os/exec) so the installer's requireAdministrator
		// manifest elevates via UAC; CreateProcess would fail with
		// ERROR_ELEVATION_REQUIRED and the installer would never appear.
		if err := update.LaunchInstaller(path); err != nil {
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

// refreshWindowTitle puts this machine's LAN address in the window title.
//
// It answers a question the app could not: standing at a second laptop, which of
// these two machines is 192.168.15.114? The address was inside the app, in the
// status strip, and only while discoverable — so the one moment you needed it
// (matching a device list against the machine in front of you) was the moment it
// might not be shown. The title bar and the taskbar entry carry it now, readable
// without the window even being focused.
//
// A machine with no LAN address keeps the plain name rather than showing an
// empty separator.
func (a *App) refreshWindowTitle() {
	if a.ctx == nil {
		return
	}
	wailsRuntime.WindowSetTitle(a.ctx, windowTitle(a.LocalAddresses()))
}

// windowTitle names the window after this machine's LAN address.
//
// LocalAddresses ranks the real LAN interface above host-only ones, so the first
// entry is the address the other laptop will actually see. A machine with no LAN
// address keeps the plain name rather than showing an empty separator.
func windowTitle(addrs []string) string {
	if len(addrs) == 0 || addrs[0] == "" {
		return "Share2Us"
	}
	return "Share2Us — " + addrs[0]
}

// PeerAlias names a nearby device, in this copy of the app only.
//
// A device publishes a name it signed, so nobody can take it — but it is still
// whatever its owner typed, and three laptops called "DESKTOP-4F2K9A" are not a
// security problem and are still an unusable list. Passing an empty name clears
// the alias and lets the device's own name show again.
//
// Keyed on the device's stable identity, so the name survives the things that
// actually change: a new DHCP lease, a different subnet, a VPN, a randomised MAC,
// or the peer simply restarting (which regenerates its session certificate).
func (a *App) PeerAlias(identity, name string) error { return alias.Set(identity, name) }
