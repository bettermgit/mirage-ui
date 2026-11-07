package sub

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	yaml "github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRenderMihomoProxiesKeepsRealityExtraFields(t *testing.T) {
	proxies := []map[string]any{
		{
			"name": "pq", "type": "vless", "server": "pq.cdn.com", "port": 443,
			"uuid": "11111111-2222-4333-8444-555555555555", "network": "tcp",
			"tls": true, "servername": "pq.cdn.com", "client-fingerprint": "chrome",
			"reality-opts": map[string]any{"public-key": "PBK", "short-id": "ab", "support-x25519mlkem768": true},
		},
		{
			"name": "classic", "type": "vless", "server": "classic.cdn.com", "port": 443,
			"uuid": "11111111-2222-4333-8444-555555555556", "network": "tcp",
			"tls": true, "servername": "classic.cdn.com", "client-fingerprint": "chrome",
			"reality-opts": map[string]any{"public-key": "PBK", "short-id": "ab", "support-x25519mlkem768": false},
		},
	}
	out, err := renderMihomoProxies(proxies, defaultSubConvRenderOptions())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var doc struct {
		Proxies []struct {
			Server      string `yaml:"server"`
			RealityOpts struct {
				MLKEM *bool `yaml:"support-x25519mlkem768"`
			} `yaml:"reality-opts"`
		} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	want := map[string]*bool{"pq.cdn.com": nil, "classic.cdn.com": nil}
	yes, no := true, false
	want["pq.cdn.com"], want["classic.cdn.com"] = &yes, &no
	for _, p := range doc.Proxies {
		exp, ok := want[p.Server]
		if !ok {
			continue
		}
		if p.RealityOpts.MLKEM == nil || *p.RealityOpts.MLKEM != *exp {
			t.Fatalf("%s: support-x25519mlkem768 lost or wrong:\n%s", p.Server, out)
		}
		delete(want, p.Server)
	}
	if len(want) != 0 {
		t.Fatalf("missing proxies:\n%s", out)
	}
}

func TestRenderMihomoProxiesSizeGuard(t *testing.T) {
	fat := strings.Repeat("n", 100_000)
	proxies := make([]map[string]any, 0, 90)
	for i := range 90 {
		proxies = append(proxies, map[string]any{
			"name": fat + string(rune('a'+i%26)), "type": "vless",
			"server": "a.example.com", "port": 443, "uuid": "11111111-2222-4333-8444-555555555555",
		})
	}
	out, err := renderMihomoProxies(proxies, defaultSubConvRenderOptions())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(out) > 2*maxRenderedBytes {
		t.Fatalf("rendered size %d exceeds budget %d", len(out), 2*maxRenderedBytes)
	}
	if len(out) < 100_000 {
		t.Fatalf("rendered size %d — guard dropped everything", len(out))
	}
}

func TestLoadSubConvRenderOptions(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	t.Cleanup(reloadSubConvCache)

	opts := loadSubConvRenderOptions()
	if !opts.Emoji || opts.SourcePrefix {
		t.Fatalf("defaults: Emoji=%v SourcePrefix=%v, want true/false", opts.Emoji, opts.SourcePrefix)
	}

	insertSetting(t, "subConvConf", `{not json`)
	reloadSubConvCache()
	if opts := loadSubConvRenderOptions(); !opts.Emoji {
		t.Fatalf("malformed stored JSON must fall back to defaults")
	}

	insertSetting(t, "subConvConf", `{"emoji":false,"template":"minimal"}`)
	reloadSubConvCache()
	opts = loadSubConvRenderOptions()
	if opts.Emoji || opts.Template != "minimal" {
		t.Fatalf("stored config not applied: emoji=%v template=%q", opts.Emoji, opts.Template)
	}
}

func reloadSubConvCache() {
	subConvCache.mu.Lock()
	subConvCache.loadedAt = time.Time{}
	subConvCache.mu.Unlock()
}

func insertSetting(t *testing.T, key, value string) {
	t.Helper()
	if err := database.GetDB().Where("key = ?", key).Delete(&model.Setting{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
		t.Fatal(err)
	}
}
