package sub

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	engmodel "github.com/mhsanaei/3x-ui/v3/internal/subconvengine/model"
)

func seedWorkbenchClient(t *testing.T, port int, email, subId, uuid string) {
	t.Helper()
	db := database.GetDB()
	ib := &model.Inbound{
		UserId:         1,
		Tag:            "wb-" + subId,
		Remark:         "wb-" + subId,
		Enable:         true,
		Port:           port,
		Protocol:       model.VLESS,
		Settings:       fmt.Sprintf(`{"clients":[{"id":%q,"email":%q,"subId":%q,"enable":true}]}`, uuid, email, subId),
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	rec := &model.ClientRecord{Email: email, SubID: subId, UUID: uuid, Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{InboundId: ib.Id, ClientId: rec.Id}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCollectPanelProxiesSelectsBySubId(t *testing.T) {
	setupInfoNodeTestDB(t)
	seedWorkbenchClient(t, 443, "a@x", "sub-a", "uuid-aaaa")
	seedWorkbenchClient(t, 444, "b@x", "sub-b", "uuid-bbbb")

	clash := NewSubClashService(NewSubService("{{EMAIL}}"))

	both, err := clash.CollectPanelProxies([]string{"sub-a", "sub-b"}, "sub.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyUUIDs(both); len(got) != 2 || !slices.Contains(got, "uuid-aaaa") || !slices.Contains(got, "uuid-bbbb") {
		t.Fatalf("expected both client proxies, got %v", got)
	}

	one, err := clash.CollectPanelProxies([]string{"sub-b"}, "sub.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyUUIDs(one); len(got) != 1 || !slices.Contains(got, "uuid-bbbb") {
		t.Fatalf("expected only sub-b proxy, got %v", got)
	}
}

func proxyUUIDs(proxies []map[string]any) []string {
	parts := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if uuid, ok := p["uuid"].(string); ok {
			parts = append(parts, uuid)
		}
	}
	return parts
}

func TestWorkbenchSourceCRUD(t *testing.T) {
	setupInfoNodeTestDB(t)
	sub := NewSubService("{{EMAIL}}")

	if _, err := sub.SaveWorkbenchSource(WorkbenchSource{Name: "no-url", URL: "ftp://x", Enable: true}); err == nil {
		t.Fatal("non-HTTP URL must be rejected")
	}
	src, err := sub.SaveWorkbenchSource(WorkbenchSource{Name: "机场A", URL: "https://a.example/sub", Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if src.ID == "" {
		t.Fatal("created source needs an id")
	}
	list, err := sub.GetWorkbenchSources()
	if err != nil || len(list) != 1 || list[0].Name != "机场A" {
		t.Fatalf("list mismatch: %+v err=%v", list, err)
	}

	src.Name = "机场A2"
	if _, err := sub.SaveWorkbenchSource(src); err != nil {
		t.Fatal(err)
	}
	list, _ = sub.GetWorkbenchSources()
	if len(list) != 1 || list[0].Name != "机场A2" {
		t.Fatalf("update mismatch: %+v", list)
	}

	if err := sub.DeleteWorkbenchSource(src.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = sub.GetWorkbenchSources()
	if len(list) != 0 {
		t.Fatalf("delete failed: %+v", list)
	}
}

func TestWorkbenchPublicationTokenRotation(t *testing.T) {
	setupInfoNodeTestDB(t)
	sub := NewSubService("{{EMAIL}}")

	if _, err := sub.SaveWorkbenchPublication(WorkbenchPublication{Name: "p"}, false); err == nil {
		t.Fatal("empty selection must be rejected")
	}
	pub, err := sub.SaveWorkbenchPublication(WorkbenchPublication{Name: "主链路", SubIDs: []string{"sub-a"}, Enable: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub.Token) < 12 || pub.CreatedAt == 0 {
		t.Fatalf("publication needs a random token and timestamp: %+v", pub)
	}
	oldToken := pub.Token
	pub.Name = "改名"
	rotated, err := sub.SaveWorkbenchPublication(pub, true)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Token == oldToken || rotated.CreatedAt == 0 {
		t.Fatalf("rotate must mint a new token, keep createdAt: %+v", rotated)
	}
	found, err := sub.FindWorkbenchPublication(rotated.Token)
	if err != nil || found == nil || found.Name != "改名" {
		t.Fatalf("token lookup failed: %+v err=%v", found, err)
	}
	if err := sub.DeleteWorkbenchPublication(pub.ID); err != nil {
		t.Fatal(err)
	}
	if found, err := sub.FindWorkbenchPublication(rotated.Token); err != nil || found != nil {
		t.Fatalf("deleted publication must not resolve: %+v %v", found, err)
	}
}

func TestRenderWorkbenchDedupesPanelAndUpstream(t *testing.T) {
	setupInfoNodeTestDB(t)
	sub := NewSubService("{{EMAIL}}")
	panelProxies := []map[string]any{
		{"name": "panel-a", "type": "vless", "server": "p.example", "port": 443, "uuid": "dup-uuid"},
		{"name": "panel-b", "type": "vless", "server": "q.example", "port": 443, "uuid": "panel-b"},
	}
	upstream := [][]engmodel.NodeIR{{
		{Name: "up-dup", Type: engmodel.ProtocolVLESS, Server: "p.example", Port: 443, Auth: engmodel.Auth{UUID: "dup-uuid"}},
		{Name: "up-x", Type: engmodel.ProtocolTrojan, Server: "t.example", Port: 443, Auth: engmodel.Auth{Password: "pw"}},
	}}

	out, err := sub.RenderWorkbenchDocument(panelProxies, upstream)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "p.example") != 1 {
		t.Fatalf("duplicate p.example node must be deduped:\n%s", out)
	}
	for _, want := range []string{"panel-b", "up-x", "t.example"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in rendered document", want)
		}
	}

	if _, err := sub.RenderWorkbenchDocument(nil, nil); err == nil {
		t.Fatal("empty workbench render must fail")
	}
}
