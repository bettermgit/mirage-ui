package service

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"

	"github.com/google/uuid"
)

// QuickTemplate is a preset turning one click into a fully random inbound
// (port, uuid/keys, paths), mirroring the v2ray-agent one-click install flow.
type QuickTemplate struct {
	Key       string `json:"key" example:"vless_reality_xhttp"`
	Name      string `json:"name" example:"VLESS + REALITY + XHTTP"`
	Protocol  string `json:"protocol" example:"vless"`
	NeedsCert bool   `json:"needsCert" example:"false"`
	Summary   string `json:"summary" example:"No domain or certificate required"`
}

type QuickAddOptions struct {
	Template   string `form:"template" validate:"required"`
	Port       int    `form:"port"`
	Remark     string `form:"remark"`
	CertFile   string `form:"certFile"`
	KeyFile    string `form:"keyFile"`
	ServerName string `form:"serverName"`
}

var quickTemplates = []QuickTemplate{
	{Key: "vless_reality_xhttp", Name: "VLESS + REALITY + XHTTP", Protocol: "vless", Summary: "No domain or certificate required, most censorship-resistant"},
	{Key: "vless_reality_tcp", Name: "VLESS + REALITY + TCP + Vision", Protocol: "vless", Summary: "No domain or certificate required"},
	{Key: "trojan_reality_tcp", Name: "Trojan + REALITY + TCP", Protocol: "trojan", Summary: "No domain or certificate required"},
	{Key: "shadowsocks", Name: "Shadowsocks 2022", Protocol: "shadowsocks", Summary: "Single-user 2022-blake3-aes-256-gcm"},
	{Key: "vless_tcp_tls", Name: "VLESS + TCP + TLS + Vision", Protocol: "vless", NeedsCert: true, Summary: "Requires certificate and key files on the server"},
	{Key: "trojan_ws_tls", Name: "Trojan + WebSocket + TLS", Protocol: "trojan", NeedsCert: true, Summary: "Requires certificate and key files on the server"},
}

// QuickTemplates returns the one-click presets in menu order.
func (s *InboundService) QuickTemplates() []QuickTemplate {
	out := make([]QuickTemplate, len(quickTemplates))
	copy(out, quickTemplates)
	return out
}

// QuickAdd materializes a preset and persists it through the normal AddInbound
// path so validation, runtime dispatch and node sync all still apply.
func (s *InboundService) QuickAdd(userId int, opts QuickAddOptions) (*model.Inbound, bool, error) {
	var tpl *QuickTemplate
	for i := range quickTemplates {
		if quickTemplates[i].Key == strings.TrimSpace(opts.Template) {
			tpl = &quickTemplates[i]
			break
		}
	}
	if tpl == nil {
		return nil, false, fmt.Errorf("unknown quick template %q", opts.Template)
	}
	if tpl.NeedsCert && (strings.TrimSpace(opts.CertFile) == "" || strings.TrimSpace(opts.KeyFile) == "") {
		return nil, false, fmt.Errorf("template %s requires certificate and key file paths", tpl.Key)
	}
	port, err := quickPickPort(opts.Port)
	if err != nil {
		return nil, false, err
	}
	settings, stream, err := quickBuildConfigs(tpl.Key, port, opts)
	if err != nil {
		return nil, false, err
	}
	remark := strings.TrimSpace(opts.Remark)
	if remark == "" {
		remark = fmt.Sprintf("%s-%d", tpl.Protocol, port)
	}
	inbound := &model.Inbound{
		UserId:         userId,
		Enable:         true,
		Port:           port,
		Protocol:       model.Protocol(tpl.Protocol),
		Settings:       settings,
		StreamSettings: stream,
		Tag:            fmt.Sprintf("quick-%d-%s", port, tpl.Key),
		Remark:         remark,
		Sniffing:       quickSniffing,
	}
	return s.AddInbound(inbound)
}

const quickSniffing = `{"enabled":true,"destOverride":["http","tls","quic","fakedns"],"metadataOnly":false,"routeOnly":false,"ipsExcluded":[],"domainsExcluded":[]}`

var realityTargets = []string{"www.microsoft.com", "www.apple.com", "www.cloudflare.com", "www.lovelive-anime.cn"}

func quickBuildConfigs(key string, port int, opts QuickAddOptions) (string, string, error) {
	email := fmt.Sprintf("q%s.%s", random.NumLower(6), strings.ToLower(random.NumLower(3)))
	subID := random.Seq(8)
	var settings, stream map[string]any
	var err error

	switch key {
	case "vless_reality_xhttp", "vless_reality_tcp":
		flow := ""
		if key == "vless_reality_tcp" {
			flow = "xtls-rprx-vision"
		}
		settings = map[string]any{
			"clients":    []any{quickClient(email, subID, map[string]any{"id": uuid.NewString(), "flow": flow})},
			"decryption": "none",
			"encryption": "none",
		}
		if key == "vless_reality_xhttp" {
			stream = map[string]any{
				"network":       "xhttp",
				"xhttpSettings": map[string]any{"path": "/" + random.NumLower(8), "mode": "auto"},
			}
		} else {
			stream = map[string]any{"network": "tcp", "tcpSettings": map[string]any{"header": map[string]any{"type": "none"}}}
		}
		quickAttachReality(stream)
	case "trojan_reality_tcp":
		settings = map[string]any{
			"clients": []any{quickClient(email, subID, map[string]any{"password": random.Seq(16)})},
		}
		stream = map[string]any{"network": "tcp", "tcpSettings": map[string]any{"header": map[string]any{"type": "none"}}}
		quickAttachReality(stream)
	case "shadowsocks":
		settings = map[string]any{
			"method": "2022-blake3-aes-256-gcm", "password": random.Base64Bytes(32),
			"network": "tcp,udp", "ivCheck": false,
		}
		stream = map[string]any{"network": "tcp", "security": "none"}
	case "vless_tcp_tls":
		settings = map[string]any{
			"clients":    []any{quickClient(email, subID, map[string]any{"id": uuid.NewString(), "flow": "xtls-rprx-vision"})},
			"decryption": "none",
			"encryption": "none",
		}
		stream = map[string]any{
			"network": "tcp", "tcpSettings": map[string]any{"header": map[string]any{"type": "none"}},
			"security": "tls", "tlsSettings": quickTLSSettings(opts.ServerName, opts.CertFile, opts.KeyFile),
		}
	case "trojan_ws_tls":
		settings = map[string]any{
			"clients": []any{quickClient(email, subID, map[string]any{"password": random.Seq(16)})},
		}
		stream = map[string]any{
			"network": "ws", "wsSettings": map[string]any{"path": "/" + random.NumLower(8), "host": ""},
			"security": "tls", "tlsSettings": quickTLSSettings(opts.ServerName, opts.CertFile, opts.KeyFile),
		}
	default:
		return "", "", fmt.Errorf("unimplemented quick template %q", key)
	}
	if err != nil {
		return "", "", err
	}
	sj, err := json.Marshal(settings)
	if err != nil {
		return "", "", err
	}
	stj, err := json.Marshal(stream)
	return string(sj), string(stj), err
}

// quickAttachReality adds a fresh REALITY keypair and random target to the stream.
func quickAttachReality(stream map[string]any) {
	priv, pub, err := quickRealityKeys()
	target := realityTargets[random.Num(len(realityTargets))]
	if err != nil {
		target = realityTargets[0]
		priv, pub = "", ""
	}
	shortID := quickShortID()
	stream["security"] = "reality"
	stream["realitySettings"] = map[string]any{
		"show": false, "xver": 0,
		"target": target + ":443", "serverNames": []string{target},
		"privateKey": priv, "minClientVer": "", "maxClientVer": "", "maxTimediff": 0,
		"shortIds": []string{shortID}, "mldsa65Seed": "",
		"settings": map[string]any{"publicKey": pub, "fingerprint": "chrome", "serverName": "", "spiderX": "/", "mldsa65Verify": ""},
	}
}

func quickClient(email, subID string, protoFields map[string]any) map[string]any {
	c := map[string]any{
		"email": email, "limitIp": 0, "totalGB": 0, "expiryTime": 0,
		"enable": true, "tgId": 0, "subId": subID, "comment": "", "reset": 0,
	}
	for k, v := range protoFields {
		c[k] = v
	}
	return c
}

func quickTLSSettings(serverName, certFile, keyFile string) map[string]any {
	return map[string]any{
		"serverName": serverName, "minVersion": "", "maxVersion": "", "cipherSuites": "",
		"rejectUnknownSni": false, "disableSystemRoot": false, "enableSessionResumption": false,
		"certificates":  []any{map[string]any{"certificateFile": filepath.Clean(certFile), "keyFile": filepath.Clean(keyFile), "oneTimeLoading": false, "usage": "encipherment", "buildChain": false}},
		"alpn":          []string{"h2", "http/1.1"},
		"echServerKeys": "",
		"settings":      map[string]any{"fingerprint": "chrome", "echConfigList": ""},
	}
}

func quickRealityKeys() (string, string, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv.Bytes()), enc.EncodeToString(priv.PublicKey().Bytes()), nil
}

func quickShortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// quickPickPort keeps an explicit port, else finds a free one checking both
// the DB (another inbound row) and the live stack (a foreign listener).
func quickPickPort(requested int) (int, error) {
	if requested > 0 {
		return requested, nil
	}
	for range 60 {
		port := random.Num(20000) + 20000
		if quickPortTaken(port) || !quickPortFreeLive(port) {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("could not find a free port, specify one manually")
}

func quickPortTaken(port int) bool {
	var n int64
	db := database.GetDB()
	if db == nil {
		return false
	}
	db.Model(&model.Inbound{}).Where("port = ?", port).Count(&n)
	return n > 0
}

func quickPortFreeLive(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	pc, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = pc.Close()
	return true
}
