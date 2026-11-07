#!/usr/bin/env bash
#
# Mirage-UI one-key installer
#
# Downloads the prebuilt binary from this repository's GitHub Releases,
# installs a systemd service and prints the initial panel credentials.
#
#   curl -fsSL https://raw.githubusercontent.com/bettermgit/mirage-ui/main/install-fork.sh | bash
#
# Environment overrides:
#   FORK_REPO=you/mirage-ui   repository whose Releases hold the binaries
#   FORK_VERSION=v1.0.0       release tag to install (default: latest)
#   XUI_PANEL_PORT=5566       panel port
#   XUI_USERNAME/XUI_PASSWORD first-login account (default: generated)
#   XUI_MAIN_FOLDER=/usr/local/x-ui
#
# The account/password are consumed only on the very first start (empty
# database); the temp env file is removed right after.
set -euo pipefail

FORK_REPO="${FORK_REPO:-bettermgit/mirage-ui}"
FORK_VERSION="${FORK_VERSION:-latest}"
PANEL_PORT="${XUI_PANEL_PORT:-5566}"
SEEDED_USER="${XUI_USERNAME:-}"
SEEDED_PASS="${XUI_PASSWORD:-}"
XUI_FOLDER="${XUI_MAIN_FOLDER:-/usr/local/x-ui}"
SERVICE=x-ui
INIT_ENV="${XUI_FOLDER}/acc/initial.env"

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; plain='\033[0m'
say()  { echo -e "${green}==> $*${plain}"; }
warn() { echo -e "${yellow}==> $*${plain}"; }
die()  { echo -e "${red}==> $*${plain}" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Run as root (or via sudo)."
command -v systemctl >/dev/null || die "systemd is required."
command -v curl >/dev/null || die "curl is required (apt install curl)."
[[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) \
  || die "XUI_PANEL_PORT must be a number between 1 and 65535."
if [[ -n "$SEEDED_PASS" && ${#SEEDED_PASS} -lt 8 ]]; then
  die "XUI_PASSWORD must be at least 8 characters."
fi
if [[ ( -n "$SEEDED_USER" && -z "$SEEDED_PASS" ) || ( -z "$SEEDED_USER" && -n "$SEEDED_PASS" ) ]]; then
  die "Set both XUI_USERNAME and XUI_PASSWORD, or neither."
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ASSET=x-ui-linux-amd64  ;;
  aarch64) ASSET=x-ui-linux-arm64  ;;
  *) die "Unsupported architecture: $ARCH (amd64/arm64 only; use --from-source)" ;;
esac

if [[ "${1:-}" == "--from-source" ]]; then
  command -v go >/dev/null || die "Go 1.27+ required for --from-source (https://go.dev/dl)."
  command -v npm >/dev/null || die "Node 26+ / npm required for --from-source."
  TMP_SRC="$(mktemp -d)"; trap 'rm -rf "$TMP_SRC"' EXIT
  say "Cloning ${FORK_REPO} ..."
  git clone --depth 1 "https://github.com/${FORK_REPO}.git" "$TMP_SRC"
  ( cd "$TMP_SRC/frontend" && npm ci && npm run build )
  ( cd "$TMP_SRC" && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o x-ui . )
  BINARY="$TMP_SRC/x-ui"
else
  if [[ "$FORK_VERSION" == "latest" ]]; then
    TAG_JSON="$(curl -fsSL "https://api.github.com/repos/${FORK_REPO}/releases/latest")" \
      || die "Cannot read Releases of ${FORK_REPO}. Set FORK_REPO=owner/repo, or pass --from-source."
    FORK_VERSION="$(printf '%s' "$TAG_JSON" | grep -m1 '"tag_name"' | cut -d'"' -f4)"
  fi
  URL="https://github.com/${FORK_REPO}/releases/download/${FORK_VERSION}/${ASSET}"
  say "Downloading ${ASSET} (${FORK_VERSION}) ..."
  TMP_BIN="$(mktemp)"; trap 'rm -f "$TMP_BIN"' EXIT
  curl -fL "$URL" -o "$TMP_BIN" \
    || die "Download failed: $URL
Release the prebuilt binary once (see README 'Building a release'), or run: $0 --from-source"
  BINARY="$TMP_BIN"
fi

say "Installing into ${XUI_FOLDER} ..."
mkdir -p "$XUI_FOLDER"/{bin,logs,acc}
install -m 0755 "$BINARY" "$XUI_FOLDER/x-ui"
touch "$XUI_FOLDER/x-ui-access.log"

cat > /etc/systemd/system/${SERVICE}.service <<UNIT
[Unit]
Description=Mirage-UI Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=${XUI_FOLDER}
Environment=XUI_DB_FOLDER=${XUI_FOLDER}
Environment=XUI_BIN_FOLDER=${XUI_FOLDER}/bin
Environment=XUI_LOG_FOLDER=${XUI_FOLDER}/logs
Environment=XUI_PORT=${PANEL_PORT}
EnvironmentFile=-${INIT_ENV}
ExecStart=${XUI_FOLDER}/x-ui run
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
UNIT

say "Initialising settings (port ${PANEL_PORT}) ..."
systemctl daemon-reload
if [[ -n "$SEEDED_USER" ]]; then
  umask 077
  printf 'XUI_USERNAME=%s\nXUI_PASSWORD=%s\n' "$SEEDED_USER" "$SEEDED_PASS" > "$INIT_ENV"
fi
systemctl enable --now "$SERVICE"

sleep 5
rm -f "$INIT_ENV"
IP="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"
CREDS="$(journalctl -u "$SERVICE" --no-pager 2>/dev/null | grep -m1 'Username:' || true)"

say "Done."
echo
echo -e "  Panel URL : http://${IP}:${PANEL_PORT}/"
if [[ -n "$SEEDED_USER" ]]; then
  echo -e "  Login     : ${SEEDED_USER} / (the XUI_PASSWORD you provided)"
elif [[ -n "$CREDS" ]]; then
  echo -e "  ${CREDS#*generated. }"
else
  warn "No first-run credentials in the log (service was running before)."
  warn "Reset with: ${XUI_FOLDER}/x-ui setting -username admin -password '<new password>' && systemctl restart ${SERVICE}"
fi
echo
warn "Open these ports in the server firewall AND cloud security group:"
warn "  ${PANEL_PORT} (panel), 2096 (subscriptions), plus every inbound port you create"
warn "Then change the password on first login, and set up TLS (panel Settings -> SSL)"
