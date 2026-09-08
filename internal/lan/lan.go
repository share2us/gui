// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Hassan Khurram

// Package lan wraps cli-core's lanshare (account-free, direct TLS 1.3 + PAKE
// transfers over the local network / Tailscale) with a GUI-friendly surface:
// one-shot Send that transparently zips folders, and a cancelable background
// Receiver that reports the sender-facing details as soon as it is listening.
package lan

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/share2us/gui/internal/alias"

	"github.com/share2us/cli-core/lanid"
	"github.com/share2us/cli-core/lanshare"
	"github.com/share2us/gui/internal/core"
)

// Listen is what a sender needs to reach this receiver, surfaced to the UI.
type Listen struct {
	Address    string `json:"address"`    // ip:port a sender can reach
	Port       int    `json:"port"`       // listen port
	Passphrase string `json:"passphrase"` // password-mode passphrase (else "")
	Pairing    string `json:"pairing"`    // s2u:// string (address + fingerprint + pass)
	Code       string `json:"code"`       // 6-digit verify code (compare to sender's list)
	DestDir    string `json:"destDir"`    // where received files land
}

// Result mirrors a completed inbound transfer for the UI.
type Result struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	From  string `json:"from"`
}

// SendOne prepares path (zipping a folder) and streams it to dest, which may be
// a bare host / host:port or a full s2u:// pairing string (in which case the
// address, cert fingerprint and passphrase are taken from it). password is used
// only when dest is not a pairing string that already carries one.
func SendOne(ctx context.Context, path, dest, password string, onProgress func(sent, total int64)) error {
	name, size, isDir, readPath, cleanup, err := core.PrepareLocal(path)
	defer cleanup()
	if err != nil {
		return err
	}
	f, err := os.Open(readPath)
	if err != nil {
		return err
	}
	defer f.Close()

	opts := lanshare.SendOptions{Dest: dest, Password: password, OnProgress: onProgress}
	// Attach this device's identity so the receiver can recognise / trust it.
	if id, ierr := lanid.Identity(); ierr == nil {
		opts.Identity = id
		if host, herr := os.Hostname(); herr == nil {
			opts.SenderName = host
		}
	}
	if lanshare.IsPairingString(dest) {
		pi, perr := lanshare.ParsePairingString(dest)
		if perr != nil {
			return perr
		}
		opts.Dest = pi.Addr()
		opts.PinFingerprint = pi.Fingerprint
		if password == "" {
			opts.Password = pi.Password
		}
	}
	// Refuse a send that can't authenticate the receiver: without a pinned cert
	// fingerprint (from a full s2u:// code) AND without a passphrase (PAKE), the
	// TLS session accepts any certificate, so an on-path LAN attacker could
	// intercept the file. Steer the user to the receiver's full code or passphrase.
	if opts.PinFingerprint == "" && opts.Password == "" {
		return errors.New("can't verify that device: use the receiver's full code (it pins the device), or add its passphrase — a bare address alone isn't safe on an untrusted network")
	}
	_, err = lanshare.Send(ctx, name, size, isDir, f, opts)
	return err
}

// Receiver is a running background receiver; call Stop to cancel it.
type Receiver struct{ cancel context.CancelFunc }

// Stop cancels the receiver (safe on a nil/stopped receiver).
func (r *Receiver) Stop() {
	if r != nil && r.cancel != nil {
		r.cancel()
	}
}

// StartReceive opens a password-mode receiver that lands files in destDir. It
// returns immediately with a handle; onListen fires once (with the sender-facing
// Listen details) as soon as the listener is up, onProgress fires as bytes
// arrive, and onDone fires exactly once when a transfer completes (res set) or
// the receiver stops/fails (err set). lanshare.Receive returns after one
// completed transfer, so onDone marks the end of a single receive session.
func StartReceive(parent context.Context, destDir string, onListen func(Listen), onProgress func(received, total int64), onDone func(res *Result, err error)) *Receiver {
	ctx, cancel := context.WithCancel(parent)
	ip := PrimaryIP()
	go func() {
		res, err := lanshare.Receive(ctx, lanshare.ReceiveOptions{
			DestDir:    destDir,
			OnProgress: onProgress,
			OnListen: func(info lanshare.ListenInfo) {
				onListen(Listen{
					Address:    net.JoinHostPort(ip, strconv.Itoa(info.Port)),
					Port:       info.Port,
					Passphrase: info.Passphrase,
					Pairing:    lanshare.BuildPairingString(ip, info),
					Code:       lanshare.VerifyCode(info.Fingerprint),
					DestDir:    destDir,
				})
			},
		})
		if err != nil {
			onDone(nil, err)
			return
		}
		from := res.PeerIP
		if from == "" {
			from = "a nearby device"
		}
		onDone(&Result{Name: res.Name, Path: res.Path, Bytes: res.Bytes, From: from}, nil)
	}()
	return &Receiver{cancel: cancel}
}

// Peer is a nearby advertised receiver, ready for the UI's "nearby devices"
// list. Dest is a dial-ready s2u:// code (pins the peer's fingerprint) to pass
// straight to SendOne.
type Peer struct {
	Name        string `json:"name"`
	Addr        string `json:"addr"`
	Dest        string `json:"dest"` // s2u:// send target (receivers)
	Code        string `json:"code"` // 6-digit verify code (compare to the device's own screen)
	Mode        string `json:"mode"`
	Fingerprint string `json:"fingerprint"` // cert fp (download pinning)
	// Identity is the device's STABLE fingerprint, from its verified device card.
	// Fingerprint above is the per-session certificate — it is what a download
	// pins, but it changes whenever the peer restarts, so it is the wrong thing
	// to remember a device by or to name one after. Empty for a peer that
	// published no card.
	Identity string `json:"identity"`
	// Address is the host on its own, so the UI can show it alongside a name
	// rather than instead of one.
	Address string `json:"address"`
	// Aliased marks a name the USER gave this device, not one the device gave
	// itself — the UI says which, because they carry different weight.
	Aliased     bool   `json:"aliased"`
	IsBroadcast bool   `json:"isBroadcast"` // true = offering a file to download
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
	// ViaScan marks a device found by probing the subnet rather than by mDNS, so
	// the UI can explain why it has an address instead of a name.
	ViaScan bool `json:"viaScan"`
	// ViaTailscale marks a device reached over the tailnet, which is not "nearby"
	// in any physical sense and should not be described as if it were.
	ViaTailscale bool `json:"viaTailscale"`
}

// Browse lists nearby Share2Us endpoints — receivers (send targets) and
// broadcasters (files to download).
//
// It uses TWO independent methods and merges them, because either one alone
// leaves a network where the app simply finds nothing:
//
//   - mDNS carries the device NAME and whether a peer is offering a file, but it
//     is link-local multicast. Plenty of real networks drop it: access points
//     with client isolation, a firewall that blocks inbound UDP 5353, a host
//     whose own responder owns the port, or two devices on different subnets.
//   - A direct subnet probe (lanshare.Scan) opens a TLS handshake against each
//     address and recognises a receiver by its certificate. It never learns a
//     name, but it does not care about multicast at all, so it works exactly
//     where mDNS does not.
//
// Running both means a blocked multicast path degrades the label rather than the
// feature: the device still appears, addressed by IP.
// Deep says whether the direct probe may sweep the whole local subnet, or must
// stay to the handful of addresses in Known.
//
// A sweep is ~254 TLS connects. Done once because a person pressed refresh, that
// is unremarkable. Done every 60 seconds by a background timer, it is the exact
// signature of a port scanner, and endpoint security is right to treat it as
// one. So the sweep is a deliberate act, and the routine tick re-probes only
// addresses already known to be Share2Us devices.
type BrowseOptions struct {
	Timeout time.Duration
	Deep    bool
	// Known are hosts previously seen as devices, re-probed on a shallow pass so
	// one stays listed while multicast is unavailable.
	Known []string
}

func Browse(ctx context.Context, opts BrowseOptions) ([]Peer, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	type scanResult struct {
		peers []lanshare.ScannedPeer
		err   error
	}
	scanCh := make(chan scanResult, 1)
	go func() {
		so := lanshare.ScanOptions{Timeout: 400 * time.Millisecond}
		if !opts.Deep {
			var targets []netip.Addr
			for _, h := range opts.Known {
				if a, err := netip.ParseAddr(h); err == nil {
					targets = append(targets, a)
				}
			}
			so.Targets = targets
			// Tailnet peers ARE enumerated on the routine pass. They come from
			// one `tailscale status` call rather than a sweep, so they cost
			// nothing like a subnet probe — and leaving them to the deep pass
			// meant a Tailscale device stayed invisible for up to ten minutes,
			// which read as "the app cannot see my tailnet devices at all".
			yes := true
			so.IncludeTailscale = &yes
			// Nothing known to re-probe: enumerate the tailnet and stop there. An
			// empty Targets list otherwise means "sweep every local subnet", which
			// is the port-scan shape this pass exists to avoid.
			if len(targets) == 0 {
				so.SkipLocalSubnets = true
			}
		}
		// The tailnet is enumerated rather than swept, so a deep pass stays cheap
		// even though it reaches devices no local broadcast ever could.
		peers, err := lanshare.Scan(ctx, so)
		scanCh <- scanResult{peers, err}
	}()

	found, mdnsErr := lanshare.Browse(ctx, timeout)
	scan := <-scanCh

	// Both failing is a real failure; either one alone is the case this exists
	// for, so it is not reported as an error.
	if mdnsErr != nil && scan.err != nil {
		return nil, mdnsErr
	}

	return mergePeers(found, scan.peers, alias.All()), nil
}

// mergePeers folds the two discovery methods into one list.
//
// Separated from Browse so it can be tested without a network: the rules here —
// which source wins, what is deduplicated, what a nameless peer is called — are
// where the mistakes live, and none of them need a socket to be wrong.
//
// aliases maps a device's stable identity fingerprint to the name THIS user gave
// it, and beats anything the device says about itself.
func mergePeers(found []lanshare.Peer, scanned []lanshare.ScannedPeer, aliases map[string]string) []Peer {
	out := make([]Peer, 0, len(found)+len(scanned))
	// A card belongs to a DEVICE, not to a discovery method, but the mDNS entry
	// is the one kept when both sources see a device — so without this, anything
	// on the local segment (the common case) would be listed from the source that
	// has no verified identity, and could be neither remembered nor named. Index
	// the scan's verified cards by the certificate both sources report.
	cards := make(map[string]lanshare.ScannedPeer, len(scanned))
	for _, p := range scanned {
		if p.Fingerprint != "" && p.IdentityFingerprint != "" {
			cards[p.Fingerprint] = p
		}
	}
	seen := make(map[string]bool, len(found))
	for _, p := range found {
		if p.Fingerprint != "" {
			seen[p.Fingerprint] = true
		}
		// The mDNS name is whatever the TXT record claimed, and a TXT record is
		// attacker-choosable. Where the same device also answered a probe with a
		// signed card, that name is proven and this one is not, so the card wins.
		card := cards[p.Fingerprint]
		out = append(out, Peer{
			Name:        firstNonEmpty(card.Name, p.Name),
			Identity:    card.IdentityFingerprint,
			Addr:        p.Addr(),
			Address:     p.Host,
			Dest:        lanshare.BuildPairingString(p.Host, lanshare.ListenInfo{Port: p.Port, Fingerprint: p.Fingerprint}),
			Code:        lanshare.VerifyCode(firstNonEmpty(card.IdentityFingerprint, p.Fingerprint)),
			Mode:        p.Mode,
			Fingerprint: p.Fingerprint,
			IsBroadcast: p.IsBroadcast,
			FileName:    p.FileName,
			FileSize:    p.FileSize,
		})
	}
	// Add only what mDNS did not already describe: its entry carries the name.
	for _, p := range scanned {
		if p.Fingerprint == "" || seen[p.Fingerprint] {
			continue
		}
		seen[p.Fingerprint] = true
		out = append(out, Peer{
			// The name comes from the peer's VERIFIED device card. Falling back to
			// the address rather than inventing a label, exactly as before, when a
			// peer publishes no card — an older build, or a receiver with no
			// identity.
			Name:         firstNonEmpty(p.Name, p.Host),
			Addr:         p.Addr(),
			Dest:         lanshare.BuildPairingString(p.Host, lanshare.ListenInfo{Port: p.Port, Fingerprint: p.Fingerprint}),
			Code:         lanshare.VerifyCode(firstNonEmpty(p.IdentityFingerprint, p.Fingerprint)),
			Mode:         "",
			Fingerprint:  p.Fingerprint,
			Identity:     p.IdentityFingerprint,
			Address:      p.Host,
			ViaScan:      true,
			ViaTailscale: p.ViaTailscale,
		})
	}
	// The user's own name for a device wins over the one the device published.
	// Applied last, in one place, so no discovery path can miss it.
	for i := range out {
		if a := aliases[out[i].Identity]; a != "" && out[i].Identity != "" {
			out[i].Name = a
			out[i].Aliased = true
		}
	}
	return out
}

// Request is an inbound transfer awaiting the user's accept/reject decision.
// Fingerprint is the sender's verified identity key fingerprint ("" if the
// sender is anonymous) — the trust key; SenderName is a cosmetic label; Code is
// the 6-digit verify code for that fingerprint.
type Request struct {
	From        string `json:"from"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	IsDir       bool   `json:"isDir"`
	Fingerprint string `json:"fingerprint"`
	SenderName  string `json:"senderName"`
	Code        string `json:"code"`
	// SafetyNumber is the long (5x4 digits) comparison number for the TRUST
	// step; the 6-digit Code is only for per-transfer prompts.
	SafetyNumber string `json:"safetyNumber"`
	// Trusted is set by the app when the sender is a trusted device in "ask"
	// mode, so the prompt can drop the verify code and the trust checkbox.
	Trusted bool `json:"trusted"`
}

// Serve runs a persistent, discoverable receiver: it advertises under name,
// accepts many transfers over one listener, asks approve() to accept/reject each
// one, and reports each completed file via onReceived. It returns a handle;
// Stop (or ctx cancel) tears down the listener and the mDNS advertisement.
func Serve(parent context.Context, name, destDir string, onListen func(Listen), approve func(Request) bool, onReceived func(Result), onErr func(error)) *Receiver {
	ctx, cancel := context.WithCancel(parent)
	ip := PrimaryIP()
	go func() {
		var adv io.Closer
		// Publish this device's card: its persistent identity and its name, signed
		// into the listener's certificate. Without it a peer scanning for us learns
		// only a per-session certificate fingerprint — no name, and a different
		// "device" every time we restart.
		identity, iderr := lanid.Identity()
		if iderr != nil {
			identity = nil // no card; discovery degrades to an address, as before
		}
		_, err := lanshare.Receive(ctx, lanshare.ReceiveOptions{
			DestDir:    destDir,
			Identity:   identity,
			DeviceName: name,
			// Bind all interfaces. A single detected IP (via the 8.8.8.8 route
			// trick) is wrong behind a VPN/virtual default route, leaving the LAN
			// unreachable. mDNS publishes every interface address, so binding all
			// keeps the listener reachable on whichever one a peer resolves. The
			// trust/PAKE/approval model — not the bind address — is the guard.
			Bind:       "",
			NoPassword: true, // open listener, but every transfer is user-approved
			Loop:       true,
			OnListen: func(info lanshare.ListenInfo) {
				if a, aerr := lanshare.Advertise(name, info); aerr == nil {
					adv = a
				}
				if onListen != nil {
					onListen(Listen{
						Address: net.JoinHostPort(ip, strconv.Itoa(info.Port)),
						Port:    info.Port,
						Pairing: lanshare.BuildPairingString(ip, info),
						// The STABLE code, not the per-session certificate's. This is
						// what a peer now reads off our card and what the transfer
						// prompt already showed, so the number on this screen finally
						// matches the number on theirs in both places.
						Code:    lanshare.VerifyCode(firstNonEmpty(info.IdentityFingerprint, info.Fingerprint)),
						DestDir: destDir,
					})
				}
			},
			OnRequest: func(r lanshare.RequestInfo) bool {
				fp := lanshare.IdentityFingerprint(r.SenderKey)
				return approve(Request{
					From: firstNonEmpty(r.PeerIP, "a nearby device"), Name: r.Name, Size: r.Size, IsDir: r.IsDir,
					Fingerprint: fp, SenderName: r.SenderName, Code: lanshare.VerifyCode(fp), SafetyNumber: lanshare.SafetyNumber(fp),
				})
			},
			OnReceived: func(res lanshare.ReceiveResult) {
				if onReceived != nil {
					onReceived(Result{Name: res.Name, Path: res.Path, Bytes: res.Bytes, From: firstNonEmpty(res.PeerIP, "a nearby device")})
				}
			},
		})
		if adv != nil {
			_ = adv.Close()
		}
		if err != nil && ctx.Err() == nil && onErr != nil {
			onErr(err)
		}
	}()
	return &Receiver{cancel: cancel}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// BcConn is a broadcast connection update surfaced to the UI (live stats).
type BcConn struct {
	Fingerprint string `json:"fingerprint"` // downloader identity fp ("" = anonymous)
	Name        string `json:"name"`        // downloader display name
	Peer        string `json:"peer"`        // IP
	Sent        int64  `json:"sent"`
	Total       int64  `json:"total"`
	Done        bool   `json:"done"`
	Err         string `json:"err"`
}

// Broadcaster is a running broadcast; Stop cancels it.
type Broadcaster struct{ cancel context.CancelFunc }

// Stop cancels the broadcast.
func (b *Broadcaster) Stop() {
	if b != nil && b.cancel != nil {
		b.cancel()
	}
}

// StartBroadcast serves path to nearby devices (pull) with the given access mode
// ("all" | "trusted" | "approve"), advertising it over mDNS. approve is consulted
// per download in approve mode; onConn reports live per-connection progress.
func StartBroadcast(parent context.Context, path, access string, approve func(Request) bool, onConn func(BcConn), onErr func(error)) (*Broadcaster, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("can't broadcast a folder — zip it first")
	}
	name := filepath.Base(path)
	size := info.Size()
	ctx, cancel := context.WithCancel(parent)
	id, _ := lanid.Identity()
	host, _ := os.Hostname()
	if host == "" {
		host = "Share2Us"
	}
	go func() {
		var adv io.Closer
		berr := lanshare.Broadcast(ctx, lanshare.BroadcastOptions{
			Path: path, Name: name, Bind: "", Access: access, Identity: id,
			IsTrusted: func(fp string) bool { _, ok := lanid.Lookup(fp); return ok },
			OnRequest: func(r lanshare.RequestInfo) bool {
				fp := lanshare.IdentityFingerprint(r.SenderKey)
				return approve(Request{
					From: firstNonEmpty(r.PeerIP, "a nearby device"), Name: r.Name, Size: r.Size,
					Fingerprint: fp, SenderName: r.SenderName, Code: lanshare.VerifyCode(fp), SafetyNumber: lanshare.SafetyNumber(fp),
				})
			},
			OnListen: func(li lanshare.ListenInfo) {
				if a, aerr := lanshare.AdvertiseBroadcast(host, li, name, size); aerr == nil {
					adv = a
				}
			},
			OnConn: func(ev lanshare.ConnEvent) {
				if onConn != nil {
					onConn(BcConn{
						Fingerprint: lanshare.IdentityFingerprint(ev.PeerKey), Name: ev.PeerName, Peer: ev.PeerIP,
						Sent: ev.Sent, Total: ev.Total, Done: ev.Done, Err: ev.Err,
					})
				}
			},
		})
		if adv != nil {
			_ = adv.Close()
		}
		if berr != nil && ctx.Err() == nil && onErr != nil {
			onErr(berr)
		}
	}()
	return &Broadcaster{cancel}, nil
}

// Download pulls a broadcast file (addr + cert fingerprint from Browse) into
// destDir, resuming if interrupted. Returns the result and the broadcaster's
// verified identity fingerprint (for the trust check / offer).
func Download(ctx context.Context, addr, fingerprint, name string, size int64, destDir string, onProgress func(received, total int64)) (Result, string, error) {
	id, _ := lanid.Identity()
	host, _ := os.Hostname()
	res, err := lanshare.Download(ctx, lanshare.DownloadOptions{
		Dest: addr, PinFingerprint: fingerprint, Name: name, Size: size, DestDir: destDir,
		Identity: id, DownloaderName: host, OnProgress: onProgress,
	})
	if err != nil {
		return Result{}, "", err
	}
	fp := lanshare.IdentityFingerprint(res.SenderKey)
	return Result{Name: res.Name, Path: res.Path, Bytes: res.Bytes, From: firstNonEmpty(res.PeerIP, "a nearby device")}, fp, nil
}

// PrimaryIP returns the address a peer on the same network can reach — the IP
// embedded in the pairing code/QR the receiver shows. It prefers a real private
// LAN interface over the internet-routing interface, because on a host whose
// default route is a VPN/Tailscale the 8.8.8.8 trick returns the overlay IP, and a
// sender handed that code connects to an address it can't reach. Falls back to the
// routing IP, then loopback.
func PrimaryIP() string {
	if ip := bestLocalIPv4(); ip != "" {
		return ip
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	if a, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return "127.0.0.1"
}

// bestLocalIPv4 enumerates the host's interfaces and returns the most
// LAN-reachable IPv4: a private RFC1918 address (the real LAN) beats a
// CGNAT/Tailscale 100.64/10 overlay, which beats any other routable address; an
// APIPA 169.254 link-local is last. Returns "" if no usable IPv4 exists. Mirrors
// the receiver-side lanshare.bestAddr choice so both ends agree on the LAN IP.
func bestLocalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	best, bestScore := "", -1
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, aerr := ifc.Addrs()
		if aerr != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			s := localIPv4Score(v4)
			if s > bestScore {
				best, bestScore = v4.String(), s
			}
		}
	}
	return best
}

func localIPv4Score(v4 net.IP) int {
	switch {
	case v4[0] == 169 && v4[1] == 254: // APIPA link-local — usually unreachable
		return 0
	case v4[0] == 100 && v4[1]&0xC0 == 0x40: // 100.64/10 CGNAT (Tailscale overlay)
		return 2
	case v4.IsPrivate(): // 10/8, 172.16/12, 192.168/16 — a real LAN
		return 3
	default: // other routable
		return 1
	}
}
