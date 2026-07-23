// Package lan wraps cli-core's lanshare (account-free, direct TLS 1.3 + PAKE
// transfers over the local network / Tailscale) with a GUI-friendly surface:
// one-shot Send that transparently zips folders, and a cancelable background
// Receiver that reports the sender-facing details as soon as it is listening.
package lan

import (
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/share2us/cli-core/lanshare"
	"github.com/share2us/gui/internal/core"
)

// Listen is what a sender needs to reach this receiver, surfaced to the UI.
type Listen struct {
	Address    string `json:"address"`    // ip:port a sender can reach
	Port       int    `json:"port"`       // listen port
	Passphrase string `json:"passphrase"` // password-mode passphrase (else "")
	Pairing    string `json:"pairing"`    // s2u:// string (address + fingerprint + pass)
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
	Name string `json:"name"`
	Addr string `json:"addr"`
	Dest string `json:"dest"`
	Mode string `json:"mode"`
}

// Browse lists nearby Share2Us receivers advertising on the local network.
func Browse(ctx context.Context, timeout time.Duration) ([]Peer, error) {
	found, err := lanshare.Browse(ctx, timeout)
	if err != nil {
		return nil, err
	}
	out := make([]Peer, 0, len(found))
	for _, p := range found {
		out = append(out, Peer{
			Name: p.Name,
			Addr: p.Addr(),
			Dest: lanshare.BuildPairingString(p.Host, lanshare.ListenInfo{Port: p.Port, Fingerprint: p.Fingerprint}),
			Mode: p.Mode,
		})
	}
	return out, nil
}

// Request is an inbound transfer awaiting the user's accept/reject decision.
type Request struct {
	From  string `json:"from"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
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
						DestDir: destDir,
					})
				}
			},
			OnRequest: func(r lanshare.RequestInfo) bool {
				return approve(Request{From: firstNonEmpty(r.PeerIP, "a nearby device"), Name: r.Name, Size: r.Size, IsDir: r.IsDir})
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
