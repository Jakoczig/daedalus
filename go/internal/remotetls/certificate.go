package remotetls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/fs"
	"math/big"
	"net"
	"time"

	"github.com/Jakoczig/daedalus/go/internal/privatefs"
)

type Bundle struct {
	Certificate tls.Certificate
	CAPEM       []byte
	Fingerprint string
	Expires     time.Time
}
type identity struct{ CA, CAKey, Leaf, LeafKey []byte }

func key() ([]byte, *ecdsa.PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(k)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), k, err
}
func serial() (*big.Int, error) { return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128)) }
func certificate(template, parent *x509.Certificate, public any, signer any) ([]byte, error) {
	der, err := x509.CreateCertificate(rand.Reader, template, parent, public, signer)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func Load(dir string, ip net.IP, now time.Time) (Bundle, error) {
	var result Bundle
	if ip == nil {
		return result, errors.New("invalid certificate address")
	}
	if err := privatefs.Prepare(dir); err != nil {
		return result, err
	}
	var saved identity
	data, err := privatefs.Read(dir, "tls.json")
	if errors.Is(err, fs.ErrNotExist) {
		if _, markerErr := privatefs.Read(dir, "tls.initialized"); !errors.Is(markerErr, fs.ErrNotExist) {
			return result, errors.New("initialized TLS identity is missing")
		}
		// Write the marker before secrets: a partial initialization must fail closed.
		if err = privatefs.Write(dir, "tls.initialized", []byte("1")); err != nil {
			return result, err
		}
		pemKey, k, err := key()
		if err != nil {
			return result, err
		}
		id, err := serial()
		if err != nil {
			return result, err
		}
		root := &x509.Certificate{SerialNumber: id, Subject: pkix.Name{CommonName: "DAEDALUS private display CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, MaxPathLenZero: true}
		saved.CA, err = certificate(root, root, &k.PublicKey, k)
		if err != nil {
			return result, err
		}
		saved.CAKey = pemKey
	} else if err != nil {
		return result, err
	} else if json.Unmarshal(data, &saved) != nil {
		return result, errors.New("invalid TLS identity")
	}
	rootBlock, _ := pem.Decode(saved.CA)
	keyBlock, _ := pem.Decode(saved.CAKey)
	if rootBlock == nil || keyBlock == nil {
		return result, errors.New("invalid TLS identity")
	}
	root, err := x509.ParseCertificate(rootBlock.Bytes)
	if err != nil || !root.IsCA || now.Before(root.NotBefore) || !now.Before(root.NotAfter) {
		return result, errors.New("expired or invalid display CA")
	}
	signer, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return result, errors.New("invalid display CA key")
	}
	caPair, err := tls.X509KeyPair(saved.CA, saved.CAKey)
	if err != nil || caPair.PrivateKey == nil {
		return result, errors.New("display CA key mismatch")
	}
	pair, pairErr := tls.X509KeyPair(saved.Leaf, saved.LeafKey)
	renew := pairErr != nil
	if !renew {
		leaf, e := x509.ParseCertificate(pair.Certificate[0])
		roots := x509.NewCertPool()
		roots.AddCert(root)
		if e != nil {
			renew = true
		} else {
			_, e = leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: ip.String(), CurrentTime: now})
			renew = e != nil || now.Add(7*24*time.Hour).After(leaf.NotAfter)
		}
	}
	if renew {
		pemKey, k, err := key()
		if err != nil {
			return result, err
		}
		id, err := serial()
		if err != nil {
			return result, err
		}
		until := now.Add(90 * 24 * time.Hour)
		if until.After(root.NotAfter) {
			until = root.NotAfter
		}
		leaf := &x509.Certificate{SerialNumber: id, Subject: pkix.Name{CommonName: "DAEDALUS secure displays"}, NotBefore: now.Add(-time.Minute), NotAfter: until, IPAddresses: []net.IP{ip}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
		saved.Leaf, err = certificate(leaf, root, &k.PublicKey, signer)
		if err != nil {
			return result, err
		}
		saved.LeafKey = pemKey
		data, err = json.Marshal(saved)
		if err != nil {
			return result, err
		}
		if err = privatefs.Write(dir, "tls.json", data); err != nil {
			return result, err
		}
		pair, err = tls.X509KeyPair(saved.Leaf, saved.LeafKey)
		if err != nil {
			return result, err
		}
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return result, err
	}
	hash := sha256.Sum256(rootBlock.Bytes)
	return Bundle{Certificate: pair, CAPEM: saved.CA, Fingerprint: hex.EncodeToString(hash[:]), Expires: leaf.NotAfter}, nil
}
