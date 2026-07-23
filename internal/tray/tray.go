// Package tray runs the Share2Us system-tray icon and its menu (Open, Open
// Downloads, Quit) around the background receiver. systray.Run takes over the
// main thread, so the tray is its own process mode (--tray), separate from the
// Wails window.
package tray

import "fyne.io/systray"

// Options configures the tray. Icon is platform-appropriate bytes (.ico on
// Windows, PNG elsewhere). The On* callbacks may be nil. UpdateReady, when a
// version string arrives on it, reveals an "Update available" menu item.
type Options struct {
	Icon            []byte
	Tooltip         string
	OnOpen          func()
	OnOpenDownloads func()
	OnUpdate        func()
	OnQuit          func()
	UpdateReady     <-chan string
}

// Run shows the tray and blocks until the user quits it, then calls OnQuit.
func Run(opts Options) {
	systray.Run(func() {
		if len(opts.Icon) > 0 {
			systray.SetIcon(opts.Icon)
		}
		if opts.Tooltip != "" {
			systray.SetTooltip(opts.Tooltip)
		}
		mOpen := systray.AddMenuItem("Open Share2Us", "Open the Share2Us window")
		mUpdate := systray.AddMenuItem("Update available", "Install the latest Share2Us")
		mUpdate.Hide()
		mDownloads := systray.AddMenuItem("Open Downloads", "Open your Downloads folder")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop receiving and exit")
		updates := opts.UpdateReady
		if updates == nil {
			updates = make(chan string) // never fires
		}
		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					if opts.OnOpen != nil {
						opts.OnOpen()
					}
				case <-mUpdate.ClickedCh:
					if opts.OnUpdate != nil {
						opts.OnUpdate()
					}
				case <-mDownloads.ClickedCh:
					if opts.OnOpenDownloads != nil {
						opts.OnOpenDownloads()
					}
				case v := <-updates:
					if v != "" {
						mUpdate.SetTitle("Update available (v" + v + ")")
					}
					mUpdate.Show()
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		if opts.OnQuit != nil {
			opts.OnQuit()
		}
	})
}
