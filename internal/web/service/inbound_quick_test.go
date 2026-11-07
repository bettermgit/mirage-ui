package service

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func quickTestSetup(t *testing.T) {
	t.Helper()
	folder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", folder)
	dbtest.InitDB(t, filepath.Join(folder, "x-ui.db"))
	previous := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(previous) })
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestQuickAddRealityXHTTPProducesUsableInbound(t *testing.T) {
	quickTestSetup(t)
	svc := &InboundService{}
	inbound, needRestart, err := svc.QuickAdd(1, QuickAddOptions{Template: "vless_reality_xhttp"})
	if err != nil {
		t.Fatalf("QuickAdd: %v", err)
	}
	if inbound.Id == 0 {
		t.Fatal("inbound not persisted")
	}
	_ = needRestart
	if inbound.Port < 20000 || inbound.Port > 39999 {
		t.Fatalf("port %d outside the quick-add range", inbound.Port)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		t.Fatalf("settings not valid JSON: %v", err)
	}
	clients, _ := settings["clients"].([]any)
	if len(clients) != 1 {
		t.Fatalf("want 1 client, got %d", len(clients))
	}
	id, _ := clients[0].(map[string]any)["id"].(string)
	if !uuidRe.MatchString(id) {
		t.Fatalf("client id %q is not a v4 uuid", id)
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		t.Fatalf("streamSettings not valid JSON: %v", err)
	}
	if stream["security"] != "reality" || stream["network"] != "xhttp" {
		t.Fatalf("unexpected stream: %v", stream)
	}
	reality, _ := stream["realitySettings"].(map[string]any)
	if reality["privateKey"] == "" || reality["privateKey"] == nil {
		t.Fatal("reality privateKey missing")
	}
	shortIds, _ := reality["shortIds"].([]any)
	if len(shortIds) != 1 || len(shortIds[0].(string)) != 8 {
		t.Fatalf("want one 8-hex shortId, got %v", shortIds)
	}

	var stored model.Inbound
	if err := database.GetDB().First(&stored, inbound.Id).Error; err != nil {
		t.Fatalf("row not in DB: %v", err)
	}
	if stored.Protocol != model.VLESS || stored.UserId != 1 || !stored.Enable {
		t.Fatalf("stored row wrong: %+v", stored)
	}
}

func TestQuickAddUnknownTemplateRejected(t *testing.T) {
	quickTestSetup(t)
	svc := &InboundService{}
	if _, _, err := svc.QuickAdd(1, QuickAddOptions{Template: "vless_magic"}); err == nil {
		t.Fatal("unknown template must fail")
	}
}

func TestQuickAddTLSTemplateRequiresCertFiles(t *testing.T) {
	quickTestSetup(t)
	svc := &InboundService{}
	_, _, err := svc.QuickAdd(1, QuickAddOptions{Template: "vless_tcp_tls"})
	if err == nil {
		t.Fatal("TLS template without cert files must fail")
	}
	if _, _, err := svc.QuickAdd(1, QuickAddOptions{Template: "shadowsocks"}); err != nil {
		t.Fatalf("shadowsocks template (no cert) failed: %v", err)
	}
}

func TestQuickTemplatesCatalogMatchesModelProtocols(t *testing.T) {
	allowed := map[string]bool{
		"vmess": true, "vless": true, "tunnel": true, "http": true, "trojan": true,
		"shadowsocks": true, "mixed": true, "wireguard": true, "hysteria": true,
		"mtproto": true, "amneziawg": true, "tuic": true, "masque": true,
	}
	svc := &InboundService{}
	seen := map[string]bool{}
	for _, tpl := range svc.QuickTemplates() {
		if seen[tpl.Key] {
			t.Fatalf("duplicate template key %q", tpl.Key)
		}
		seen[tpl.Key] = true
		if !allowed[tpl.Protocol] {
			t.Fatalf("template %q uses unknown protocol %q", tpl.Key, tpl.Protocol)
		}
	}
	if len(seen) < 5 {
		t.Fatalf("catalog unexpectedly small: %d", len(seen))
	}
}
