package desktop

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type Controller struct {
	mu          sync.Mutex
	handle      func(string) error
	quitOnClose atomic.Bool
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
