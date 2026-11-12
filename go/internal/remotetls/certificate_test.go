package remotetls

import (
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/privatefs"
)

func TestCertificateTrustAndRenewal(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	ip := net.ParseIP("127.0.0.1")
	first, err := Load(dir, ip, now)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(first.CAPEM) {
		t.Fatal("missing public root")
	}
	verify := func(bundle Bundle, address string, at time.Time) error {
		leaf, err := x509.ParseCertificate(bundle.Certificate.Certificate[0])
		if err != nil {
			return err
		}
		_, err = leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: address, CurrentTime: at})
		return err
	}
	if verify(first, "127.0.0.1", now) != nil || verify(first, "127.0.0.2", now) == nil {
		t.Fatal("SAN verification broken")
	}
	again, err := Load(dir, ip, now)
	if err != nil || again.Fingerprint != first.Fingerprint || string(again.Certificate.Certificate[0]) != string(first.Certificate.Certificate[0]) {
		t.Fatal("identity changed on reload", err)
	}
	renewed, err := Load(dir, ip, now.Add(85*24*time.Hour))
	if err != nil || renewed.Fingerprint != first.Fingerprint || verify(renewed, "127.0.0.1", now.Add(85*24*time.Hour)) != nil {
		t.Fatal("renewal broke trust", err)
	}
	if err := privatefs.Write(dir, "tls.json", []byte(`{"broken":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(dir, ip, now); err == nil {
		t.Fatal("corrupt identity silently replaced")
	}
}
