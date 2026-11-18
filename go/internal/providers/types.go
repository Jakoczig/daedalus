// Package providers owns optional external traffic. It never owns a journal reader.
package providers

import (
	"context"
	"net/http"
	"time"
)

var InaraApplicationName string                               // set only after application whitelisting
var FrontierClientID = "3a7cd51a-f629-4fec-b0c7-967bb7e2fa53" // public client identifier; never a shared secret
const FrontierRedirectURI = "https://eridion.fitoserver.org/auth/frontier/callback"
const software = "ERIDION"
const version = "2.2.0"

var ids = []string{"edsm", "inara", "edastro", "eddn", "frontier", "spansh", "elitebgs", "raven", "canonn", "edsm-reference"}

type Credential struct {
	JournalFID   string    `json:"journalFid,omitempty"`
	APIKey       string    `json:"apiKey,omitempty"`
	Commander    string    `json:"commander,omitempty"`
	AccessToken  string    `json:"accessToken,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}
type Vault interface {
	Load() (map[string]Credential, error)
	Save(map[string]Credential) error
}
type Options struct {
	RequireHistory                 bool
	Client                         *http.Client
	Vault                          Vault
	Now                            func() time.Time
	Manual                         bool
	InaraAppName, FrontierClientID string
}
type Config struct {
	CommunityGoals bool     `json:"communityGoals"`
	Read           bool     `json:"read"`
	Upload         bool     `json:"upload"`
	Subscribe      bool     `json:"subscribe"`
	Paused         bool     `json:"paused"`
	Categories     []string `json:"categories"`
	CommanderHash  string   `json:"-"`
}
type storedConfig struct {
	Config
	Identity string `json:"identity,omitempty"`
}
type Capability struct {
	Evidence        string   `json:"evidence"`
	Protocol        string   `json:"protocol"`
	EventTypes      []string `json:"eventTypes"`
	ID              string   `json:"id"`
	Endpoint        string   `json:"endpoint"`
	Documentation   string   `json:"documentation"`
	Read            []string `json:"read"`
	Upload          []string `json:"upload"`
	Subscribe       []string `json:"subscribe"`
	IntervalSeconds int      `json:"intervalSeconds"`
	Gate            string   `json:"gate"`
	Scopes          []string `json:"scopes"`
}
type Status struct {
	NoNewObservations uint64            `json:"noNewObservations"`
	Capability        Capability        `json:"capability"`
	Config            Config            `json:"config"`
	State             string            `json:"state"`
	CredentialStored  bool              `json:"credentialStored"`
	Queue             int               `json:"queue"`
	Sent              uint64            `json:"sent"`
	Dropped           uint64            `json:"dropped"`
	LastSuccess       string            `json:"lastSuccess"`
	LastError         string            `json:"lastError"`
	RetryAt           string            `json:"retryAt"`
	Counts            map[string]uint64 `json:"counts"`
	Snapshot          *Snapshot         `json:"snapshot,omitempty"`
}
type Point struct {
	SystemName  string     `json:"systemName,omitempty"`
	Name        string     `json:"name"`
	Coordinates [3]float64 `json:"coordinates"`
	Category    string     `json:"category"`
	Source      string     `json:"source"`
}
type Layers struct {
	Points    []Point `json:"points"`
	Source    string  `json:"source"`
	FetchedAt string  `json:"fetchedAt"`
	ExpiresAt string  `json:"expiresAt"`
	Truncated bool    `json:"truncated"`
}
type Fact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Snapshot struct {
	ActiveCommander bool `json:"activeCommander"`
	commanderHash   string
	Source          string `json:"source"`
	FetchedAt       string `json:"fetchedAt"`
	ExpiresAt       string `json:"expiresAt"`
	Facts           []Fact `json:"facts"`
}
type queued struct {
	candidateID           string
	key, category         string
	body                  []byte
	endpoint, contentType string
	generation            uint64
	attempts              int
}
type providerState struct {
	wake                   chan struct{}
	config                 Config
	queue                  []queued
	generation             uint64
	ctx                    context.Context
	cancel                 context.CancelFunc
	next                   time.Time
	sent, dropped          uint64
	noNewObservations      uint64
	noNewLast              bool
	lastSuccess, lastError string
	counts                 map[string]uint64
	snapshot               *Snapshot
	busy                   bool
}

func capabilities() []Capability {
	return []Capability{
		{ID: "edsm-reference", Protocol: "Public sphere/station reference GETs", Endpoint: "https://www.edsm.net", Documentation: "https://www.edsm.net/en/api-system-v1", Read: []string{"nearby", "stations", "market", "shipyard", "outfitting"}, IntervalSeconds: 60, Scopes: []string{"Owner-selected system; verified locked-coordinate candidates; no fan-out; separate account/map consent"}},
		{ID: "elitebgs", Protocol: "V5 bounded named GET pages", Endpoint: "https://elitebgs.app/api/ebgs/v5", Documentation: "https://elitebgs.app/ebgs/docs/V5/paths/systems", Read: []string{"systems", "factions", "stations", "ticks"}, IntervalSeconds: 60, Scopes: []string{"Community observations; no causal attribution; no uploads"}},
		{ID: "raven", Protocol: "Documented project GET only", Endpoint: "https://ravencolonial100-awcbdvabgze4c5cq.canadacentral-01.azurewebsites.net", Documentation: "https://ravencolonial100-awcbdvabgze4c5cq.canadacentral-01.azurewebsites.net/About", Read: []string{"system", "project", "last", "stats"}, IntervalSeconds: 60, Scopes: []string{"Optional read; writes unavailable pending canonical-host/authentication approval"}},
		{ID: "canonn", Protocol: "CAPIv2 filtered site collections", Endpoint: "https://api.canonn.tech", Documentation: "https://docs.canonn.tech/home/about.html", Read: []string{"guardian", "thargoid", "biology", "generation"}, IntervalSeconds: 60, Scopes: []string{"Exact system.id64 filter; 32 sites/page; public science reference; no uploads"}},
		{ID: "spansh", Protocol: "OpenAPI 3.3.0 GET lookups only", Endpoint: "https://spansh.co.uk/api", Documentation: "https://docs.spansh.co.uk/", Read: []string{"system", "body", "station", "dump"}, IntervalSeconds: 60, Scopes: []string{"Public reference only; owner requested exact ID; no route job API"}},
		{ID: "edsm", Protocol: "journal-v1 / logs-v1", EventTypes: []string{"FSDJump", "Location", "Docked", "CarrierJump", "Scan", "SAASignalsFound", "FSSDiscoveryScan", "Loadout", "Materials", "Rank", "Progress"}, Endpoint: "https://www.edsm.net/api-journal-v1", Documentation: "https://www.edsm.net/en/api-journal-v1", Read: []string{"flightlog"}, Upload: []string{"travel", "exploration", "loadout", "materials", "ranks"}, IntervalSeconds: 60, Scopes: []string{"personal API key; commander-bound"}},
		{ID: "inara", Protocol: "inapi/v1", EventTypes: []string{"getCommanderProfile", "addCommanderTravelFSDJump", "setCommanderShipLoadout", "setCommanderInventoryMaterials", "setCommanderRankPilot", "setCommanderCredits"}, Endpoint: "https://inara.cz/inapi/v1/", Documentation: "https://inara.cz/elite/inara-api-docs/", Read: []string{"profile", "communitygoals"}, Upload: []string{"travel", "loadout", "materials", "ranks", "credits"}, IntervalSeconds: 60, Gate: "application approval required", Scopes: []string{"personal API key; whitelisted application"}},
		{ID: "edastro", Protocol: "documented unversioned journal / GEC API", EventTypes: []string{"ScanOrganic", "CarrierStats", "CarrierJumpRequest"}, Endpoint: "https://edastro.com/api/journal", Documentation: "https://edastro.com/api-details.html", Read: []string{"system", "gec"}, Upload: []string{"biology", "carrier", "carrier_destination"}, IntervalSeconds: 5, Scopes: []string{"selected public events; carrier categories independently opt in"}},
		{ID: "eddn", Protocol: "journal/1 (live schema)", EventTypes: []string{"FSDJump", "Location", "CarrierJump", "Scan", "SAASignalsFound"}, Endpoint: "https://eddn.edcd.io:4430/upload/", Documentation: "https://github.com/EDCD/EDDN/tree/live", Upload: []string{"travel", "exploration", "commodities", "shipyard", "outfitting"}, IntervalSeconds: 2, Scopes: []string{"journal/1 live public observations; anonymous installation uploader"}},
		{ID: "frontier", Protocol: "OAuth2 PKCE / read-only CAPI", EventTypes: []string{"profile", "market", "shipyard", "fleetcarrier"}, Endpoint: "https://companion.orerve.net", Documentation: "https://github.com/EDCD/FDevIDs/tree/master/Frontier%20API", Read: []string{"profile", "market", "shipyard", "fleetcarrier", "communitygoals"}, IntervalSeconds: 60, Gate: "Frontier developer approval required", Scopes: []string{"auth", "capi"}},
	}
}
