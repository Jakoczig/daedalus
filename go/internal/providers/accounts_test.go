package providers

import (
	"context"
	"database/sql"
	"errors"
	"github.com/Jakoczig/eridion/go/internal/history"
	"path/filepath"
	"testing"
)

func TestAccountSecretsConsentAndInflightGenerationAreIsolated(t *testing.T) {
	v := &testVault{}
	m := New(context.Background(), t.TempDir(), Options{Vault: v, Manual: true})
	defer m.Close()
	if e := m.ActivateAccount("FIXTURE-A", false); e != nil {
		t.Fatal(e)
	}
	if e := m.SetCredential("edsm", Credential{APIKey: "A-SECRET", Commander: "A"}); e != nil {
		t.Fatal(e)
	}
	if e := m.Configure("edsm", Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	g := m.states["edsm"].generation
	if e := m.ActivateAccount("FIXTURE-B", false); e != nil {
		t.Fatal(e)
	}
	if m.secrets["edsm"].APIKey != "" || m.states["edsm"].config.Read || m.states["edsm"].generation == g {
		t.Fatal("cross-account credential/consent/generation")
	}
	if e := m.ActivateAccount("FIXTURE-A", false); e != nil {
		t.Fatal(e)
	}
	if m.secrets["edsm"].APIKey != "A-SECRET" || !m.states["edsm"].config.Read {
		t.Fatal("account not restored")
	}
}

func TestAccountRestartRetainsNamespacedVaultAndCanonicalConsent(t *testing.T) {
	db, e := history.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	dir := t.TempDir()
	v := &testVault{}
	m := New(context.Background(), dir, Options{Vault: v, Manual: true})
	if e = m.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-A", Name: "A"}); e != nil {
		t.Fatal(e)
	}
	if e = m.SetCredential("edsm", Credential{APIKey: "A-PRIVATE", Commander: "A"}); e != nil {
		t.Fatal(e)
	}
	if e = m.Configure("edsm", Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	if e = m.ActivateAccount("FIXTURE-B", false); e != nil {
		t.Fatal(e)
	}
	m.Close()
	next := New(context.Background(), dir, Options{Vault: v, Manual: true})
	defer next.Close()
	if e = next.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-B", Name: "B"}); e != nil {
		t.Fatal(e)
	}
	if next.secrets["edsm"].APIKey != "" || next.states["edsm"].config.Read {
		t.Fatal("B inherited A")
	}
	if e = next.ActivateAccount("FIXTURE-A", false); e != nil {
		t.Fatal(e)
	}
	if next.secrets["edsm"].APIKey != "A-PRIVATE" || !next.states["edsm"].config.Read {
		t.Fatalf("A credential/consent lost after restart: credential=%t read=%t accounts=%v", next.secrets["edsm"].APIKey != "", next.states["edsm"].config.Read, next.accountConfigs)
	}
}

func TestLegacyCredentialsAreNotAssignedByLastSeenFIDGuess(t *testing.T) {
	db, e := history.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	v := &testVault{v: map[string]Credential{"frontier": {AccessToken: "UNASSOCIATED", RefreshToken: "UNASSOCIATED"}, "edsm": {APIKey: "OTHER-KEY", Commander: "Other"}, "inara": {APIKey: "MATCHING-KEY", Commander: "Verified"}}}
	m := New(context.Background(), t.TempDir(), Options{Vault: v, Manual: true})
	defer m.Close()
	if e = m.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-A", Name: "Verified"}); e != nil {
		t.Fatal(e)
	}
	if m.secrets["frontier"].AccessToken != "" || m.secrets["edsm"].APIKey != "" || m.secrets["inara"].APIKey != "MATCHING-KEY" {
		t.Fatal("ambiguous legacy credentials assigned")
	}
	if v.v["frontier"].AccessToken != "UNASSOCIATED" || v.v["edsm"].APIKey != "OTHER-KEY" {
		t.Fatal("unassociated encrypted credentials discarded")
	}
}

type unavailableSaveVault struct{}

func (*unavailableSaveVault) Load() (map[string]Credential, error) {
	return map[string]Credential{}, nil
}
func (*unavailableSaveVault) Save(map[string]Credential) error {
	return errors.New("Secret Service unavailable")
}
func TestOfflineAccountAdoptionDoesNotCreateAnEmptyOSVault(t *testing.T) {
	db, e := history.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	m := New(context.Background(), t.TempDir(), Options{Vault: &unavailableSaveVault{}, Manual: true})
	defer m.Close()
	if e = m.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-A", Name: "Fixture"}); e != nil {
		t.Fatal("Offline cached account unnecessarily required a writable vault", e)
	}
}

func TestLegacyUploadConsentStaysWithItsOriginalCommander(t *testing.T) {
	db, e := history.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	original := []byte(`{"providers":{"eddn":{"upload":true,"categories":["travel"],"identity":"` + hash("FIXTURE-A") + `"}},"ledger":{}}`)
	if _, e = db.LoadProviderConsent(original); e != nil {
		t.Fatal(e)
	}
	m := New(context.Background(), t.TempDir(), Options{Vault: &testVault{}, Manual: true})
	defer m.Close()
	if e = m.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-B", Name: "B"}); e != nil {
		t.Fatal(e)
	}
	if m.states["eddn"].config.Upload {
		t.Fatal("last-seen B inherited A's upload consent")
	}
	if e = m.ActivateAccount("FIXTURE-A", false); e != nil {
		t.Fatal(e)
	}
	if c := m.states["eddn"].config; !c.Upload || c.CommanderHash != hash("FIXTURE-A") {
		t.Fatal("original A publication permission was abandoned")
	}
}

func TestHistoryRestoreUsesPinnedAccountInsteadOfCachedReaderAccount(t *testing.T) {
	db, e := history.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	m := New(context.Background(), t.TempDir(), Options{Vault: &testVault{}, Manual: true})
	defer m.Close()
	m.PinAccount("FIXTURE-B")
	if e = m.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-A", Name: "A"}); e != nil {
		t.Fatal(e)
	}
	if m.accountFID != "FIXTURE-B" || m.fid != "FIXTURE-A" {
		t.Fatal("provider namespace did not remain selected B while reader restores A")
	}
	if e = m.ActivateAccount("FIXTURE-A", false); e == nil {
		t.Fatal("stale caller replaced the pinned provider account")
	}
}

func TestAccountConsentFailureRetainsNamespaceAndRetry(t *testing.T) {
	dir := t.TempDir()
	db, e := history.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	v := &testVault{}
	m := New(context.Background(), dir, Options{Vault: v, Manual: true})
	defer m.Close()
	if e = m.AttachHistory(db, history.ProviderSeed{FID: "FIXTURE-A", Name: "A"}); e != nil {
		t.Fatal(e)
	}
	if e = m.SetCredential("edsm", Credential{APIKey: "A-PRIVATE", Commander: "A"}); e != nil {
		t.Fatal(e)
	}
	if e = m.Configure("edsm", Config{Read: true}); e != nil {
		t.Fatal(e)
	}
	oldCtx := m.states["edsm"].ctx
	generation := m.states["edsm"].generation
	m.states["edsm"].queue = []queued{{key: "cancel-me", generation: generation}}
	sqlDB, e := sql.Open("sqlite3", filepath.Join(dir, "daedalus.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer sqlDB.Close()
	if _, e = sqlDB.Exec(`CREATE TRIGGER fail_consent BEFORE INSERT ON app_state WHEN NEW.key='provider-ledger' BEGIN SELECT RAISE(FAIL,'injected consent failure'); END`); e != nil {
		t.Fatal(e)
	}
	if e = m.ActivateAccount("FIXTURE-B", false); e == nil {
		t.Fatal("injected save succeeded")
	}
	if m.accountFID != "FIXTURE-A" || m.opts.Vault.(*scopedVault).scope != hash("FIXTURE-A")+"/" || m.secrets["edsm"].APIKey != "A-PRIVATE" || !m.states["edsm"].config.Read {
		t.Fatal("failed activation left namespace, credential or consent inconsistent")
	}
	if oldCtx.Err() == nil || m.states["edsm"].generation <= generation || len(m.states["edsm"].queue) != 0 {
		t.Fatal("failed switch resurrected a canceled generation/queue")
	}
	if _, e = sqlDB.Exec(`DROP TRIGGER fail_consent`); e != nil {
		t.Fatal(e)
	}
	if e = m.ActivateAccount("FIXTURE-B", false); e != nil {
		t.Fatal("retry", e)
	}
	if m.accountFID != "FIXTURE-B" || m.secrets["edsm"].APIKey != "" || m.states["edsm"].config.Read {
		t.Fatal("retry did not isolate B")
	}
	if e = m.ActivateAccount("FIXTURE-A", false); e != nil {
		t.Fatal(e)
	}
	if m.secrets["edsm"].APIKey != "A-PRIVATE" || !m.states["edsm"].config.Read || len(m.states["edsm"].queue) != 0 {
		t.Fatal("A state not restored or queue resurrected")
	}
}
