package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/camouflage"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func camouflageSetup(t *testing.T) {
	t.Helper()
	folder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", folder)
	dbtest.InitDB(t, filepath.Join(folder, "x-ui.db"))
	t.Cleanup(func() { _ = camouflage.Default.Stop() })
}

func insertMaster(t *testing.T, protocol model.Protocol, stream string) int {
	t.Helper()
	settings := `{"decryption":"none","clients":[{"id":"11111111-1111-1111-1111-111111111111","email":"master@example.test"}]}`
	if protocol == model.Trojan {
		settings = `{"clients":[{"password":"pw","email":"master@example.test"}]}`
	}
	ib := &model.Inbound{
		Enable: true, Port: 14430, Protocol: protocol,
		Tag:      fmt.Sprintf("master-%s-%d", protocol, time.Now().UnixNano()),
		Settings: settings, StreamSettings: stream,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	return ib.Id
}

func TestCamouflageApplyWiresFallbackAndServes(t *testing.T) {
	camouflageSetup(t)
	masterId := insertMaster(t, model.Trojan, `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/x.crt","keyFile":"/x.key"}]}}`)

	svc := &CamouflageService{}
	port := camouflageDefaultPort + 7
	status, err := svc.Apply(CamouflageApplyRequest{Template: "company", Port: port, MasterInboundId: masterId})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !status.Enabled || !status.Running || status.Port != port {
		t.Fatalf("unexpected status %+v", status)
	}

	var rows []model.InboundFallback
	if err := database.GetDB().Where("master_id = ?", masterId).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Dest != fmt.Sprintf("127.0.0.1:%d", port) || rows[0].Name != camouflageFallbackName {
		t.Fatalf("fallback rows wrong: %+v", rows)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/")
	if err != nil {
		t.Fatalf("cover site not serving: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cover site status %d", resp.StatusCode)
	}

	if err := svc.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	var n int64
	if err := database.GetDB().Model(&model.InboundFallback{}).
		Where("master_id = ? AND name = ?", masterId, camouflageFallbackName).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("camouflage fallback still present after disable")
	}
	cfg, err := svc.GetConfig()
	if err != nil || cfg.Enabled {
		t.Fatalf("config still enabled: %+v %v", cfg, err)
	}
}

func TestCamouflageApplyRejectsIncompatibleMaster(t *testing.T) {
	camouflageSetup(t)
	masterId := insertMaster(t, model.VLESS, `{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/x"},"tlsSettings":{"certificates":[{"certificateFile":"/x.crt","keyFile":"/x.key"}]}}`)

	svc := &CamouflageService{}
	if _, err := svc.Apply(CamouflageApplyRequest{Template: "blog", Port: camouflageDefaultPort + 8, MasterInboundId: masterId}); err == nil {
		t.Fatal("xhttp master must be rejected for fallbacks")
	}
	if st := camouflage.Default.Status(); st.Running {
		t.Fatal("server must be stopped after a failed apply")
	}
}

func TestCamouflageApplyPreservesUserFallbacks(t *testing.T) {
	camouflageSetup(t)
	masterId := insertMaster(t, model.Trojan, `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/x.crt","keyFile":"/x.key"}]}}`)
	child := insertMaster(t, model.Trojan, `{"network":"tcp","security":"tls"}`)
	database.GetDB().Create(&model.InboundFallback{MasterId: masterId, ChildId: child, Path: "/trj", SortOrder: 0})

	svc := &CamouflageService{}
	if _, err := svc.Apply(CamouflageApplyRequest{Template: "blog", Port: camouflageDefaultPort + 9, MasterInboundId: masterId}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var rows []model.InboundFallback
	if err := database.GetDB().Where("master_id = ?", masterId).Order("sort_order").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want user row + camouflage row, got %+v", rows)
	}
	if rows[0].Path != "/trj" {
		t.Fatalf("user fallback lost: %+v", rows[0])
	}
	if rows[1].Name != camouflageFallbackName {
		t.Fatalf("second row must be the camouflage fallback: %+v", rows[1])
	}

	status, err := svc.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(status.Templates)
	if len(status.Templates) < 3 {
		t.Fatalf("template catalog missing: %s", raw)
	}
}

func realityTargetOf(t *testing.T, id int) string {
	t.Helper()
	var ib model.Inbound
	if err := database.GetDB().First(&ib, id).Error; err != nil {
		t.Fatal(err)
	}
	var stream map[string]any
	if err := json.Unmarshal([]byte(ib.StreamSettings), &stream); err != nil {
		t.Fatal(err)
	}
	rs, _ := stream["realitySettings"].(map[string]any)
	target, _ := rs["target"].(string)
	return target
}

// REALITY hands every failed handshake to realitySettings.target and ignores
// inbound fallbacks, so for a REALITY master the cover site must become target.
func TestCamouflageApplyRedirectsRealityTarget(t *testing.T) {
	camouflageSetup(t)
	masterId := insertMaster(t, model.VLESS,
		`{"network":"tcp","security":"reality","realitySettings":{"target":"real.example:443","serverNames":["real.example"]}}`)

	svc := &CamouflageService{}
	port := camouflageDefaultPort + 11
	if _, err := svc.Apply(CamouflageApplyRequest{Template: "blog", Port: port, MasterInboundId: masterId}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := fmt.Sprintf("127.0.0.1:%d", port)
	if got := realityTargetOf(t, masterId); got != want {
		t.Fatalf("reality target after apply: want %s, got %s", want, got)
	}
	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RealityTarget != "real.example:443" {
		t.Fatalf("original target not stored: %+v", cfg)
	}

	// Re-apply on a new port keeps the true original stored, never the cover.
	port2 := camouflageDefaultPort + 12
	if _, err := svc.Apply(CamouflageApplyRequest{Template: "blog", Port: port2, MasterInboundId: masterId}); err != nil {
		t.Fatalf("re-Apply: %v", err)
	}
	cfg, _ = svc.GetConfig()
	if cfg.RealityTarget != "real.example:443" {
		t.Fatalf("re-apply overwrote stored target with cover: %+v", cfg)
	}

	if err := svc.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if got := realityTargetOf(t, masterId); got != "real.example:443" {
		t.Fatalf("target after disable: %s", got)
	}
	cfg, _ = svc.GetConfig()
	if cfg.RealityTarget != "" {
		t.Fatalf("stored target must clear after disable: %+v", cfg)
	}
}
