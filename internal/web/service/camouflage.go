package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/camouflage"
	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

const (
	camouflageSettingKey   = "camouflageSite"
	camouflageFallbackName = "camouflage-site"
	camouflageDefaultPort  = 2082
)

// CamouflageConfig is persisted as one JSON value in the settings table.
type CamouflageConfig struct {
	Enabled     bool   `json:"enabled" example:"true"`
	Template    string `json:"template" example:"blog"`
	Port        int    `json:"port" example:"2082"`
	RedirectURL string `json:"redirectUrl,omitempty" example:"https://example.com"`
	MasterID    int    `json:"masterInboundId,omitempty" example:"3"`
	// REALITY forwards failed handshakes to target, so the cover replaces it
	// while enabled; this holds the displaced original to restore on disable.
	RealityTarget string `json:"realityTarget,omitempty"`
	UpdatedAt     int64  `json:"updatedAt" example:"1730000000"`
}

type CamouflageApplyRequest struct {
	Template        string `form:"template"`
	Port            int    `form:"port"`
	RedirectURL     string `form:"redirectUrl"`
	MasterInboundId int    `form:"masterInboundId"`
}

type CamouflageStatus struct {
	Enabled   bool                 `json:"enabled" example:"true"`
	Running   bool                 `json:"running" example:"true"`
	Port      int                  `json:"port" example:"2082"`
	Template  string               `json:"template" example:"blog"`
	Redirect  string               `json:"redirectUrl,omitempty"`
	MasterID  int                  `json:"masterInboundId,omitempty" example:"3"`
	Server    string               `json:"server" example:"127.0.0.1:2082"`
	Templates []CamouflageTemplate `json:"templates"`
}

// CamouflageTemplate mirrors camouflage.Template so the type generator stays package-local.
type CamouflageTemplate struct {
	Key  string `json:"key" example:"blog"`
	Name string `json:"name" example:"Personal Blog"`
}

type CamouflageService struct{}

func camouflageDataDir() string {
	return filepath.Join(config.GetDBFolderPath(), "camouflage")
}

func (s *CamouflageService) GetConfig() (CamouflageConfig, error) {
	var cfg CamouflageConfig
	var row model.Setting
	err := database.GetDB().Where("key = ?", camouflageSettingKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if row.Value != "" {
		if err := json.Unmarshal([]byte(row.Value), &cfg); err != nil {
			return CamouflageConfig{}, fmt.Errorf("stored camouflage config is corrupt: %w", err)
		}
	}
	return cfg, nil
}

func (s *CamouflageService) saveConfig(cfg CamouflageConfig) error {
	cfg.UpdatedAt = time.Now().Unix()
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	row := model.Setting{Key: camouflageSettingKey, Value: string(b)}
	return database.GetDB().Where(model.Setting{Key: camouflageSettingKey}).
		Assign(model.Setting{Value: string(b)}).FirstOrCreate(&row).Error
}

// Status reports config plus live listener state and the template catalog.
func (s *CamouflageService) GetStatus() (CamouflageStatus, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return CamouflageStatus{}, err
	}
	st := camouflage.Default.Status()
	tpls := make([]CamouflageTemplate, 0, 4)
	for _, t := range camouflage.List() {
		tpls = append(tpls, CamouflageTemplate{Key: t.Key, Name: t.Name})
	}
	return CamouflageStatus{
		Enabled: cfg.Enabled, Running: st.Running, Port: st.Port,
		Template: cfg.Template, Redirect: cfg.RedirectURL, MasterID: cfg.MasterID,
		Server:    fmt.Sprintf("127.0.0.1:%d", st.Port),
		Templates: tpls,
	}, nil
}

// Apply deploys the template, (re)starts the static server and attaches an
// unconditional fallback to the master inbound so unmatched TLS goes to the site.
func (s *CamouflageService) Apply(req CamouflageApplyRequest) (CamouflageStatus, error) {
	tpl := strings.TrimSpace(req.Template)
	redirect := strings.TrimSpace(req.RedirectURL)
	if tpl == "" && redirect == "" {
		return CamouflageStatus{}, fmt.Errorf("provide a template or a redirect URL")
	}
	if tpl != "" && !camouflage.Valid(tpl) {
		return CamouflageStatus{}, fmt.Errorf("unknown camouflage template %q", tpl)
	}
	port := req.Port
	if port <= 0 {
		port = camouflageDefaultPort
	}

	dir := camouflageDataDir()
	if tpl != "" {
		if err := camouflage.Deploy(tpl, dir); err != nil {
			return CamouflageStatus{}, err
		}
	}
	if err := camouflage.Default.Start(dir, port, redirect); err != nil {
		return CamouflageStatus{}, err
	}

	old, _ := s.GetConfig()
	if err := s.removeFallbacks(old.MasterID, req.MasterInboundId); err != nil {
		_ = camouflage.Default.Stop()
		return CamouflageStatus{}, err
	}
	if old.MasterID > 0 && old.MasterID != req.MasterInboundId {
		if err := restoreRealityTarget(old.MasterID, old.RealityTarget); err != nil {
			_ = camouflage.Default.Stop()
			return CamouflageStatus{}, err
		}
	}
	if req.MasterInboundId > 0 {
		if err := s.attachFallback(req.MasterInboundId, port); err != nil {
			_ = camouflage.Default.Stop()
			return CamouflageStatus{}, s.clearMaster(req.MasterInboundId, err)
		}
	}
	storedTarget := old.RealityTarget
	if req.MasterInboundId > 0 {
		saved, err := redirectRealityTarget(req.MasterInboundId, fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			_ = camouflage.Default.Stop()
			return CamouflageStatus{}, s.clearMaster(req.MasterInboundId, err)
		}
		if storedTarget == "" {
			storedTarget = saved
		}
	}
	if err := s.saveConfig(CamouflageConfig{
		Enabled: true, Template: tpl, Port: port,
		RedirectURL: redirect, MasterID: req.MasterInboundId, RealityTarget: storedTarget,
	}); err != nil {
		return CamouflageStatus{}, err
	}
	(&XrayService{}).SetToNeedRestart()
	return s.GetStatus()
}

// Disable stops the server and removes only the panel-created camouflage fallbacks.
func (s *CamouflageService) Disable() error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}
	if err := s.removeFallbacks(cfg.MasterID, 0); err != nil {
		return err
	}
	if err := restoreRealityTarget(cfg.MasterID, cfg.RealityTarget); err != nil {
		return err
	}
	if err := camouflage.Default.Stop(); err != nil {
		return err
	}
	cfg.Enabled = false
	cfg.MasterID = 0
	cfg.RealityTarget = ""
	return s.saveConfig(cfg)
}

// EnsureRunning is the boot hook: when the site was enabled before a restart,
// redeploy (dir may be gone) and rebind, tolerating a busy port.
func (s *CamouflageService) EnsureRunning() {
	cfg, err := s.GetConfig()
	if err != nil || !cfg.Enabled {
		return
	}
	dir := camouflageDataDir()
	if cfg.Template != "" {
		if err := camouflage.Deploy(cfg.Template, dir); err != nil {
			return
		}
	}
	_ = camouflage.Default.Start(dir, cfg.Port, cfg.RedirectURL)
}

func (s *CamouflageService) attachFallback(masterId, port int) error {
	var fb FallbackService
	existing, err := fb.GetByMaster(masterId)
	if err != nil {
		return err
	}
	items := make([]FallbackInput, 0, len(existing)+1)
	for _, r := range existing {
		if r.Name == camouflageFallbackName {
			continue
		}
		items = append(items, FallbackInput{ChildId: r.ChildId, Name: r.Name, Alpn: r.Alpn, Path: r.Path, Dest: r.Dest, Xver: r.Xver, SortOrder: r.SortOrder})
	}
	items = append(items, FallbackInput{
		Name: camouflageFallbackName, Dest: fmt.Sprintf("127.0.0.1:%d", port), SortOrder: len(items),
	})
	if err := fb.SetByMaster(masterId, items); err != nil {
		return err
	}
	return nil
}

// redirectRealityTarget points a REALITY master's target at the cover site and
// returns the displaced original; non-REALITY streams are left untouched.
func redirectRealityTarget(masterId int, addr string) (string, error) {
	stream, ib, err := loadInboundStream(masterId)
	if err != nil || stream == nil {
		return "", err
	}
	rs, _ := stream["realitySettings"].(map[string]any)
	if rs == nil {
		return "", nil
	}
	cur, _ := rs["target"].(string)
	if cur == "" || cur == addr {
		return "", nil
	}
	rs["target"] = addr
	return cur, storeInboundStream(ib, stream)
}

func restoreRealityTarget(masterId int, saved string) error {
	if masterId <= 0 || saved == "" {
		return nil
	}
	stream, ib, err := loadInboundStream(masterId)
	if err != nil || stream == nil {
		return err
	}
	rs, _ := stream["realitySettings"].(map[string]any)
	if rs == nil {
		return nil
	}
	rs["target"] = saved
	return storeInboundStream(ib, stream)
}

func loadInboundStream(masterId int) (map[string]any, *model.Inbound, error) {
	var ib model.Inbound
	if err := database.GetDB().First(&ib, masterId).Error; err != nil {
		return nil, nil, err
	}
	var stream map[string]any
	if json.Unmarshal([]byte(ib.StreamSettings), &stream) != nil || stream == nil {
		return nil, nil, nil
	}
	return stream, &ib, nil
}

func storeInboundStream(ib *model.Inbound, stream map[string]any) error {
	b, err := json.Marshal(stream)
	if err != nil {
		return err
	}
	return database.GetDB().Model(&model.Inbound{}).
		Where("id = ?", ib.Id).Update("stream_settings", string(b)).Error
}

func (s *CamouflageService) removeFallbacks(masterIds ...int) error {
	seen := map[int]bool{}
	for _, id := range masterIds {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		err := database.GetDB().
			Where("master_id = ? AND name = ?", id, camouflageFallbackName).
			Delete(&model.InboundFallback{}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// clearMaster records that the master kept its list after a failed attach.
func (s *CamouflageService) clearMaster(masterId int, cause error) error {
	cfg, _ := s.GetConfig()
	cfg.Enabled = false
	cfg.MasterID = 0
	if err := s.saveConfig(cfg); err != nil {
		return fmt.Errorf("%w (and persisting disabled state failed: %v)", cause, err)
	}
	return cause
}
