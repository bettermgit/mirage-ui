[English](/README.md) | [中文](/README.zh_CN.md)

# Mirage-UI

An all-in-one proxy panel combining **REALITY cover-site camouflage** and a
**mihomo subscription workbench**, built by integrating and modifying three
open-source projects.

## Origins & Attribution

This distribution is a modification and integration of:

| Project | License | What we took |
| --- | --- | --- |
| [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) | AGPL-3.0 | The panel foundation: web UI, API, Xray-core management, database schema — this fork's code lives on top of it |
| [Earl9/subconv-next](https://github.com/Earl9/subconv-next) | MIT | The subscription-conversion engine, vendored (at commit `b63bbca`) via its fork [bettermgit/subconv-next-custom](https://github.com/bettermgit/subconv-next-custom) as `internal/subconvengine/` — see [UPSTREAM.md](internal/subconvengine/UPSTREAM.md) |
| [mack-a/v2ray-agent](https://github.com/mack-a/v2ray-agent) | MIT | Design reference: its one-click install flow inspired the quick-inbound generator (random port/keys/paths), and its TLS-fronting cover-site approach inspired the camouflage engine in `internal/camouflage/` |

All original copyright and license texts are retained (`LICENSE`,
`internal/subconvengine/UPSTREAM-LICENSE`).

## AI Authorship

Every code change, bug fix, test and document in this distribution —
including the camouflage engine's hardening, the workbench fixes, this
README and the one-key installer — was written, tested and organized
end-to-end by **Qwen3.8-Flash**, running locally on
[llama.cpp](https://github.com/ggml-org/llama.cpp) on a single AMD Ryzen AI
Max+ 395 workstation (UD-IQ4_XS quantization).

## Features

### 🎭 Camouflage
- One-click cover site per REALITY/TLS inbound: every connection that fails
  the TLS handshake (no SNI, plaintext probes included) is served the static
  cover site instead of an error — the port never looks dead
- Three built-in multi-page templates (blog / company / photo gallery):
  real Chinese content, inline CSS + SVG art, robots.txt, RSS, favicon,
  custom 404
- Scanner-proof: empty probes cannot kill the cover server (peek timeout +
  noise drop), and an accidental serve exit self-heals in 2 seconds
- Cover certificates are generated automatically; real domains keep using
  standard TLS settings

### 🛠 Subscription workbench (mihomo / subconv)
- Built-in rule-set catalog: enable / edit / preview rule sources
- Sources → publication → private subscription link (random token path,
  scan-resistant)
- Publication links are always absolute URLs (reverse-proxy URI first,
  otherwise derived from the panel scheme + subscription port)

### 🩹 Stability fixes
- Workbench writes are sent as JSON end to end (fixes the false
  "publication name is required" caused by form encoding)
- xray `fallbacks` generation no longer leaks panel-internal markers
- Full robustness hardening of the cover server against scanner traffic

## One-key install

```bash
# Prebuilt binaries from your repository's Releases (recommended)
curl -fsSL https://raw.githubusercontent.com/<you>/mirage-ui/main/install-fork.sh | FORK_REPO=<you>/mirage-ui bash

# Or build from source on the server (Go 1.27+ / Node 26+):
git clone https://github.com/<you>/mirage-ui && cd mirage-ui && bash install-fork.sh --from-source
```

The installer prints the panel URL and initial account. Security notes:

1. **Change the password on first login**
2. Open in both the system firewall and the cloud security group: panel port
   (default 5566), subscription port 2096, and every inbound you create

## Docker

```bash
docker build -t mirage-ui .
docker run -d --name mirage-ui --restart unless-stopped --network host \
  -v mirage-xui:/etc/x-ui mirage-ui
```

`--network host` lets proxy inbounds use arbitrary ports without mapping each
one; replace it with `-p 2053:2053 -p 2096:2096 …` if you prefer isolation.
The panel database persists in the `mirage-xui` volume.

## HTTPS

Panel and subscription server accept standard certificate settings
(Settings → SSL). Pair with any ACME tool (acme.sh / certbot) for automated
issuance and renewal; when port 443 is taken by REALITY, validate via
HTTP-01 (port 80) or DNS-01.

## Maintainers: publishing a release

```bash
cd frontend && npm ci && npm run build && cd ..
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o x-ui-linux-amd64 .
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o x-ui-linux-arm64 .
gh release create v1.0.0 x-ui-linux-amd64 x-ui-linux-arm64
```

## Development

The local gate is `make verify`; engineering conventions, the TDD test
philosophy and layering rules are documented in [CLAUDE.md](CLAUDE.md).

## License

AGPL-3.0, inherited from 3x-ui. The vendored subconv engine retains its MIT
license (see `internal/subconvengine/UPSTREAM-LICENSE`).
