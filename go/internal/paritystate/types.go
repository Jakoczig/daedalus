// Package paritystate projects bounded, explicit journal and sidecar facts.
package paritystate

import "github.com/Jakoczig/eridion/go/internal/shipstate"

type View struct {
	Evidence     string         `json:"evidence"`
	Ship         shipstate.View `json:"ship"`
	Modules      []Module       `json:"modules"`
	ModulesState string         `json:"modulesState"`
	Cargo        CargoView      `json:"cargo"`
	Inventory    InventoryView  `json:"inventory"`
	Materials    MaterialView   `json:"materials"`
	Status       StatusView     `json:"status"`
}
type Module struct {
	Slot         string           `json:"slot"`
	Item         string           `json:"item"`
	Name         string           `json:"name"`
	Health       *float64         `json:"health"`
	On           *bool            `json:"on"`
	Priority     *int             `json:"priority"`
	AmmoInClip   *int             `json:"ammoInClip"`
	AmmoInHopper *int             `json:"ammoInHopper"`
	Engineering  *Engineering     `json:"engineering"`
	Reference    *ModuleReference `json:"reference"`
}
type ModuleReference struct {
	Source      string   `json:"source"`
	Class       *int     `json:"class"`
	Rating      string   `json:"rating"`
	MassT       *float64 `json:"massT"`
	PowerMW     *float64 `json:"powerMW"`
	Description string   `json:"description"`
}
type Engineering struct {
	Blueprint    string     `json:"blueprint"`
	Grade        *int       `json:"grade"`
	Quality      *float64   `json:"quality"`
	Experimental string     `json:"experimental"`
	Modifiers    []Modifier `json:"modifiers"`
	Truncated    bool       `json:"truncated"`
}
type Modifier struct {
	Label         string   `json:"label"`
	Value         *float64 `json:"value"`
	OriginalValue *float64 `json:"originalValue"`
	LessIsGood    *bool    `json:"lessIsGood"`
}
type CargoView struct {
	State     string      `json:"state"`
	Count     *int        `json:"count"`
	Items     []CargoItem `json:"items"`
	Truncated bool        `json:"truncated"`
}
type CargoItem struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Count   int    `json:"count"`
	Stolen  *int   `json:"stolen"`
	Mission bool   `json:"mission"`
}
type InventoryView struct {
	ShipLockerState string          `json:"shipLockerState"`
	BackpackState   string          `json:"backpackState"`
	State           string          `json:"state"`
	Items           []InventoryItem `json:"items"`
	Truncated       bool            `json:"truncated"`
}
type InventoryItem struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Location string `json:"location"`
	Count    int    `json:"count"`
}
type MaterialView struct {
	State     string         `json:"state"`
	Items     []MaterialItem `json:"items"`
	Truncated bool           `json:"truncated"`
}
type MaterialItem struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Count    int    `json:"count"`
}
type Pips struct {
	Sys float64 `json:"sys"`
	Eng float64 `json:"eng"`
	Wep float64 `json:"wep"`
}
type StatusView struct {
	State          string        `json:"state"`
	ObservedAt     string        `json:"observedAt"`
	Flags          *StatusFlags  `json:"flags"`
	Flags2         *StatusFlags2 `json:"flags2"`
	Pips           *Pips         `json:"pips"`
	FuelMainT      *float64      `json:"fuelMainT"`
	FuelReservoirT *float64      `json:"fuelReservoirT"`
	CargoT         *float64      `json:"cargoT"`
	Temperature    *float64      `json:"temperature"`
	GuiFocus       *int          `json:"guiFocus"`
}
type StatusFlags2 struct {
	OnFoot                bool `json:"onFoot"`
	InTaxi                bool `json:"inTaxi"`
	InMulticrew           bool `json:"inMulticrew"`
	LowOxygen             bool `json:"lowOxygen"`
	LowHealth             bool `json:"lowHealth"`
	Cold                  bool `json:"cold"`
	Hot                   bool `json:"hot"`
	VeryCold              bool `json:"veryCold"`
	VeryHot               bool `json:"veryHot"`
	GlideMode             bool `json:"glideMode"`
	BreathableAtmosphere  bool `json:"breathableAtmosphere"`
	TelepresenceMulticrew bool `json:"telepresenceMulticrew"`
	PhysicalMulticrew     bool `json:"physicalMulticrew"`
	FSDHyperdriveCharging bool `json:"fsdHyperdriveCharging"`
}
type StatusFlags struct {
	Docked             bool `json:"docked"`
	Landed             bool `json:"landed"`
	ShieldsUp          bool `json:"shieldsUp"`
	Supercruise        bool `json:"supercruise"`
	InMainShip         bool `json:"inMainShip"`
	InFighter          bool `json:"inFighter"`
	InSRV              bool `json:"inSRV"`
	LightsOn           bool `json:"lightsOn"`
	NightVision        bool `json:"nightVision"`
	CargoScoopDeployed bool `json:"cargoScoopDeployed"`
	LandingGearDown    bool `json:"landingGearDown"`
	HardpointsDeployed bool `json:"hardpointsDeployed"`
	Overheating        bool `json:"overheating"`
	BeingInterdicted   bool `json:"beingInterdicted"`
	InDanger           bool `json:"inDanger"`
	LowFuel            bool `json:"lowFuel"`
	FSDMassLocked      bool `json:"fsdMassLocked"`
	FSDCooldown        bool `json:"fsdCooldown"`
	FSDCharging        bool `json:"fsdCharging"`
	FSDJump            bool `json:"fsdJump"`
	FlightAssistOff    bool `json:"flightAssistOff"`
	SilentRunning      bool `json:"silentRunning"`
	ScoopingFuel       bool `json:"scoopingFuel"`
	GlideMode          bool `json:"glideMode"`
	AnalysisMode       bool `json:"analysisMode"`
}
