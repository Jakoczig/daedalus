package desktop

import (
	"errors"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type nativeHost struct {
	app       *application.App
	started   atomic.Bool
	requested atomic.Bool
	stopped   atomic.Bool
	quitOnce  sync.Once
}

func newNativeHost(stop func()) (windowHost, error) {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, errors.New("native desktop needs a graphical session and GTK4/WebKitGTK 6.0; use --smoke-test for offline headless verification")
	}
	host := &nativeHost{}
	host.app = application.New(application.Options{
		Name:        "DAEDALUS G1A Offline Preview",
		Description: "Offline desktop foundation; gameplay is not implemented",
		// Native Wails uses its custom-scheme handler, not another TCP service.
		// All page assets and RPC traffic use the supplied loopback URL.
		Assets:     application.AssetOptions{Handler: http.NotFoundHandler(), DisableLogging: true},
		OnShutdown: func() { host.stopped.Store(true); stop() },
	})
	host.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { host.started.Store(true); host.maybeQuit() })
	return host, nil
}

func (h *nativeHost) Open(localURL string) error {
	h.app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "DAEDALUS — Experimental Go Preview", Width: 1280, Height: 850, URL: localURL})
	return nil
}
func (h *nativeHost) Run() error { defer h.stopped.Store(true); return h.app.Run() }
func (h *nativeHost) Quit()      { h.requested.Store(true); h.maybeQuit() }
func (h *nativeHost) maybeQuit() {
	if h.started.Load() && h.requested.Load() && !h.stopped.Load() {
		h.quitOnce.Do(func() { h.app.Quit() })
	}
}
