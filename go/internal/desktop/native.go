package desktop

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type nativeHost struct {
	windowMu      sync.Mutex
	trayAvailable bool
	app           *application.App
	launcher      *application.WebviewWindow
	dashboard     atomic.Pointer[application.WebviewWindow]
	controller    *Controller
	base          string
	companion     bool
	started       atomic.Bool
	requested     atomic.Bool
	stopped       atomic.Bool
	quitOnce      sync.Once
}

func newNativeHost(stop func()) (windowHost, error) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, errors.New("native desktop needs a graphical session and GTK4/WebKitGTK 6.0; use --smoke-test for offline headless verification")
	}
	host := &nativeHost{}
	host.app = application.New(application.Options{
		Name:        "DAEDALUS Companion Preview",
		Description: "Native local-first companion",
		Windows:     application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Linux:       application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
		// Native Wails uses its custom-scheme handler, not another TCP service.
		// All page assets and RPC traffic use the supplied loopback URL.
		Assets:     application.AssetOptions{Handler: http.NotFoundHandler(), DisableLogging: true},
		OnShutdown: func() { host.stopped.Store(true); stop() },
	})
	host.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { host.started.Store(true); host.maybeQuit() })
	return host, nil
}

func (h *nativeHost) Bind(c *Controller) { h.controller = c; c.Bind(h.Action) }
func (h *nativeHost) Open(localURL string) error {
	h.trayAvailable = runtime.GOOS != "linux"
	if runtime.GOOS == "linux" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		out, _ := exec.CommandContext(ctx, "gdbus", "call", "--session", "--dest", "org.freedesktop.DBus", "--object-path", "/org/freedesktop/DBus", "--method", "org.freedesktop.DBus.NameHasOwner", "org.kde.StatusNotifierWatcher").Output()
		h.trayAvailable = strings.Contains(string(out), "true")
	}
	h.companion = strings.HasSuffix(localURL, "/launcher")
	h.base = strings.TrimSuffix(localURL, "/launcher")
	width, height := 1280, 850
	if h.companion {
		width, height = 680, 580
	}
	h.launcher = h.app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "DAEDALUS — Experimental Go Preview", Width: width, Height: height, URL: localURL})
	h.launcher.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if h.companion && h.controller != nil && !h.controller.quitOnClose.Load() && !h.requested.Load() {
			e.Cancel()
			if h.trayAvailable {
				h.launcher.Hide()
			} else {
				h.launcher.Minimise()
			}
		} else {
			h.Quit()
		}
	})
	if h.companion {
		tray := h.app.SystemTray.New()
		icon := image.NewRGBA(image.Rect(0, 0, 32, 32))
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				if (x-16)*(x-16)+(y-16)*(y-16) < 180 {
					icon.Set(x, y, color.RGBA{255, 184, 0, 255})
				}
			}
		}
		var b bytes.Buffer
		_ = png.Encode(&b, icon)
		tray.SetIcon(b.Bytes())
		tray.SetTooltip("DAEDALUS companion — journal service running")
		menu := application.NewMenu()
		menu.Add("Show Launcher").OnClick(func(*application.Context) { _ = h.Action("launcher") })
		menu.Add("Open Dashboard").OnClick(func(*application.Context) { _ = h.Action("dashboard") })
		menu.AddSeparator()
		menu.Add("Quit DAEDALUS").OnClick(func(*application.Context) { h.Quit() })
		tray.SetMenu(menu)
		tray.OnClick(func() { _ = h.Action("launcher") })
	}
	return nil
}
func (h *nativeHost) Action(action string) error {
	switch action {
	case "launcher":
		h.launcher.UnMinimise()
		h.launcher.Show()
		h.launcher.Focus()
	case "hide":
		if h.trayAvailable {
			h.launcher.Hide()
		} else {
			h.launcher.Minimise()
		}
	case "dashboard":
		if !h.companion {
			return errors.New("companion mode required")
		}
		h.windowMu.Lock()
		d := h.dashboard.Load()
		if d == nil {
			d = h.app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "DAEDALUS Dashboard — Experimental", Width: 1366, Height: 900, URL: h.base + "/nav/route"})
			h.dashboard.Store(d)
			d.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
				h.dashboard.CompareAndSwap(d, nil)
			})
		}
		h.windowMu.Unlock()
		d.Show()
		d.Focus()
	case "closeLauncher":
		h.launcher.Close()
	case "closeDashboard":
		d := h.dashboard.Swap(nil)
		if d != nil {
			d.Close()
		}
	case "quit":
		h.Quit()
	default:
		return errors.New("unsupported desktop action")
	}
	return nil
}
func (h *nativeHost) Run() error { defer h.stopped.Store(true); return h.app.Run() }
func (h *nativeHost) Quit()      { h.requested.Store(true); h.maybeQuit() }
func (h *nativeHost) maybeQuit() {
	if h.started.Load() && h.requested.Load() && !h.stopped.Load() {
		h.quitOnce.Do(func() { h.app.Quit() })
	}
}
