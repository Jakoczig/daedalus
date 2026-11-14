package webapp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/gamestate"
	"github.com/Jakoczig/daedalus/go/internal/journal"
)

type Options struct {
	StorageDir        string
	EnableJournal     bool
	EnableNavigation  bool
	EnableExploration bool
	EnableDashboard   bool
	NavigationSmoke   bool
	JournalDir        string
	Discovery         journal.DiscoveryOptions
	PollInterval      time.Duration
}

func (s *Server) startGame(ctx context.Context, opts Options) {
	if !opts.EnableJournal {
		return
	}
	directory := opts.JournalDir
	configError := ""
	if directory == "" {
		var err error
		directory, err = journal.LoadOverride(opts.StorageDir)
		if err != nil {
			configError = err.Error()
		}
	}
	s.g2b = opts.EnableNavigation
	s.g2c = opts.EnableExploration && s.g2b
	s.g2d = opts.EnableDashboard && s.g2c
	s.navigationSmoke = opts.NavigationSmoke && s.g2b
	s.game, _ = gamestate.NewService(gamestate.Options{Directory: directory, Discovery: opts.Discovery, ConfigError: configError, PollInterval: opts.PollInterval})
	gameCtx, cancel := context.WithCancel(ctx)
	s.gameCancel = cancel
	s.gameDone = make(chan struct{})
	s.profile = opts.StorageDir
	go func() { defer close(s.gameDone); _ = s.game.Run(gameCtx) }()
}
func (s *Server) prototype() map[string]any {
	stage := "G1A"
	if s.remote != nil {
		stage = "G1B"
	}
	if s.game != nil {
		stage = "G2A"
		if s.g2b {
			stage = "G2B"
			if s.g2c {
				stage = "G2C"
				if s.g2d {
					stage = "G2D"
				}
			}
		}
	}
	marker := map[string]any{"stage": stage, "offline": s.game == nil}
	if s.navigationSmoke {
		marker["navigationSmoke"] = true
	}
	return marker
}
func (s *Server) ownerJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" || !s.sameOrigin(r) || !s.sessions.valid(s.sessions.token(r)) {
		http.Error(w, "Forbidden", 403)
		return
	}
	if s.game == nil {
		http.Error(w, "Journal ingestion disabled", 404)
		return
	}
	var request struct {
		Action    string `json:"action"`
		Directory string `json:"directory"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return
	}
	// Decode an untrusted/slow body before acquiring the configuration lock.
	s.gameConfigMu.Lock()
	defer s.gameConfigMu.Unlock()
	if !s.sessions.valid(s.sessions.token(r)) {
		http.Error(w, "Session closed", 403)
		return
	}
	switch request.Action {
	case "status":
	case "selectDir", "clearOverride":
		dir := request.Directory
		if request.Action == "clearOverride" {
			dir = ""
		}
		if dir != "" {
			if err := journal.ValidateDir(dir); err != nil {
				http.Error(w, "Choose an existing readable absolute journal directory", 400)
				return
			}
		}
		if err := journal.SaveOverride(s.profile, dir); err != nil {
			http.Error(w, "Journal preference storage unavailable; existing source retained", 503)
			return
		}
		if err := s.game.SetDirectory(r.Context(), dir); err != nil {
			http.Error(w, "Journal selection saved but service unavailable; restart the local app", 503)
			return
		}
	default:
		http.Error(w, "Unknown journal action", 400)
		return
	}
	writeJSON(w, s.game.Details())
}
