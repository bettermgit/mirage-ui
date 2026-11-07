package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// xray reads a fallback's "name" as an SNI match condition, so the panel's
// internal marker on the camouflage row must never reach the xray JSON or the
// fallback demands SNI=camouflage-site and every real visitor is reset.
func TestBuildFallbacksJSONCamouflageRowStaysUnconditional(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()

	master := mkInbound(t, 30301, model.VLESS, clientsSettings(t, nil))
	master.StreamSettings = `{"network":"tcp","security":"reality","realitySettings":{"serverNames":["s"]}}`
	if err := db.Save(master).Error; err != nil {
		t.Fatalf("save master: %v", err)
	}
	rows := []model.InboundFallback{
		{MasterId: master.Id, ChildId: 0, Name: "camouflage-site", Dest: "127.0.0.1:2082", SortOrder: 0},
		{MasterId: master.Id, ChildId: 0, Name: "s.example", Dest: "127.0.0.1:8443", SortOrder: 1},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed fallback %d: %v", i, err)
		}
	}

	out, err := (&FallbackService{}).BuildFallbacksJSON(nil, master.Id)
	if err != nil {
		t.Fatalf("BuildFallbacksJSON: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 fallbacks, got %d", len(out))
	}
	if _, ok := out[0]["name"]; ok {
		t.Fatalf("camouflage fallback must carry no name (SNI condition), got %v", out[0])
	}
	if out[0]["dest"] != "127.0.0.1:2082" {
		t.Fatalf("camouflage dest: %v", out[0]["dest"])
	}
	if out[1]["name"] != "s.example" {
		t.Fatalf("regular fallback lost its SNI name: %v", out[1])
	}
}
