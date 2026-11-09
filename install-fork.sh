#!/usr/bin/env bash
#
# Mirage-UI installer & management script
#
#   curl -fsSL https://raw.githubusercontent.com/bettermgit/mirage-ui/main/install-fork.sh | bash
#
# Like the 3x-ui original: after installing, the script is placed at
# /usr/bin/x-ui, so a bare  x-ui  opens the management menu, and
#   x-ui start|stop|restart|status|settings|enable|disable|log|update|uninstall
# work directly. Non-interactive when stdin is not a terminal or when all
# answers are provided up front:
#
#   FORK_REPO=bettermgit/mirage-ui  Releases to download from
#   FORK_VERSION=v1.0.1             release tag (default: latest)
#   XUI_PANEL_PORT=5566             panel port (implies unattended install)
#   XUI_USERNAME + XUI_PASSWORD     fixed first login (else: generated)
#   XUI_MAIN_FOLDER=/usr/local/x-ui install folder
set -euo pipefail

# install_manager rewrites these two lines in the installed copy.
XUI_FOLDER_DEFAULT=/usr/local/x-ui
SERVICE_DEFAULT=x-ui

FORK_REPO="${FORK_REPO:-bettermgit/mirage-ui}"
FORK_VERSION="${FORK_VERSION:-latest}"
PANEL_PORT="${XUI_PANEL_PORT:-}"
SEEDED_USER="${XUI_USERNAME:-}"
SEEDED_PASS="${XUI_PASSWORD:-}"
XUI_FOLDER="${XUI_MAIN_FOLDER:-$XUI_FOLDER_DEFAULT}"
SERVICE="${XUI_SERVICE:-$SERVICE_DEFAULT}"
INIT_ENV="${XUI_FOLDER}/acc/initial.env"
UNIT_FILE="/etc/systemd/system/${SERVICE}.service"
BINARY="${XUI_FOLDER}/x-ui"
VERSION_FILE="${XUI_FOLDER}/version"

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;34m'; cyan='\033[0;36m'; plain='\033[0m'
say()  { echo -e "${green}==> $*${plain}"; }
warn() { echo -e "${yellow}==> $*${plain}"; }
die()  { echo -e "${red}==> $*${plain}" >&2; exit 1; }

is_tty() { [[ -t 0 && -t 1 ]]; }

installed_version() {
  [[ -f "$VERSION_FILE" ]] && cat "$VERSION_FILE" || echo "unknown"
}

ask() {
  local __var="$1" __prompt="$2" __default="${3:-}" __val
  while true; do
    if ! IFS= read -rp "${__prompt}${__default:+ [${__default}]}: " __val; then
      echo; __val=""
    fi
    __val="${__val:-$__default}"
    [[ -n "$__val" || -n "$__default" ]] || { warn "A value is required."; continue; }
    printf -v "$__var" '%s' "$__val"
    return 0
  done
}

port_in_use() { ss -Hltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${1}\$"; }

# Official flow: y/N customize, N picks a random free port.
select_panel_port() {
  local confirm=""
  ask confirm "Would you like to customize the Panel Port settings? (If not, a random port will be applied)" "n"
  if [[ "${confirm,,}" =~ ^(y|yes)$ ]]; then
    while true; do
      ask PANEL_PORT "Please set up the panel port" "5566"
      [[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) || \
        { warn "Invalid port. Enter a number between 1 and 65535."; continue; }
      if port_in_use "$PANEL_PORT"; then
        warn "Port ${PANEL_PORT} is already in use. Choose another panel port."
        continue
      fi
      echo -e "Your Panel Port is: ${PANEL_PORT}"
      return 0
    done
  fi
  local attempt
  for attempt in {1..100}; do
    PANEL_PORT=$(( RANDOM % (62000 - 1024) + 1024 ))
    if ! port_in_use "$PANEL_PORT"; then
      echo -e "Generated random port: ${PANEL_PORT}"
      return 0
    fi
  done
  die "Could not find an available random port."
}

ask_creds() {
  [[ -n "$SEEDED_USER" ]] && return 0
  local fix=""
  ask fix "Set your own username and password? (N = random, shown at the end)" "n"
  [[ "${fix,,}" =~ ^(y|yes)$ ]] || return 0
  ask SEEDED_USER "Username" "admin"
  local p1="" p2=""
  while true; do
    if ! IFS= read -rsp "Password (min 8 chars, hidden): " p1; then echo; exit 1; fi
    echo
    if ! IFS= read -rsp "Repeat password: " p2; then echo; exit 1; fi
    echo
    [[ ${#p1} -ge 8 ]] || { warn "Too short (min 8 characters)."; continue; }
    [[ "$p1" == "$p2" ]] || { warn "Passwords do not match."; continue; }
    SEEDED_PASS="$p1"
    return 0
  done
}

server_ip() {
  curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}'
}

current_port() {
  [[ -f "$UNIT_FILE" ]] || { echo "2053"; return; }
  grep -m1 -oE 'XUI_PORT=[0-9]+' "$UNIT_FILE" | cut -d= -f2 || echo 2053
}

panel_login() {
  local base="http://127.0.0.1:$1" jar tok out
  jar="$(mktemp)"
  tok="$(curl -s -m 10 -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  out="$(curl -s -m 10 -b "$jar" -X POST "$base/login" -H "X-CSRF-Token: $tok" \
    --data-urlencode "username=$2" --data-urlencode "password=$3" || true)"
  rm -f "$jar"
  grep -q '"success":true' <<<"$out"
}

panel_post() {
  local port="$1" user="$2" pass="$3" path="$4" body="$5"
  local base="http://127.0.0.1:$port" jar tok
  jar="$(mktemp)"
  tok="$(curl -s -m 10 -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  curl -s -m 10 -b "$jar" -c "$jar" -X POST "$base/login" -H "X-CSRF-Token: $tok" \
    --data-urlencode "username=$user" --data-urlencode "password=$pass" >/dev/null
  tok="$(curl -s -m 10 -b "$jar" -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  curl -s -m 15 -b "$jar" -X POST "$base$path" -H "X-CSRF-Token: $tok" \
    -H "Content-Type: application/json" -d "$body"
  rm -f "$jar"
}

json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  printf '%s' "$s"
}

# configure_panel_https <port> <user> <pass> <crt> <key>
configure_panel_https() {
  local port="$1" user="$2" pass="$3" crt="$4" key="$5"
  local base="http://127.0.0.1:$port" jar tok all body
  jar="$(mktemp)"
  tok="$(curl -s -m 10 -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  curl -s -m 10 -b "$jar" -c "$jar" -X POST "$base/login" -H "X-CSRF-Token: $tok" \
    --data-urlencode "username=$user" --data-urlencode "password=$pass" >/dev/null
  tok="$(curl -s -m 10 -b "$jar" -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  all="$(curl -s -m 10 -b "$jar" -X POST "$base/panel/api/setting/all" -H "X-CSRF-Token: $tok")"
  if ! body="$(ALL="$all" CRT="$crt" KEY="$key" python3 - <<'PY'
import json, os, urllib.parse
d = json.loads(os.environ["ALL"])["obj"]
d["webCertFile"] = os.environ["CRT"]
d["webKeyFile"] = os.environ["KEY"]
print(urllib.parse.urlencode(d))
PY
)"; then
    rm -f "$jar"
    return 1
  fi
  curl -s -m 15 -b "$jar" -X POST "$base/panel/api/setting/update" -H "X-CSRF-Token: $tok" \
    -H "Content-Type: application/x-www-form-urlencoded" --data "$body"
  rm -f "$jar"
}

need_root_checks() {
  [[ $EUID -eq 0 ]] || die "Run as root (or via sudo)."
  command -v systemctl >/dev/null || die "systemd is required."
  command -v curl >/dev/null || die "curl is required (apt install curl)."
}

detect_asset() {
  case "$(uname -m)" in
    x86_64)  ASSET=x-ui-linux-amd64;  XRAY_ASSET=Xray-linux-64.zip       ;;
    aarch64) ASSET=x-ui-linux-arm64;  XRAY_ASSET=Xray-linux-arm64-v8a.zip ;;
    *) die "Unsupported architecture: $(uname -m) (amd64/arm64 only)" ;;
  esac
}

# install_xray_core: best effort; the panel also offers it later in Settings.
install_xray_core() {
  local dest="${XUI_FOLDER}/bin/xray-linux-amd64"
  [[ "$(uname -m)" == aarch64 ]] && dest="${XUI_FOLDER}/bin/xray-linux-arm64"
  local tmp
  tmp="$(mktemp -d)"
  say "Downloading xray core (latest release) ..."
  if ! curl -fsSL --max-time 120 \
      "https://github.com/XTLS/Xray-core/releases/latest/download/${XRAY_ASSET}" -o "$tmp/xray.zip"; then
    rm -rf "$tmp"
    warn "xray download failed; skipped (panel Settings can install it later)."
    return 0
  fi
  if command -v unzip >/dev/null; then
    unzip -p "$tmp/xray.zip" xray > "$dest" || { rm -rf "$tmp"; warn "xray extract failed; skipped."; return 0; }
  elif command -v python3 >/dev/null; then
    python3 - "$tmp/xray.zip" > "$dest" <<'PY' || { rm -rf "$tmp"; warn "xray extract failed; skipped."; return 0; }
import sys, zipfile
sys.stdout.buffer.write(zipfile.ZipFile(sys.argv[1]).read("xray"))
PY
  else
    rm -rf "$tmp"
    warn "Neither unzip nor python3 available; xray skipped (panel Settings can install it later)."
    return 0
  fi
  chmod 0755 "$dest"
  rm -rf "$tmp"
  say "xray core installed."
}

download_binary() {
  if [[ "${1:-}" == "--from-source" ]]; then
    command -v go  >/dev/null || die "Go 1.27+ required for --from-source (https://go.dev/dl)."
    command -v npm >/dev/null || die "Node 26+ / npm required for --from-source."
    TMP_SRC="$(mktemp -d)"; trap 'rm -rf "$TMP_SRC"' EXIT
    say "Cloning ${FORK_REPO} ..."
    git clone --depth 1 "https://github.com/${FORK_REPO}.git" "$TMP_SRC"
    ( cd "$TMP_SRC/frontend" && npm ci && npm run build )
    ( cd "$TMP_SRC" && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o x-ui . )
    BINARY_SRC="$TMP_SRC/x-ui"
    SCRIPT_SRC="$TMP_SRC/install-fork.sh"
    return
  fi
  if [[ "$FORK_VERSION" == "latest" ]]; then
    if TAG_JSON="$(curl -fsS --max-time 15 "https://api.github.com/repos/${FORK_REPO}/releases/latest" 2>/dev/null)"; then
      FORK_VERSION="$(printf '%s' "$TAG_JSON" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)"
    fi
    [[ "$FORK_VERSION" == v* ]] || FORK_VERSION="latest"
  fi
  if [[ "$FORK_VERSION" == "latest" ]]; then
    URL="https://github.com/${FORK_REPO}/releases/latest/download/${ASSET}"
    say "Downloading ${ASSET} (latest release) ..."
  else
    URL="https://github.com/${FORK_REPO}/releases/download/${FORK_VERSION}/${ASSET}"
    say "Downloading ${ASSET} (${FORK_VERSION}) ..."
  fi
  TMP_BIN="$(mktemp)"; trap 'rm -f "$TMP_BIN"' EXIT
  curl -fL "$URL" -o "$TMP_BIN" \
    || die "Download failed: $URL
Release the prebuilt binary once (see README), or run with --from-source."
  BINARY_SRC="$TMP_BIN"
}

write_unit() {
  cat > "$UNIT_FILE" <<UNIT
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
  systemctl daemon-reload
}

# Like the official post-install card: address, credentials, x-ui commands.
print_info() {
  local port; port="$(current_port)"
  echo
  echo -e "┌───────────────────────────────────────────────────────┐"
  echo -e "│  ${blue}Mirage-UI$(installed_version | sed 's/^/ /')${plain}  http://$(server_ip):${port}/"
  if [[ -n "${SHOW_USER:-}" ]]; then
    echo -e "│  Username: ${SHOW_USER}"
    echo -e "│  Password: ${SHOW_PASS}"
  fi
  echo -e "│                                                       │"
  echo -e "│  ${blue}x-ui${plain}         - Admin Management Script              │"
  echo -e "│  ${blue}x-ui start${plain}   - Start                                │"
  echo -e "│  ${blue}x-ui stop${plain}    - Stop                                 │"
  echo -e "│  ${blue}x-ui restart${plain} - Restart                              │"
  echo -e "│  ${blue}x-ui status${plain}  - Current Status                        │"
  echo -e "│  ${blue}x-ui log${plain}     - Check logs                            │"
  echo -e "│  ${blue}x-ui update${plain}  - Update to latest release              │"
  echo -e "└───────────────────────────────────────────────────────┘"
  echo "  Open ports in firewall AND cloud security group."
}

collect_creds_after_install() {
  if [[ -n "$SEEDED_USER" ]]; then
    SHOW_USER="$SEEDED_USER"; SHOW_PASS="$SEEDED_PASS"
  else
    local creds
    # --since: a reinstall over the same unit must not print the previous
    # first-run credentials found further down the journal.
    creds="$(journalctl -u "$SERVICE" --since "-2 min" --no-pager 2>/dev/null | grep -m1 'Username:')"
    SHOW_USER="$(sed -n 's/.*Username: *\([^ ]*\).*/\1/p' <<<"$creds")"
    SHOW_PASS="$(sed -n 's/.*Password: *\([^ ]*\).*/\1/p' <<<"$creds")"
  fi
}

# Install this script itself as the management command (official pattern).
install_manager() {
  local src="${1:-}" mgr=/usr/bin/x-ui
  if [[ -z "$src" && -f "$0" && "$0" != "bash" && "$(readlink -f "$0" 2>/dev/null)" != "$mgr" ]]; then
    src="$0"
  fi
  if [[ -z "$src" ]]; then
    src="$(mktemp)"
    curl -fsSL --max-time 30 \
      "https://raw.githubusercontent.com/${FORK_REPO}/main/install-fork.sh" -o "$src" \
      || { warn "Could not fetch the management script; re-run the installer file for the menu."; return 0; }
  fi
  install -m 0755 "$src" "$mgr"
  sed -i "s|^XUI_FOLDER_DEFAULT=.*|XUI_FOLDER_DEFAULT=${XUI_FOLDER}|;s|^SERVICE_DEFAULT=.*|SERVICE_DEFAULT=${SERVICE}|" "$mgr"
  say "Management command installed: run  x-ui  for the menu."
}

# ---------- HTTPS ----------

apply_https() {
  local user="$1" pass="$2" port
  port="$(current_port)"
  local domain email
  echo "Free HTTPS certificate via Let's Encrypt (acme.sh, HTTP-01 challenge)."
  echo "The domain must already resolve to this server and port 80 must be free."
  ask domain "Domain"
  [[ "$domain" =~ ^[A-Za-z0-9.-]+$ ]] || { warn "Invalid domain."; return; }
  ask email "Email for the ACME account"
  [[ "$email" == *@* ]] || { warn "Enter a valid email address."; return; }
  if port_in_use 80; then
    warn "Something already listens on port 80; stop it first (the challenge binds it)."
    return
  fi
  local acme_home="${ACME_HOME:-$HOME/.acme.sh}" acme
  if [[ ! -x "$acme_home/acme.sh" ]]; then
    say "Installing acme.sh ..."
    if ! curl -fsSL --max-time 60 "https://raw.githubusercontent.com/acmesh-official/acme.sh/master/acme.sh" -o /tmp/acme-install.sh \
      && ! curl -fsSL --max-time 60 https://get.acme.sh -o /tmp/acme-install.sh; then
      warn "Could not download acme.sh; aborting certificate setup."
      return
    fi
    bash /tmp/acme-install.sh --install --home "$acme_home" --accountemail "$email" >/dev/null 2>&1 \
      || { warn "acme.sh installation failed."; return; }
  fi
  acme="$acme_home/acme.sh"
  say "Requesting certificate for ${domain} (port 80 must stay reachable from the Internet) ..."
  if ! "$acme" --home "$acme_home" --server letsencrypt --issue --standalone --keylength ec-256 \
      -d "$domain" > /tmp/acme-issue.log 2>&1; then
    warn "Issuance failed. Last lines:"
    grep -vE "^\s*$" /tmp/acme-issue.log | tail -3 | sed 's/^/    /'
    warn "Check: firewall/cloud security group for port 80, DNS points here, ISP does not block 80."
    return
  fi
  mkdir -p "$XUI_FOLDER/ssl"
  "$acme" --home "$acme_home" --install-cert -d "$domain" --ecc \
    --key-file "${XUI_FOLDER}/ssl/${domain}.key" \
    --fullchain-file "${XUI_FOLDER}/ssl/${domain}.crt" \
    --reloadcmd "systemctl restart ${SERVICE}" >/dev/null 2>&1 \
    || { warn "install-cert failed; see ${acme_home}/account.conf"; return; }
  chmod 600 "${XUI_FOLDER}/ssl/${domain}.key" "${XUI_FOLDER}/ssl/${domain}.crt" 2>/dev/null || true
  apply_cert_paths "$port" "$user" "$pass" "${XUI_FOLDER}/ssl/${domain}.crt" "${XUI_FOLDER}/ssl/${domain}.key"
  echo -e "  Panel URL : ${cyan}https://${domain}:${port}/${plain}"
  echo "Renewals run automatically (acme.sh cron) and reload this service."
}

apply_cert_paths() {
  local port="$1" user="$2" pass="$3" crt="$4" key="$5"
  say "Applying the certificate in panel settings ..."
  if ! grep -q '"success":true' <<<"$(configure_panel_https "$port" "$user" "$pass" "$crt" "$key")"; then
    warn "Panel settings update failed; set the paths manually in Settings (files: ${crt})."
    return 1
  fi
  # Apply now regardless of the renewal hook; acme.sh reloadcmd also restarts later.
  systemctl restart "$SERVICE"
  sleep 8
  if curl -ks -o /dev/null -m 10 "https://127.0.0.1:${port}/"; then
    say "HTTPS is live."
  else
    warn "Not answering on https yet: journalctl -u ${SERVICE} -n 30"
  fi
}

# Official-style SSL choice.
ssl_choice_menu() {
  local user="$1" pass="$2" choice
  echo -e "${blue}SSL configuration:${plain}"
  echo "  1) Issue a free Let's Encrypt certificate for a domain (HTTP-01, port 80)"
  echo "  2) Use existing certificate files (fullchain + key paths)"
  echo "  3) Skip (HTTP for now; configure later via menu item 6)"
  ask choice "Choose an option" "3"
  case "$choice" in
    1) apply_https "$user" "$pass";;
    2)
      local crt key port; port="$(current_port)"
      ask crt "Certificate file (fullchain .crt / .pem)"
      ask key "Private key file"
      [[ -s "$crt" && -s "$key" ]] || { warn "File not found."; return; }
      apply_cert_paths "$port" "$user" "$pass" "$crt" "$key";;
    *) echo "Skipped. HTTPS can be set anytime from the management menu (option 6)."; return;;
  esac
}

# ---------- install ----------

install_fresh() {
  need_root_checks
  local script_src="${SCRIPT_SRC:-}"
  if is_tty; then
    echo -e "${cyan}Mirage-UI installation${plain}  (${FORK_REPO})"
    select_panel_port
    ask_creds
  else
    PANEL_PORT="${PANEL_PORT:-5566}"
  fi
  [[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) \
    || die "XUI_PANEL_PORT must be a number between 1 and 65535."
  if [[ -n "$SEEDED_PASS" && ${#SEEDED_PASS} -lt 8 ]]; then
    die "XUI_PASSWORD must be at least 8 characters."
  fi
  if [[ ( -n "$SEEDED_USER" && -z "$SEEDED_PASS" ) || ( -z "$SEEDED_USER" && -n "$SEEDED_PASS" ) ]]; then
    die "Set both XUI_USERNAME and XUI_PASSWORD, or neither."
  fi
  detect_asset
  download_binary "${1:-}"
  say "Installing into ${XUI_FOLDER} ..."
  mkdir -p "$XUI_FOLDER"/{bin,logs,acc}
  install -m 0755 "$BINARY_SRC" "$BINARY"
  touch "$XUI_FOLDER/x-ui-access.log"
  install_xray_core
  write_unit
  say "Starting service (port ${PANEL_PORT}) ..."
  if [[ -n "$SEEDED_USER" ]]; then
    umask 077
    printf 'XUI_USERNAME=%s\nXUI_PASSWORD=%s\n' "$SEEDED_USER" "$SEEDED_PASS" > "$INIT_ENV"
  fi
  systemctl enable --now "$SERVICE"
  sleep 5
  rm -f "$INIT_ENV"
  echo "${FORK_VERSION:-unknown}" > "$VERSION_FILE"
  collect_creds_after_install
  install_manager "$script_src"
  say "Mirage-UI $(installed_version) installation finished, it is running now..."
  print_info
  if is_tty && [[ -n "${SHOW_USER:-}" && -n "${SHOW_PASS:-}" ]]; then
    echo
    ssl_choice_menu "$SHOW_USER" "$SHOW_PASS"
  fi
}

# ---------- menu actions ----------

confirm_restart() {
  local r=""
  ask r "Restart the panel, Attention: Restarting the panel will also restart xray" "y"
  [[ "${r,,}" =~ ^(y|yes)$ ]] && systemctl restart "$SERVICE"
}

change_port() {
  local cur; cur="$(current_port)"
  while true; do
    ask PANEL_PORT "Please set up the panel port" "$cur"
    [[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) || \
      { warn "Invalid port."; continue; }
    break
  done
  write_unit
  confirm_restart
  echo -e "Your Panel Port is: ${PANEL_PORT}"
}

read_login() {
  echo "Enter the current panel login (verified against the panel; nothing is stored)."
  LOGIN_U=""; LOGIN_P=""
  read -rp "Username: " LOGIN_U
  if ! IFS= read -rsp "Password (hidden): " LOGIN_P; then echo; return 1; fi
  echo
  panel_login "$(current_port)" "$LOGIN_U" "$LOGIN_P"
}

change_credentials() {
  local port; port="$(current_port)"
  if ! read_login; then
    warn "Login rejected, or panel unreachable on port ${port}."
    warn "Forgot it? Reset: ${BINARY} setting -username <u> -password <p> && systemctl restart ${SERVICE}"
    return
  fi
  local nu="$LOGIN_U" p1="" p2=""
  read -rp "New username [${LOGIN_U}]: " x; [[ -n "$x" ]] && nu="$x"
  while true; do
    IFS= read -rsp "New password (min 8, hidden; Enter keeps current): " p1; echo
    [[ -z "$p1" ]] && { p1="$LOGIN_P"; break; }
    [[ ${#p1} -ge 8 ]] || { warn "Too short (min 8 characters)."; continue; }
    IFS= read -rsp "Repeat password: " p2; echo
    [[ "$p1" == "$p2" ]] || { warn "Passwords do not match."; continue; }
    break
  done
  local body res
  body="$(printf '{"oldUsername":"%s","oldPassword":"%s","newUsername":"%s","newPassword":"%s"}' \
    "$(json_escape "$LOGIN_U")" "$(json_escape "$LOGIN_P")" "$(json_escape "$nu")" "$(json_escape "$p1")")"
  res="$(panel_post "$port" "$LOGIN_U" "$LOGIN_P" /panel/api/setting/updateUser "$body")"
  if grep -q '"success":true' <<<"$res"; then
    say "Login updated. Use ${nu} and the new password on the next sign-in."
  else
    warn "Update failed: ${res}"
  fi
}

show_settings() {
  echo -e "${blue}Current settings:${plain}"
  echo "  Version    : $(installed_version)"
  echo "  Folder     : ${XUI_FOLDER}"
  echo "  Service    : ${SERVICE} ($(systemctl is-active "$SERVICE" 2>/dev/null || echo stopped), autostart: $(systemctl is-enabled "$SERVICE" 2>/dev/null || echo ?))"
  echo "  Panel URL  : http://$(server_ip):$(current_port)/"
}

update_fork() {
  need_root_checks
  detect_asset
  local backup="${BINARY}.bak"
  FORK_VERSION="latest"
  download_binary
  say "Updating binary (previous kept at ${backup}) ..."
  systemctl stop "$SERVICE"
  cp -f "$BINARY" "$backup" 2>/dev/null || true
  install -m 0755 "$BINARY_SRC" "$BINARY"
  echo "${FORK_VERSION}" > "$VERSION_FILE"
  systemctl start "$SERVICE"
  sleep 4
  if systemctl is-active --quiet "$SERVICE"; then
    say "Updated to release ${FORK_VERSION}."
  else
    warn "New binary failed to start; rolling back."
    mv -f "$backup" "$BINARY"
    systemctl start "$SERVICE"
  fi
}

uninstall() {
  local wipe=""
  echo -e "${red}This stops and removes Mirage-UI.${plain}"
  ask wipe "Type YES to also delete the data folder (${XUI_FOLDER}, incl. database); anything else keeps it" ""
  systemctl stop "$SERVICE" 2>/dev/null || true
  systemctl disable "$SERVICE" 2>/dev/null || true
  rm -f "$UNIT_FILE" /usr/bin/x-ui
  systemctl daemon-reload
  if [[ "$wipe" == "YES" ]]; then
    rm -rf "$XUI_FOLDER"
    say "Removed unit, management command and ${XUI_FOLDER}."
  else
    say "Removed unit and management command; data kept in ${XUI_FOLDER}."
  fi
}

menu_https() {
  if ! read_login; then
    warn "Login rejected, or panel unreachable on port $(current_port)."
    return
  fi
  ssl_choice_menu "$LOGIN_U" "$LOGIN_P"
}

before_show_menu() {
  echo; read -rp "Press enter to return to the main menu: " _ || return
}

show_menu() {
  clear 2>/dev/null || true
  echo -e "╔────────────────────────────────────────────────╗"
  echo -e "║  ${green}Mirage-UI Management$(installed_version | sed 's/^/ /' | cut -c1-24)${plain}"
  echo -e "║  $(systemctl is-active "$SERVICE" 2>/dev/null || echo stopped)  port $(current_port)  ${XUI_FOLDER}"
  echo -e "╟────────────────────────────────────────────────╢"
  echo -e "║  ${green} 1.${plain} Update to latest release                 ║"
  echo -e "║  ${green} 2.${plain} Uninstall                                ║"
  echo -e "╟────────────────────────────────────────────────╢"
  echo -e "║  ${green} 3.${plain} Reset Username & Password                ║"
  echo -e "║  ${green} 4.${plain} Change Port                              ║"
  echo -e "║  ${green} 5.${plain} View Current Settings                    ║"
  echo -e "║  ${green} 6.${plain} SSL Certificate (issue / renew / set)    ║"
  echo -e "╟────────────────────────────────────────────────╢"
  echo -e "║  ${green} 7.${plain} Start      ${green} 8.${plain} Stop                     ║"
  echo -e "║  ${green} 9.${plain} Restart    ${green}10.${plain} Check Status               ║"
  echo -e "║  ${green}11.${plain} Check Logs ${green}12.${plain} Enable/Disable Autostart   ║"
  echo -e "╟────────────────────────────────────────────────╢"
  echo -e "║  ${green} 0.${plain} Exit Script                              ║"
  echo -e "╚════════════════════════════════════════════════╝"
}

menu() {
  local choice
  while true; do
    show_menu
    if ! IFS= read -rp "Enter an option: " choice; then return; fi
    case "$choice" in
      1) update_fork;;
      2) uninstall; return;;
      3) change_credentials;;
      4) change_port;;
      5) show_settings;;
      6) menu_https; continue;;
      7) systemctl start "$SERVICE";;
      8) systemctl stop "$SERVICE";;
      9) confirm_restart;;
      10) systemctl status "$SERVICE" --no-pager | head -12;;
      11) echo "Ctrl+C stops following; showing the last 30 lines:"; journalctl -u "$SERVICE" -n 30 --no-pager; choice="";;
      12)
        if [[ "$(systemctl is-enabled "$SERVICE" 2>/dev/null || echo ?)" == enabled ]]; then
          systemctl disable "$SERVICE" && say "Autostart disabled."
        else
          systemctl enable "$SERVICE" && say "Autostart enabled."
        fi;;
      0|"") return;;
      *) warn "Unknown option.";;
    esac
    [[ -n "$choice" ]] && before_show_menu
  done
}

show_usage() {
  echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}x-ui control menu usages (subcommands):${plain}              │
│                                                       │
│  ${blue}x-ui${plain}              - Admin Management Script          │
│  ${blue}x-ui start${plain}        - Start                            │
│  ${blue}x-ui stop${plain}         - Stop                             │
│  ${blue}x-ui restart${plain}      - Restart                          │
│  ${blue}x-ui status${plain}       - Current Status                   │
│  ${blue}x-ui settings${plain}     - Current Settings                 │
│  ${blue}x-ui enable${plain}       - Enable Autostart on OS Startup   │
│  ${blue}x-ui disable${plain}      - Disable Autostart on OS Startup  │
│  ${blue}x-ui log${plain}          - Check logs                       │
│  ${blue}x-ui update${plain}       - Update                           │
│  ${blue}x-ui install${plain}      - Install                          │
│  ${blue}x-ui uninstall${plain}    - Uninstall                        │
└───────────────────────────────────────────────────────┘"
}

# ---------- main ----------

CMD="${1:-}"
case "$CMD" in
  help|--help|-h) show_usage; exit 0;;
  start|stop|restart|status|settings|enable|disable|log|update|uninstall)
    [[ -x "$BINARY" ]] || die "Not installed yet; run the installer first."
    need_root_checks
    case "$CMD" in
      start)     systemctl start "$SERVICE";;
      stop)      systemctl stop "$SERVICE";;
      restart)   systemctl restart "$SERVICE";;
      status)    systemctl status "$SERVICE" --no-pager | head -12;;
      settings)  show_settings;;
      enable)    systemctl enable --now "$SERVICE";;
      disable)   systemctl disable --now "$SERVICE";;
      log)       journalctl -u "$SERVICE" -n 100 -f;;
      update)    update_fork;;
      uninstall) uninstall;;
    esac
    exit 0;;
esac

if [[ "$CMD" == "--from-source" || "$CMD" == "install" || -n "$PANEL_PORT" || ! -x "$BINARY" ]]; then
  install_fresh "$CMD"
elif is_tty; then
  need_root_checks
  menu
else
  print_info
  warn "Already installed. Run x-ui from a terminal for the management menu."
fi
