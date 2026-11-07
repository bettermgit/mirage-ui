package sub

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	engmodel "github.com/mhsanaei/3x-ui/v3/internal/subconvengine/model"
	"github.com/mhsanaei/3x-ui/v3/internal/subconvengine/parser"
	"github.com/mhsanaei/3x-ui/v3/internal/subconvengine/renderer"
)

const (
	subConvConfKey      = "subConvConf"
	subConvConfCacheTTL = 5 * time.Second
)

var subConvCache struct {
	mu       sync.Mutex
	opts     engmodel.RenderOptions
	loadedAt time.Time
}

// defaultSubConvRenderOptions tunes the vendored engine defaults for the 3x-ui
// context: every node comes from this panel (no multi-source prefixes) and flag
// emoji are enabled out of the box.
func defaultSubConvRenderOptions() engmodel.RenderOptions {
	opts := engmodel.DefaultRenderOptions()
	opts.Emoji = true
	opts.SourcePrefix = false
	return opts
}

// loadSubConvRenderOptions reads the engine config JSON from the settings table
// with a short TTL so edits apply to later fetches without a panel restart.
// Malformed stored JSON falls back to defaults instead of breaking delivery.
func loadSubConvRenderOptions() engmodel.RenderOptions {
	subConvCache.mu.Lock()
	defer subConvCache.mu.Unlock()
	if !subConvCache.loadedAt.IsZero() && time.Since(subConvCache.loadedAt) < subConvConfCacheTTL {
		return subConvCache.opts
	}
	raw := ""
	setting := &model.Setting{}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", subConvConfKey).First(setting).Error; err == nil {
		raw = setting.Value
	}
	opts := defaultSubConvRenderOptions()
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &opts); err != nil {
			logger.Warningf("sub: invalid %s JSON, falling back to engine defaults: %v", subConvConfKey, err)
			opts = defaultSubConvRenderOptions()
		}
	}
	subConvCache.opts = opts
	subConvCache.loadedAt = time.Now()
	return opts
}

// renderMihomoProxies hands the built clash proxy maps to the vendored subconv
// engine: wrap them into a minimal mihomo document, parse to NodeIR, then let
// the engine render the full config. The engine repeats every name across all
// groups, so re-check against the sub budget and halve the node set until it fits.
func renderMihomoProxies(proxies []map[string]any, opts engmodel.RenderOptions) (string, error) {
	limit := 2 * maxRenderedBytes
	for {
		out, err := renderMihomoDocument(proxies, opts)
		if err != nil {
			return "", err
		}
		if len(out) <= limit || len(proxies) <= 1 {
			return out, nil
		}
		logger.Warningf("sub: mihomo render is %d bytes, dropping the tail to fit %d", len(out), limit)
		proxies = proxies[:len(proxies)/2]
	}
}

func renderMihomoDocument(proxies []map[string]any, opts engmodel.RenderOptions) (string, error) {
	doc, err := marshalClashYAML(map[string]any{"proxies": proxies})
	if err != nil {
		return "", fmt.Errorf("subconv: marshal proxies: %w", err)
	}
	parsed := parser.ParseContent(doc, engmodel.SourceInfo{ID: "panel", Name: "panel", Kind: "panel"})
	if len(parsed.Nodes) == 0 {
		if len(parsed.Errors) > 0 {
			return "", fmt.Errorf("subconv: no parsable proxies (%s)", parsed.Errors[0].Message)
		}
		return "", fmt.Errorf("subconv: no parsable proxies")
	}
	out, err := renderer.RenderMihomo(parsed.Nodes, opts)
	if err != nil {
		return "", fmt.Errorf("subconv: render failed: %w", err)
	}
	return string(out), nil
}
