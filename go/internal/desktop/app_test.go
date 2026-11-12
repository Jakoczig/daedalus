package desktop

import (
	"context"
	"errors"
	"testing"
)

type fakeHost struct {
	urls     []string
	shutdown func()
	fail     bool
	quits    int
}

func (f *fakeHost) Open(url string) error {
	f.urls = append(f.urls, url)
	if f.fail {
		return errors.New("window failed")
	}
	return nil
}
func (f *fakeHost) Run() error { f.shutdown(); return nil }
func (f *fakeHost) Quit()      { f.quits++ }

func TestLifecycle(t *testing.T) {
	for _, fail := range []bool{false, true} {
		stopped := 0
		host := &fakeHost{fail: fail}
		err := run(context.Background(), "http://127.0.0.1:45678", func() { stopped++ }, func(stop func()) (windowHost, error) { host.shutdown = stop; return host, nil })
		if (err != nil) != fail || stopped != 1 || len(host.urls) != 1 || host.urls[0] != "http://127.0.0.1:45678" {
			t.Fatalf("lifecycle: %v stops=%d urls=%v", err, stopped, host.urls)
		}
	}
}

func TestRejectUnsafeDesktopURL(t *testing.T) {
	for _, url := range []string{"http://0.0.0.0:80", "https://example.com", "http://localhost:42", "http://127.0.0.1:42/evil", "http://user@127.0.0.1:42"} {
		called := false
		err := run(context.Background(), url, func() {}, func(func()) (windowHost, error) { called = true; return nil, nil })
		if err == nil || called {
			t.Fatalf("accepted %s", url)
		}
	}
}

func TestFactoryFailureStopsService(t *testing.T) {
	stopped := false
	err := run(context.Background(), "http://127.0.0.1:45678", func() { stopped = true }, func(func()) (windowHost, error) { return nil, errors.New("startup failed") })
	if err == nil || !stopped {
		t.Fatal("startup left service running")
	}
}
