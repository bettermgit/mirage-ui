package camouflage

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// ensureTLS loads the cover certificate, minting a self-signed ECDSA pair on
// first use: REALITY fallbacks forward raw TLS records, so the cover must speak TLS.
func ensureTLS(dir string) (*tls.Certificate, error) {
	certPath := filepath.Join(dir, "cover-cert.pem")
	keyPath := filepath.Join(dir, "cover-key.pem")
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return &cert, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "www.example.com"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(2, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"www.example.com", "example.com"},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// sniffListener answers TLS when the first byte is a handshake record (REALITY
// masters forward raw records) and plain HTTP otherwise (TLS masters forward
// already-decrypted bytes).
type sniffListener struct {
	net.Listener
	cfg *tls.Config
}

func (l *sniffListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		// Bound the peek: one idle or stalled connection must never wedge
		// the single Accept loop of the whole cover server.
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
		br := bufio.NewReader(c)
		first, err := br.Peek(1)
		_ = c.SetReadDeadline(time.Time{})
		if err != nil {
			// A scanner that closed or stalled before sending anything is
			// noise, not a listener failure: drop it, keep serving.
			_ = c.Close()
			continue
		}
		pc := &peekedConn{Conn: c, r: br}
		if first[0] == 0x16 {
			return tls.Server(pc, l.cfg), nil
		}
		return pc, nil
	}
}

type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
