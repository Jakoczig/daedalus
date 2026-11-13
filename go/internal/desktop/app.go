// Package desktop owns native windows, never a second HTTP listener.
package desktop

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
)

type windowHost interface {
	Open(string) error
	Run() error
	Quit()
}
type hostFactory func(func()) (windowHost, error)

// Run opens the existing local service. onShutdown also runs when the native
// platform closes its last window, before process teardown can bypass defers.
func Run(ctx context.Context, localURL string, onShutdown func()) error {
	return run(ctx, localURL, onShutdown, newNativeHost)
}

func run(ctx context.Context, localURL string, onShutdown func(), factory hostFactory) error {
	var once sync.Once
	stop := func() { once.Do(onShutdown) }
	defer stop()
	parsed, err := url.Parse(localURL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/nav/route" && parsed.Path != "/exploration/system") || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid local desktop URL")
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil || host != "127.0.0.1" || port == "" {
		return errors.New("invalid local desktop URL")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	native, err := factory(stop)
	if err != nil {
		return err
	}
	if err = native.Open(localURL); err != nil {
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			native.Quit()
		case <-done:
		}
	}()
	return native.Run()
}
