package camouflage

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// A public cover port is scanned constantly; a probe that connects and closes
// without sending must end that connection, not the whole server.
func TestServerSurvivesEmptyProbes(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("blog", dir); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })
	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	for i := 0; i < 5; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		_ = c.Close()
	}
	time.Sleep(150 * time.Millisecond)
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("server died after empty probes: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d after empty probes", resp.StatusCode)
	}
}

// Accept must not block forever on one idle connection: peek has a deadline,
// after which the server keeps accepting the next visitor.
func TestServerIdleConnectionDoesNotWedgeAccept(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("blog", dir); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })
	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	idle, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("idle connection wedged Accept: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// If Serve dies without Stop/rebind (a crash, a killed fd), the cover must
// come back on its own — REALITY keeps forwarding visitors to it regardless.
func TestServerRestartsWithoutManualIntervention(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("blog", dir); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })
	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	addr := "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	resp, err := http.Get(addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	srv.mu.Lock()
	ln := srv.listener
	srv.mu.Unlock()
	_ = ln.Close() // simulate an uncontrolled death of the accept loop

	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(addr)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("cover never self-healed (last err %v)", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// A REALITY fallback forwards the raw TLS record stream to the cover site, so
// one port must answer both TLS (REALITY master) and plaintext (TLS master).
func TestServerServesTLSAndPlaintext(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("blog", dir); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })

	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	base := "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	resp, err := http.Get(base)
	if err != nil {
		t.Fatalf("plain GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("plain status %d", resp.StatusCode)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	tresp, err := client.Get("https://127.0.0.1:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("tls GET: %v", err)
	}
	defer func() { _ = tresp.Body.Close() }()
	if tresp.StatusCode != http.StatusOK {
		t.Fatalf("tls status %d", tresp.StatusCode)
	}
	if tresp.TLS == nil || tresp.TLS.Version == 0 {
		t.Fatal("no negotiated TLS version")
	}
}

func TestServerTLSCertificatePersists(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("blog", dir); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })

	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	dial := func() string {
		conn, err := tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port), &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("tls dial: %v", err)
		}
		defer func() { _ = conn.Close() }()
		certs := conn.ConnectionState().PeerCertificates
		if len(certs) == 0 {
			t.Fatal("no peer certificate")
		}
		return certs[0].SerialNumber.String()
	}
	first := dial()

	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("re-Start: %v", err)
	}
	if second := dial(); second != first {
		t.Fatalf("certificate serial changed across restart: %s -> %s", first, second)
	}
}
