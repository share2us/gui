package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"

	"github.com/gen2brain/beeep"
	"github.com/share2us/gui/internal/autostart"
	"github.com/share2us/gui/internal/core"
	"github.com/share2us/gui/internal/receiver"
	"github.com/share2us/gui/internal/sharetarget"
	"github.com/share2us/gui/internal/shell"
	"github.com/share2us/gui/internal/singleton"
	"github.com/share2us/gui/internal/tray"
	"github.com/share2us/gui/internal/update"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// Tray icons (S2u brand): .ico for the Windows tray, PNG elsewhere.
//
//go:embed build/windows/icon.ico
var trayIconICO []byte

//go:embed build/appicon.png
var trayIconPNG []byte

// buildVersion is stamped at release time by CI with a UTC timestamp, mirroring
// the CLI's version model:
//
//	go build -ldflags "-X main.buildVersion=$(date -u +%Y%m%d%H%M%S)"
var buildVersion = "dev"

// main handles the CLI verbs before opening the window:
//
//	share2us-gui.exe share "<path>" ["<path>" ...]   (from the file-manager verb)
//	share2us-gui.exe --install-shell | --uninstall-shell
//	share2us-gui.exe --receive [dir]                 (background receiver, autostarted)
//	share2us-gui.exe --enable-autostart | --disable-autostart
//
// With no verb it opens the share window.
func main() {
	pending, quit := parseArgs(os.Args[1:])
	if quit {
		return
	}
	// Windows Share sheet (MSIX share target): use the shared items as the files.
	if paths, ok := sharetarget.Activation(); ok && len(paths) > 0 {
		pending = paths
	}
	// Keep a background tray alive so closing the share window doesn't quit the
	// app (and it keeps receiving). No-ops if a tray is already running.
	ensureTray()
	runWindow(NewApp(pending))
}

// trayInstancePort is the loopback port the background tray binds to enforce a
// single running instance (see internal/singleton). Arbitrary high port.
const trayInstancePort = "47829"

// ensureTray launches the background tray process detached from this one, unless
// a tray is already running. The spawned process self-terminates immediately if
// it can't take the single-instance lock, so launching the app repeatedly never
// stacks up duplicate tray icons.
func ensureTray() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = exec.Command(exe, "--tray").Start()
}

// parseArgs handles the immediate-exit verbs and returns the selected paths for
// the window otherwise.
func parseArgs(args []string) (pending []string, quit bool) {
	if len(args) == 0 {
		return nil, false
	}
	switch args[0] {
	case "--version", "version":
		fmt.Println(buildVersion)
		return nil, true
	case "--install-shell":
		exitOn("install shell", shell.Install(""))
		return nil, true
	case "--uninstall-shell":
		exitOn("uninstall shell", shell.Uninstall())
		return nil, true
	case "--enable-autostart":
		exitOn("enable autostart", autostart.Enable(""))
		return nil, true
	case "--disable-autostart":
		exitOn("disable autostart", autostart.Disable())
		return nil, true
	case "--receive":
		dir := receiver.DownloadsDir()
		if len(args) > 1 && args[1] != "" {
			dir = args[1]
		}
		runReceive(dir)
		return nil, true
	case "--tray":
		runTray()
		return nil, true
	case "share":
		return args[1:], false
	default:
		// Treat bare path arguments as a share too (drag-onto-exe, etc.).
		return args, false
	}
}

func exitOn(what string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, what+":", err)
		os.Exit(1)
	}
}

// runReceive polls the inbox and saves incoming files into dir until interrupted,
// popping a native toast for each batch. This is the headless background receiver
// launched at login by autostart.
func runReceive(dir string) {
	c, err := core.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "receive:", err)
		os.Exit(1)
	}
	if !c.HasDeviceKey() {
		fmt.Fprintln(os.Stderr, "receive: this login has no device key; sign in with the app first")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Printf("Share2Us: receiving to %s (Ctrl-C to stop)\n", dir)
	receiver.Loop(ctx, c, dir, receiver.DefaultInterval, func(e receiver.Event) {
		if e.Err != nil {
			fmt.Fprintln(os.Stderr, "receive:", e.Err)
			return
		}
		for _, r := range e.Received {
			fmt.Printf("saved %s%s\n", r.FileName, fromSuffix(r.From))
		}
		notifyReceived(e.Received)
	})
}

func notifyReceived(rs []core.Received) {
	if len(rs) == 0 {
		return
	}
	msg := fmt.Sprintf("Received %s%s", rs[0].FileName, fromSuffix(rs[0].From))
	if len(rs) > 1 {
		msg = fmt.Sprintf("Received %d files", len(rs))
	}
	_ = beeep.Notify("Share2Us", msg, "")
}

func fromSuffix(from string) string {
	if from == "" {
		return ""
	}
	return " from " + from
}

// runTray shows the system-tray icon and runs the background receiver behind it
// until the user quits. Launched at login by autostart.
func runTray() {
	// Single-instance: if a tray already holds the lock, exit quietly. This lets
	// ensureTray() fire on every launch without stacking duplicate tray icons.
	lock, ok := singleton.Acquire(trayInstancePort)
	if !ok {
		return
	}
	defer lock.Release()

	dir := receiver.DownloadsDir()
	ctx, cancel := context.WithCancel(context.Background())
	// Receive in the background when signed in with a device key; toast on arrival.
	if c, err := core.Load(); err == nil && c.HasDeviceKey() {
		go receiver.Loop(ctx, c, dir, receiver.DefaultInterval, func(e receiver.Event) {
			if e.Err == nil {
				notifyReceived(e.Received)
			}
		})
	}
	// One-shot update check: toast + reveal the tray "Update available" item.
	updateReady := make(chan string, 1)
	go func() {
		if info, err := update.Check(ctx, buildVersion); err == nil && info.Available {
			_ = beeep.Notify("Share2Us update", "Version "+info.Latest+" is available — open Share2Us to install.", "")
			select {
			case updateReady <- info.Latest:
			case <-ctx.Done():
			}
		}
	}()
	icon := trayIconPNG
	if runtime.GOOS == "windows" {
		icon = trayIconICO
	}
	tray.Run(tray.Options{
		Icon:            icon,
		Tooltip:         "Share2Us",
		OnOpen:          launchSelf,
		OnOpenDownloads: func() { openPath(dir) },
		OnUpdate:        launchSelf, // open the window; the banner has the Install button
		OnQuit:          cancel,
		UpdateReady:     updateReady,
	})
}

// launchSelf opens a fresh Share2Us window in a new process (the tray owns this
// process's main thread, so it can't open the Wails window itself).
func launchSelf() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = exec.Command(exe).Start()
}

// openPath opens a folder in the OS file manager.
func openPath(p string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	_ = cmd.Start()
}

func runWindow(app *App) {
	err := wails.Run(&options.App{
		Title:  "Share2Us",
		Width:  520,
		Height: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 21, A: 1},
		OnStartup:        app.startup,
		// Native file drop: dropped file paths are forwarded to the modal.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
	}
}
