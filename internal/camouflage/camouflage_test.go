package camouflage

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestDeployWritesTemplateAndMarker(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("blog", dir); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	page, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("index.html missing: %v", err)
	}
	if len(page) < 200 {
		t.Fatalf("index.html suspiciously small: %d bytes", len(page))
	}
	if _, err := os.Stat(filepath.Join(dir, MarkerFile)); err != nil {
		t.Fatalf("marker file missing: %v", err)
	}
	if err := Deploy("nope", dir); err == nil {
		t.Fatal("unknown template must fail")
	}
}

func TestDeployRefusesForeignDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Deploy("blog", dir); err == nil {
		t.Fatal("deploy into a hand-built site dir must fail")
	}
}

func TestListContainsBundledTemplates(t *testing.T) {
	keys := map[string]bool{}
	for _, tpl := range List() {
		keys[tpl.Key] = true
	}
	for _, want := range []string{"blog", "company", "gallery"} {
		if !keys[want] {
			t.Fatalf("template %q not embedded (got %v)", want, keys)
		}
	}
}

func TestServerServesFilesAndRedirect(t *testing.T) {
	dir := t.TempDir()
	if err := Deploy("gallery", dir); err != nil {
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
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	port2 := freePort(t)
	if err := srv.Start(dir, port2, "https://example.org/x"); err != nil {
		t.Fatalf("Start redirect: %v", err)
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       3 * time.Second,
	}
	resp2, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port2) + "/anything")
	if err != nil {
		t.Fatalf("GET redirect: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusFound || resp2.Header.Get("Location") != "https://example.org/x" {
		t.Fatalf("want 302 -> https://example.org/x, got %d %q", resp2.StatusCode, resp2.Header.Get("Location"))
	}
}

func TestServerRestartOnSamePort(t *testing.T) {
	dirA := t.TempDir()
	if err := Deploy("blog", dirA); err != nil {
		t.Fatal(err)
	}
	dirB := t.TempDir()
	if err := Deploy("company", dirB); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })
	port := freePort(t)

	if err := srv.Start(dirA, port, ""); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Re-applying on the same port must release the prior bind, not collide with it.
	if err := srv.Start(dirB, port, ""); err != nil {
		t.Fatalf("rebind same port failed: %v", err)
	}
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("GET after rebind: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d after rebind", resp.StatusCode)
	}
}

func TestServerServesCustom404(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>home</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "404.html"), []byte("<html>custom missing</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })
	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/definitely-not-here")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 with custom page, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "custom missing") {
		t.Fatalf("custom 404 page not served: %s", body)
	}
}

func TestServerSPAIndexWhenNoCustom404(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>home only</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := &Server{}
	t.Cleanup(func() { _ = srv.Stop() })
	port := freePort(t)
	if err := srv.Start(dir, port, ""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/whatever")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("no-404.html deploy must keep index fallback, got %d", resp.StatusCode)
	}
}

// Every bundled template must be self-consistent: deployed sites carry the
// realism assets their pages reference (favicon, robots, 404).
func TestTemplatesCarryFaviconRobotsAnd404(t *testing.T) {
	for _, tpl := range List() {
		t.Run(tpl.Key, func(t *testing.T) {
			dir := t.TempDir()
			if err := Deploy(tpl.Key, dir); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"index.html", "404.html", "robots.txt", "favicon.svg"} {
				info, err := os.Stat(filepath.Join(dir, name))
				if err != nil {
					t.Fatalf("%s missing: %v", name, err)
				}
				if info.Size() == 0 {
					t.Fatalf("%s is empty", name)
				}
			}
			page, err := os.ReadFile(filepath.Join(dir, "index.html"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(page), "favicon.svg") {
				t.Fatal("index.html does not reference favicon.svg")
			}
		})
	}
}
