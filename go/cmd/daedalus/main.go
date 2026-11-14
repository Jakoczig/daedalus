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
	"path/filepath"
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
	journalSmoke := flags.Bool("journal-smoke-test", false, "verify an explicitly supplied synthetic journal fixture")
	nativeJournalSmoke := flags.Bool("desktop-journal-smoke-test", false, "verify native React journal card using an explicit fixture")
	navigationSmoke := flags.Bool("navigation-smoke-test", false, "verify explicitly supplied synthetic route and survey")
	nativeNavigationSmoke := flags.Bool("desktop-navigation-smoke-test", false, "verify native owner React route and survey using explicit fixture")
	explorationSmoke := flags.Bool("exploration-smoke-test", false, "verify explicit synthetic map, physical details and organic samples")
	nativeExplorationSmoke := flags.Bool("desktop-exploration-smoke-test", false, "verify native G2C exploration React workspace with explicit fixture")
	terminalSmoke := flags.Bool("terminal-smoke-test", false, "verify explicit synthetic G2D terminal and ship")
	nativeTerminalSmoke := flags.Bool("desktop-terminal-smoke-test", false, "verify native G2D terminal and ship using explicit fixture")
	paritySmoke := flags.Bool("parity-smoke-test", false, "verify explicit synthetic native parity workspace")
	nativeCompanionSmoke := flags.Bool("desktop-companion-smoke-test", false, "verify launcher, optional dashboard closure and shared-service shutdown")
	nativeParitySmoke := flags.Bool("desktop-parity-smoke-test", false, "verify native React G2E parity workspace and shutdown")
	dataDir := flags.String("data-dir", "", "private Go profile directory (legacy profile is never used)")
	journalDir := flags.String("journal-dir", "", "local owner journal directory override")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return errors.New("usage: daedalus [--smoke-test | --desktop-smoke-test]")
	}
	modes := 0
	for _, enabled := range []bool{*smoke, *nativeSmoke, *journalSmoke, *nativeJournalSmoke, *navigationSmoke, *nativeNavigationSmoke, *explorationSmoke, *nativeExplorationSmoke, *terminalSmoke, *nativeTerminalSmoke, *paritySmoke, *nativeParitySmoke, *nativeCompanionSmoke} {
		if enabled {
			modes++
		}
	}
	if modes > 1 {
		return errors.New("choose one smoke mode")
	}
	if (*journalSmoke || *nativeJournalSmoke || *navigationSmoke || *nativeNavigationSmoke || *explorationSmoke || *nativeExplorationSmoke || *terminalSmoke || *nativeTerminalSmoke || *paritySmoke || *nativeParitySmoke || *nativeCompanionSmoke) && *journalDir == "" {
		return errors.New("journal smoke requires --journal-dir with a synthetic fixture")
	}
	if modes > 0 {
		temporary, err := os.MkdirTemp("", "daedalus-go-smoke-")
		if err != nil {
			return errors.New("cannot create isolated smoke profile")
		}
		defer os.RemoveAll(temporary)
		*dataDir = temporary
	} else if *dataDir == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return errors.New("cannot locate private Go profile")
		}
		*dataDir = filepath.Join(config, "daedalus-go")
	}
	controller := &desktop.Controller{}
	if modes == 0 || *nativeCompanionSmoke {
		ctx = desktop.WithController(ctx, controller)
	}
	service, err := webapp.StartLocalWithOptions(ctx, assets, webapp.Options{DesktopAction: controller.Action, DesktopCloseMode: controller.SetCloseMode, StorageDir: *dataDir, EnableJournal: !*smoke && !*nativeSmoke, EnableNavigation: !*smoke && !*nativeSmoke && !*journalSmoke && !*nativeJournalSmoke, EnableDashboard: modes == 0 || *terminalSmoke || *nativeTerminalSmoke || *paritySmoke || *nativeParitySmoke || *nativeCompanionSmoke, EnableParity: modes == 0 || *paritySmoke || *nativeParitySmoke || *nativeCompanionSmoke, EnableExploration: modes == 0 || *explorationSmoke || *nativeExplorationSmoke || *terminalSmoke || *nativeTerminalSmoke || *paritySmoke || *nativeParitySmoke || *nativeCompanionSmoke, NavigationSmoke: *nativeNavigationSmoke || *nativeExplorationSmoke || *nativeTerminalSmoke || *nativeParitySmoke || *nativeCompanionSmoke, JournalDir: *journalDir})
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
	if *journalSmoke {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := webapp.JournalSmokeTest(checkCtx, service); err != nil {
			return err
		}
		fmt.Fprintln(out, "G2A synthetic journal smoke passed")
		return nil
	}
	if *navigationSmoke || *explorationSmoke || *terminalSmoke || *paritySmoke {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := webapp.NavigationSmokeTest(checkCtx, service); err != nil {
			return err
		}
		if *paritySmoke {
			fmt.Fprintln(out, "G2E synthetic parity smoke passed")
		} else if *terminalSmoke {
			fmt.Fprintln(out, "G2D synthetic terminal and ship smoke passed")
		} else if *explorationSmoke {
			fmt.Fprintln(out, "G2C synthetic map, biology and navigation smoke passed")
		} else {
			fmt.Fprintln(out, "G2B synthetic navigation and survey smoke passed")
		}
		return nil
	}
	if *nativeCompanionSmoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 35*time.Second)
		defer cancel()
		result := make(chan error, 1)
		go func() {
			select {
			case <-service.LauncherReady():
			case <-ctx.Done():
				result <- errors.New("launcher did not bootstrap")
				cancel()
				return
			}
			if err := controller.Action("dashboard"); err != nil {
				result <- err
				cancel()
				return
			}
			select {
			case <-service.NavigationClientReady():
			case <-ctx.Done():
				result <- errors.New("separate dashboard did not bootstrap")
				cancel()
				return
			}
			_ = controller.Action("closeDashboard")
			_ = controller.Action("closeLauncher")
			if err := webapp.NavigationSmokeTest(ctx, service); err != nil {
				result <- err
				cancel()
				return
			}
			_ = controller.Action("launcher")
			result <- nil
			cancel()
		}()
		if err := launch(ctx, service.URL()+"/launcher", stop); err != nil {
			return err
		}
		if err := <-result; err != nil {
			return err
		}
		fmt.Fprintln(out, "G2G native launcher, separate dashboard close, background service and quit smoke passed")
		return nil
	}
	var ready atomic.Bool
	if *nativeSmoke || *nativeJournalSmoke || *nativeNavigationSmoke || *nativeExplorationSmoke || *nativeTerminalSmoke || *nativeParitySmoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		go func() {
			readyChannel := service.ClientReady()
			if *nativeJournalSmoke {
				readyChannel = service.JournalClientReady()
			}
			if *nativeNavigationSmoke || *nativeExplorationSmoke || *nativeTerminalSmoke || *nativeParitySmoke {
				readyChannel = service.NavigationClientReady()
			}
			select {
			case <-readyChannel:
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
	location := service.URL()
	if modes == 0 {
		location += "/launcher"
	}
	if *nativeNavigationSmoke || *nativeExplorationSmoke || *nativeTerminalSmoke || *nativeParitySmoke {
		location += "/nav/route"
	}
	if err := launch(ctx, location, stop); err != nil {
		return fmt.Errorf("native desktop failed: %w", err)
	}
	if *nativeSmoke || *nativeJournalSmoke || *nativeNavigationSmoke || *nativeExplorationSmoke || *nativeTerminalSmoke || *nativeParitySmoke {
		if !ready.Load() {
			return errors.New("native React bootstrap was not observed")
		}
		if *nativeNavigationSmoke || *nativeExplorationSmoke || *nativeTerminalSmoke || *nativeParitySmoke {
			if *nativeParitySmoke {
				fmt.Fprintln(out, "G2E native parity workspace bootstrap and shutdown passed")
			} else if *nativeTerminalSmoke {
				fmt.Fprintln(out, "G2D native terminal and ship bootstrap and shutdown passed")
			} else if *nativeExplorationSmoke {
				fmt.Fprintln(out, "G2C native exploration and biology bootstrap and shutdown passed")
			} else {
				fmt.Fprintln(out, "G2B native navigation and survey bootstrap and shutdown passed")
			}
		} else if *nativeJournalSmoke {
			fmt.Fprintln(out, "G2A native journal bootstrap and shutdown passed")
		} else {
			fmt.Fprintln(out, "G1A native React bootstrap and shutdown passed")
		}
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
