package desktop

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

type Controller struct {
	mu           sync.Mutex
	handle       func(string) error
	clipboard    func(string) bool
	saveCSV      func(string, string) error
	chooseAvatar func() ([]byte, error)
	quitOnClose  atomic.Bool
}

func (c *Controller) BindAvatar(f func() ([]byte, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chooseAvatar = f
}
func (c *Controller) ChooseAvatar() ([]byte, error) {
	c.mu.Lock()
	f := c.chooseAvatar
	c.mu.Unlock()
	if f == nil {
		return nil, errors.New("Use the native launcher to change portraits")
	}
	return f()
}

func (c *Controller) BindCSV(f func(string, string) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.saveCSV = f
}
func (c *Controller) SaveFieldCSV(name, data string) error {
	if (name != "daedalus-itinerary.csv" && name != "daedalus-private-bgs.csv") || len(data) > 2<<20 {
		return errors.New("Invalid field export")
	}
	c.mu.Lock()
	f := c.saveCSV
	c.mu.Unlock()
	if f == nil {
		return errors.New("Native save dialog unavailable")
	}
	return f(name, data)
}

func (c *Controller) BindClipboard(f func(string) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clipboard = f
}
func (c *Controller) CopyWaypoint(name string) error {
	if name == "" || len(name) > 256 || !utf8.ValidString(name) {
		return errors.New("Invalid waypoint")
	}
	c.mu.Lock()
	f := c.clipboard
	c.mu.Unlock()
	if f == nil || !f(name) {
		return errors.New("Native clipboard unavailable; select and copy manually")
	}
	return nil
}

type controllerKey struct{}

func WithController(ctx context.Context, c *Controller) context.Context {
	return context.WithValue(ctx, controllerKey{}, c)
}
func (c *Controller) Bind(f func(string) error) { c.mu.Lock(); defer c.mu.Unlock(); c.handle = f }
func (c *Controller) SetCloseMode(mode string)  { c.quitOnClose.Store(mode == "quit") }
func (c *Controller) Action(action string) error {
	switch action {
	case "dashboard", "launcher", "hide", "closeDashboard", "closeLauncher", "quit":
	default:
		return errors.New("unsupported desktop action")
	}
	c.mu.Lock()
	f := c.handle
	c.mu.Unlock()
	if f == nil {
		return errors.New("native desktop not ready")
	}
	return f(action)
}
