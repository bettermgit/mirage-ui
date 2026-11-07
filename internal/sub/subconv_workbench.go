package sub

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	engmodel "github.com/mhsanaei/3x-ui/v3/internal/subconvengine/model"
	"github.com/mhsanaei/3x-ui/v3/internal/subconvengine/parser"
	"github.com/mhsanaei/3x-ui/v3/internal/subconvengine/renderer"
)

const (
	subConvSourcesKey      = "subConvSources"
	subConvPublicationsKey = "subConvPublications"
	// Upstream fetch freshness: publication reads serve the cache; entries
	// older than this trigger a synchronous refresh with stale fallback.
	subConvSourceTTL = 10 * time.Minute
)

// WorkbenchSource is one upstream subscription URL aggregated by the engine
// workbench.
type WorkbenchSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Enable      bool   `json:"enable"`
	LastFetchAt int64  `json:"lastFetchAt"`
	LastCount   int    `json:"lastCount"`
	LastError   string `json:"lastError,omitempty"`
}

// WorkbenchPublication is a generated private subscription link merging the
// selected panel clients and upstream sources.
type WorkbenchPublication struct {
	ID        string   `json:"id"`
	Token     string   `json:"token"`
	Name      string   `json:"name"`
	SourceIDs []string `json:"sourceIds"`
	SubIDs    []string `json:"subIds"`
	Enable    bool     `json:"enable"`
	CreatedAt int64    `json:"createdAt"`
}

func (s *SubService) readWorkbenchList(key string, dst any) error {
	setting := &model.Setting{}
	err := database.GetDB().Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err == gorm.ErrRecordNotFound || (err == nil && strings.TrimSpace(setting.Value) == "") {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(setting.Value), dst)
}

func (s *SubService) writeWorkbenchList(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	setting := &model.Setting{}
	db := database.GetDB()
	err = db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		setting.Key = key
	}
	setting.Value = string(raw)
	return db.Save(setting).Error
}

// GetWorkbenchSources lists the configured upstream subscription sources.
func (s *SubService) GetWorkbenchSources() ([]WorkbenchSource, error) {
	list := []WorkbenchSource{}
	if err := s.readWorkbenchList(subConvSourcesKey, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// SaveWorkbenchSource inserts (empty ID) or updates a source, validating the
// URL shape and name.
func (s *SubService) SaveWorkbenchSource(src WorkbenchSource) (WorkbenchSource, error) {
	src.URL = strings.TrimSpace(src.URL)
	src.Name = strings.TrimSpace(src.Name)
	if !strings.HasPrefix(src.URL, "http://") && !strings.HasPrefix(src.URL, "https://") {
		return WorkbenchSource{}, fmt.Errorf("source URL must start with http:// or https://")
	}
	if src.Name == "" {
		return WorkbenchSource{}, fmt.Errorf("source name is required")
	}
	list, err := s.GetWorkbenchSources()
	if err != nil {
		return WorkbenchSource{}, err
	}
	if src.ID == "" {
		id, err := newWorkbenchID(8)
		if err != nil {
			return WorkbenchSource{}, err
		}
		src.ID = id
		list = append(list, src)
	} else {
		found := false
		for i := range list {
			if list[i].ID == src.ID {
				src.LastFetchAt, src.LastCount, src.LastError = list[i].LastFetchAt, list[i].LastCount, list[i].LastError
				list[i] = src
				found = true
				break
			}
		}
		if !found {
			return WorkbenchSource{}, fmt.Errorf("source %q not found", src.ID)
		}
	}
	if err := s.writeWorkbenchList(subConvSourcesKey, list); err != nil {
		return WorkbenchSource{}, err
	}
	return src, nil
}

// DeleteWorkbenchSource removes a source and drops its node cache.
func (s *SubService) DeleteWorkbenchSource(id string) error {
	list, err := s.GetWorkbenchSources()
	if err != nil {
		return err
	}
	out := list[:0]
	for _, item := range list {
		if item.ID != id {
			out = append(out, item)
		}
	}
	if err := s.writeWorkbenchList(subConvSourcesKey, out); err != nil {
		return err
	}
	subConvSourceCache.remove(id)
	return nil
}

// GetWorkbenchPublications lists the generated private publications.
func (s *SubService) GetWorkbenchPublications() ([]WorkbenchPublication, error) {
	list := []WorkbenchPublication{}
	if err := s.readWorkbenchList(subConvPublicationsKey, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// SaveWorkbenchPublication inserts or updates a publication, generating a
// fresh random token when requested.
func (s *SubService) SaveWorkbenchPublication(pub WorkbenchPublication, rotateToken bool) (WorkbenchPublication, error) {
	pub.Name = strings.TrimSpace(pub.Name)
	if pub.Name == "" {
		return WorkbenchPublication{}, fmt.Errorf("publication name is required")
	}
	if len(pub.SourceIDs) == 0 && len(pub.SubIDs) == 0 {
		return WorkbenchPublication{}, fmt.Errorf("select at least one source or client")
	}
	list, err := s.GetWorkbenchPublications()
	if err != nil {
		return WorkbenchPublication{}, err
	}
	if pub.ID == "" {
		id, err := newWorkbenchID(8)
		if err != nil {
			return WorkbenchPublication{}, err
		}
		pub.ID = id
		token, err := newWorkbenchID(16)
		if err != nil {
			return WorkbenchPublication{}, err
		}
		pub.Token = token
		pub.CreatedAt = time.Now().Unix()
		list = append(list, pub)
	} else {
		found := false
		for i := range list {
			if list[i].ID == pub.ID {
				pub.Token = list[i].Token
				pub.CreatedAt = list[i].CreatedAt
				if rotateToken {
					token, err := newWorkbenchID(16)
					if err != nil {
						return WorkbenchPublication{}, err
					}
					pub.Token = token
				}
				list[i] = pub
				found = true
				break
			}
		}
		if !found {
			return WorkbenchPublication{}, fmt.Errorf("publication %q not found", pub.ID)
		}
	}
	if err := s.writeWorkbenchList(subConvPublicationsKey, list); err != nil {
		return WorkbenchPublication{}, err
	}
	return pub, nil
}

// DeleteWorkbenchPublication revokes a private link.
func (s *SubService) DeleteWorkbenchPublication(id string) error {
	list, err := s.GetWorkbenchPublications()
	if err != nil {
		return err
	}
	out := list[:0]
	for _, item := range list {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return s.writeWorkbenchList(subConvPublicationsKey, out)
}

// FindWorkbenchPublication resolves a publication by its random token.
func (s *SubService) FindWorkbenchPublication(token string) (*WorkbenchPublication, error) {
	list, err := s.GetWorkbenchPublications()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Token == token {
			return &list[i], nil
		}
	}
	return nil, nil
}

type workbenchSourceEntry struct {
	nodes     []engmodel.NodeIR
	fetchedAt time.Time
	err       error
}

type workbenchSourceCache struct {
	mu    sync.Mutex
	items map[string]workbenchSourceEntry
}

var subConvSourceCache = workbenchSourceCache{items: map[string]workbenchSourceEntry{}}

func (c *workbenchSourceCache) remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, id)
}

func (c *workbenchSourceCache) get(id string) (workbenchSourceEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[id]
	return entry, ok
}

// FetchWorkbenchSource downloads and parses one upstream subscription,
// updating its status fields in the settings list.
func (s *SubService) FetchWorkbenchSource(id string) ([]engmodel.NodeIR, error) {
	list, err := s.GetWorkbenchSources()
	if err != nil {
		return nil, err
	}
	var src *WorkbenchSource
	for i := range list {
		if list[i].ID == id {
			src = &list[i]
			break
		}
	}
	if src == nil {
		return nil, fmt.Errorf("source %q not found", id)
	}
	nodes, fetchErr := fetchUpstreamNodes(src.URL, src.Name)
	src.LastFetchAt = time.Now().Unix()
	src.LastCount = len(nodes)
	src.LastError = ""
	if fetchErr != nil {
		src.LastError = fetchErr.Error()
	}
	if writeErr := s.writeWorkbenchList(subConvSourcesKey, list); writeErr != nil {
		logger.Warningf("sub: workbench source status write failed: %v", writeErr)
	}
	subConvSourceCache.mu.Lock()
	subConvSourceCache.items[id] = workbenchSourceEntry{nodes: nodes, fetchedAt: time.Now(), err: fetchErr}
	subConvSourceCache.mu.Unlock()
	return nodes, fetchErr
}

// sourceNodes returns cached nodes for a source, refreshing when stale;
// stale nodes are served when a refresh fails.
func (s *SubService) sourceNodes(src WorkbenchSource, maxStale time.Duration) ([]engmodel.NodeIR, error) {
	if entry, ok := subConvSourceCache.get(src.ID); ok {
		fresh := time.Since(entry.fetchedAt) < subConvSourceTTL
		if fresh {
			return entry.nodes, entry.err
		}
		nodes, err := s.FetchWorkbenchSource(src.ID)
		if err != nil && time.Since(entry.fetchedAt) <= maxStale {
			return entry.nodes, nil
		}
		return nodes, err
	}
	nodes, err := s.FetchWorkbenchSource(src.ID)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// RefreshWorkbenchSources fetches every enabled source (cron entry point).
func (s *SubService) RefreshWorkbenchSources() {
	list, err := s.GetWorkbenchSources()
	if err != nil {
		logger.Warningf("sub: workbench refresh listing failed: %v", err)
		return
	}
	for _, src := range list {
		if !src.Enable {
			continue
		}
		if _, err := s.FetchWorkbenchSource(src.ID); err != nil {
			logger.Warningf("sub: workbench source %q refresh failed: %v", src.Name, err)
		}
	}
}

var workbenchHTTPClient = &http.Client{
	Timeout:   8 * time.Second,
	Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, IdleConnTimeout: 60 * time.Second},
}

func fetchUpstreamNodes(rawURL, sourceName string) ([]engmodel.NodeIR, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// Providers commonly gate subscription bodies on a client User-Agent;
	// clash.meta makes them return the mihomo profile variant.
	req.Header.Set("User-Agent", "clash.meta/v1")
	resp, err := workbenchHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, subscriptionMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > subscriptionMaxBytes {
		return nil, fmt.Errorf("upstream response exceeds %d bytes", subscriptionMaxBytes)
	}
	parsed := parser.ParseContent(body, engmodel.SourceInfo{ID: sourceName, Name: sourceName, Kind: "subscription"})
	if len(parsed.Nodes) == 0 {
		if len(parsed.Errors) > 0 {
			return nil, fmt.Errorf("no parsable nodes (%s)", parsed.Errors[0].Message)
		}
		return nil, fmt.Errorf("no parsable nodes in upstream content")
	}
	return parsed.Nodes, nil
}

// RenderWorkbenchDocument merges panel-client proxies with upstream source
// nodes and renders one mihomo document.
func (s *SubService) RenderWorkbenchDocument(panelProxies []map[string]any, sourceNodes [][]engmodel.NodeIR) (string, error) {
	nodes := make([]engmodel.NodeIR, 0, 64)
	for _, batch := range sourceNodes {
		nodes = append(nodes, batch...)
	}
	if len(panelProxies) > 0 {
		doc, err := marshalClashYAML(map[string]any{"proxies": panelProxies})
		if err != nil {
			return "", fmt.Errorf("workbench: marshal panel proxies: %w", err)
		}
		parsed := parser.ParseContent(doc, engmodel.SourceInfo{ID: "panel", Name: "panel", Kind: "panel"})
		if len(parsed.Nodes) == 0 && len(parsed.Errors) > 0 {
			return "", fmt.Errorf("workbench: panel proxies unparsable (%s)", parsed.Errors[0].Message)
		}
		nodes = append(nodes, parsed.Nodes...)
	}
	nodes = dedupeWorkbenchNodes(nodes)
	if len(nodes) == 0 {
		return "", fmt.Errorf("workbench: nothing to render (no clients or upstream nodes selected)")
	}
	opts := loadSubConvRenderOptions()
	if len(sourceNodes) > 0 {
		opts.SourcePrefix = true
	}
	return renderMihomoNodes(nodes, opts)
}

func dedupeWorkbenchNodes(nodes []engmodel.NodeIR) []engmodel.NodeIR {
	seen := make(map[string]struct{}, len(nodes))
	out := make([]engmodel.NodeIR, 0, len(nodes))
	for _, node := range nodes {
		key := strings.ToLower(strings.Join([]string{
			string(node.Type), node.Server, fmt.Sprint(node.Port), node.Auth.UUID, node.Auth.Password, node.Auth.Username, node.Auth.Token,
		}, "|"))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, node)
	}
	return out
}

func renderMihomoNodes(nodes []engmodel.NodeIR, opts engmodel.RenderOptions) (string, error) {
	limit := 2 * maxRenderedBytes
	for {
		out, err := renderer.RenderMihomo(nodes, opts)
		if err != nil {
			return "", fmt.Errorf("workbench: render failed: %w", err)
		}
		if len(out) <= limit || len(nodes) <= 1 {
			return string(out), nil
		}
		logger.Warningf("sub: workbench render is %d bytes, dropping the tail to fit %d", len(out), limit)
		nodes = nodes[:len(nodes)/2]
	}
}

func newWorkbenchID(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(buf), "="), nil
}

// WorkbenchSourceOptions lists source id/name pairs for the frontend.
func (s *SubService) WorkbenchSourceOptions() ([]map[string]any, error) {
	list, err := s.GetWorkbenchSources()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for _, src := range list {
		out = append(out, map[string]any{"id": src.ID, "name": src.Name})
	}
	sort.Slice(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["name"]) < fmt.Sprint(out[j]["name"])
	})
	return out, nil
}

func (s *SubService) findWorkbenchSource(id string) *WorkbenchSource {
	list, err := s.GetWorkbenchSources()
	if err != nil {
		return nil
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// RenderPublication renders one private link: selected panel clients plus
// upstream sources merged through the mihomo engine. Unreachable sources
// degrade to their cached nodes within 24h, otherwise they are skipped.
func (s *SubClashService) RenderPublication(pub *WorkbenchPublication, host string) (string, error) {
	var panelProxies []map[string]any
	if len(pub.SubIDs) > 0 {
		proxies, err := s.CollectPanelProxies(pub.SubIDs, host)
		if err != nil {
			return "", err
		}
		panelProxies = proxies
	}
	var batches [][]engmodel.NodeIR
	var failures []string
	for _, sourceID := range pub.SourceIDs {
		src := s.SubService.findWorkbenchSource(sourceID)
		if src == nil {
			failures = append(failures, "unknown source "+sourceID)
			continue
		}
		nodes, err := s.SubService.sourceNodes(*src, 24*time.Hour)
		if err != nil {
			failures = append(failures, src.Name+": "+err.Error())
			continue
		}
		if len(nodes) > 0 {
			batches = append(batches, nodes)
		}
	}
	out, err := s.SubService.RenderWorkbenchDocument(panelProxies, batches)
	if err != nil {
		if len(failures) > 0 {
			return "", fmt.Errorf("%s: %w", strings.Join(failures, "; "), err)
		}
		return "", err
	}
	return out, nil
}
