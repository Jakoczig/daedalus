package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/desktop"
	"github.com/Jakoczig/daedalus/go/internal/frontend"
	"github.com/Jakoczig/daedalus/go/internal/webapp"
)

type launcher func(context.Context, string, func()) error

func run(ctx context.Context, args []string, assets fs.FS, launch launcher, out io.Writer) error {
	flags := flag.NewFlagSet("daedalus", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	smoke := flags.Bool("smoke-test", false, "verify offline service without a window")
	nativeSmoke := flags.Bool("desktop-smoke-test", false, "open native window, require React authentication and RPC, then close")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return errors.New("usage: daedalus [--smoke-test | --desktop-smoke-test]")
	}
	if *smoke && *nativeSmoke {
		return errors.New("choose one smoke mode")
	}
	service, err := webapp.StartLocal(ctx, assets)
	if err != nil {
		return errors.New("cannot start embedded local service")
	}
	stop := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = service.Close(closeCtx)
	}
	defer stop()
	if *smoke {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := webapp.SmokeTest(checkCtx, service, assets); err != nil {
			return errors.New("G1A offline smoke failed")
		}
		stop()
		fmt.Fprintln(out, "G1A offline smoke passed")
		return nil
	}
	var ready atomic.Bool
	if *nativeSmoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		go func() {
			select {
			case <-service.ClientReady():
				ready.Store(true)
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
				}
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	if err := launch(ctx, service.URL(), stop); err != nil {
		return fmt.Errorf("native desktop failed: %w", err)
	}
	if *nativeSmoke {
		if !ready.Load() {
			return errors.New("native React bootstrap was not observed")
		}
		fmt.Fprintln(out, "G1A native React bootstrap and shutdown passed")
	}
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	assets, err := frontend.FS()
	if err == nil {
		err = run(ctx, os.Args[1:], assets, desktop.Run, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
