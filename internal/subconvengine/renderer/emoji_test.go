package renderer

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/subconvengine/model"
)

func TestApplyRegionEmojiByTag(t *testing.T) {
	nodes := []model.NodeIR{
		{Name: "Node A", Tags: []string{"HK"}},
		{Name: "Node B", Tags: []string{"JP"}},
		{Name: "🇺🇸 Node C", Tags: []string{"US"}},
		{Name: "Node D", Tags: []string{"OTHER"}},
	}

	got := applyRegionEmoji(nodes)
	want := []string{"🇭🇰 Node A", "🇯🇵 Node B", "🇺🇸 Node C", "Node D"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("applyRegionEmoji()[%d].Name = %q, want %q", i, got[i].Name, want[i])
		}
	}
}

func TestApplyRegionEmojiFallsBackToName(t *testing.T) {
	nodes := []model.NodeIR{{Name: "香港 01"}}
	got := applyRegionEmoji(nodes)
	if got[0].Name != "🇭🇰 香港 01" {
		t.Fatalf("applyRegionEmoji() = %q, want %q", got[0].Name, "🇭🇰 香港 01")
	}
}
