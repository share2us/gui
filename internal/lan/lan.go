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
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/share2us/cli-core/lanshare"
	"github.com/share2us/gui/internal/core"
	"github.com/share2us/gui/internal/lanid"
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
	IsBroadcast bool   `json:"isBroadcast"` // true = offering a file to download
	FileName    string `json:"fileName"`
	FileSize    int64  `json:"fileSize"`
}

// Browse lists nearby Share2Us endpoints — receivers (send targets) and
// broadcasters (files to download).
func Browse(ctx context.Context, timeout time.Duration) ([]Peer, error) {
	found, err := lanshare.Browse(ctx, timeout)
	if err != nil {
		return nil, err
	}
	out := make([]Peer, 0, len(found))
	for _, p := range found {
		out = append(out, Peer{
			Name:        p.Name,
			Addr:        p.Addr(),
			Dest:        lanshare.BuildPairingString(p.Host, lanshare.ListenInfo{Port: p.Port, Fingerprint: p.Fingerprint}),
			Code:        lanshare.VerifyCode(p.Fingerprint),
			Mode:        p.Mode,
			Fingerprint: p.Fingerprint,
			IsBroadcast: p.IsBroadcast,
			FileName:    p.FileName,
			FileSize:    p.FileSize,
		})
	}
	return out, nil
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
		_, err := lanshare.Receive(ctx, lanshare.ReceiveOptions{
			DestDir:    destDir,
			Bind:       ip,   // bind the LAN/overlay IP, not 0.0.0.0 (limit exposure)
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
						Code:    lanshare.VerifyCode(info.Fingerprint),
						DestDir: destDir,
					})
				}
			},
			OnRequest: func(r lanshare.RequestInfo) bool {
				fp := lanshare.IdentityFingerprint(r.SenderKey)
				return approve(Request{
					From: firstNonEmpty(r.PeerIP, "a nearby device"), Name: r.Name, Size: r.Size, IsDir: r.IsDir,
					Fingerprint: fp, SenderName: r.SenderName, Code: lanshare.VerifyCode(fp),
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
	ip := PrimaryIP()
	id, _ := lanid.Identity()
	host, _ := os.Hostname()
	if host == "" {
		host = "Share2Us"
	}
	go func() {
		var adv io.Closer
		berr := lanshare.Broadcast(ctx, lanshare.BroadcastOptions{
			Path: path, Name: name, Bind: ip, Access: access, Identity: id,
			IsTrusted: func(fp string) bool { _, ok := lanid.Lookup(fp); return ok },
			OnRequest: func(r lanshare.RequestInfo) bool {
				fp := lanshare.IdentityFingerprint(r.SenderKey)
				return approve(Request{
					From: firstNonEmpty(r.PeerIP, "a nearby device"), Name: r.Name, Size: r.Size,
					Fingerprint: fp, SenderName: r.SenderName, Code: lanshare.VerifyCode(fp),
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

// PrimaryIP returns this host's primary outbound LAN/overlay IP (the address a
// peer on the same network can reach), or 127.0.0.1 if it can't be determined.
// It opens no connection — the UDP "dial" just selects the outbound interface.
func PrimaryIP() string {
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
