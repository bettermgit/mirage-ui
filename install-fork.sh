#!/usr/bin/env bash
#
# Mirage-UI installer & manager
#
#   curl -fsSL https://raw.githubusercontent.com/bettermgit/mirage-ui/main/install-fork.sh | bash
#
# First run installs; running it again opens the management menu.
# Fully non-interactive when stdin is not a terminal or answers are
# provided up front:
#
#   FORK_REPO=bettermgit/mirage-ui  Releases to download from
#   FORK_VERSION=v1.0.0             release tag (default: latest)
#   XUI_PANEL_PORT=5566             panel port (implies unattended install)
#   XUI_USERNAME + XUI_PASSWORD     fixed first login (else: asked / generated)
#   XUI_MAIN_FOLDER=/usr/local/x-ui install folder
set -euo pipefail

FORK_REPO="${FORK_REPO:-bettermgit/mirage-ui}"
FORK_VERSION="${FORK_VERSION:-latest}"
PANEL_PORT="${XUI_PANEL_PORT:-}"
SEEDED_USER="${XUI_USERNAME:-}"
SEEDED_PASS="${XUI_PASSWORD:-}"
XUI_FOLDER="${XUI_MAIN_FOLDER:-/usr/local/x-ui}"
SERVICE=x-ui
INIT_ENV="${XUI_FOLDER}/acc/initial.env"
UNIT_FILE="/etc/systemd/system/${SERVICE}.service"
BINARY="${XUI_FOLDER}/x-ui"

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; cyan='\033[0;36m'; plain='\033[0m'
say()  { echo -e "${green}==> $*${plain}"; }
warn() { echo -e "${yellow}==> $*${plain}"; }
die()  { echo -e "${red}==> $*${plain}" >&2; exit 1; }

is_tty() { [[ -t 0 && -t 1 ]]; }

# ask <var> <prompt> <default>: Enter takes the default; empty default loops.
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

ask_port() {
  while true; do
    ask PANEL_PORT "Panel port" "${1:-5566}"
    [[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) || \
      { warn "Port must be a number between 1 and 65535."; continue; }
    if ss -Hltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${PANEL_PORT}\$"; then
      warn "Port ${PANEL_PORT} is already in use on this machine."
      continue
    fi
    return 0
  done
}

# ask_creds: fixed via env, or asked (hidden, twice, confirmed), or generated.
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

# panel_login <port> <user> <pass>: prints nothing, exit 0 when credentials work.
panel_login() {
  local base="http://127.0.0.1:$1" jar tok out
  jar="$(mktemp)"
  tok="$(curl -s -m 10 -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  out="$(curl -s -m 10 -b "$jar" -X POST "$base/login" -H "X-CSRF-Token: $tok" \
    --data-urlencode "username=$2" --data-urlencode "password=$3" || true)"
  rm -f "$jar"
  grep -q '"success":true' <<<"$out"
}

# panel_post <port> <user> <pass> <path> <json>: logged-in POST, prints body.
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

need_root_checks() {
  [[ $EUID -eq 0 ]] || die "Run as root (or via sudo)."
  command -v systemctl >/dev/null || die "systemd is required."
  command -v curl >/dev/null || die "curl is required (apt install curl)."
}

detect_asset() {
  case "$(uname -m)" in
    x86_64)  ASSET=x-ui-linux-amd64 ;;
    aarch64) ASSET=x-ui-linux-arm64 ;;
    *) die "Unsupported architecture: $(uname -m) (amd64/arm64 only)" ;;
  esac
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
    return
  fi
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

print_info() {
  local port; port="$(current_port)"
  echo -e "${cyan}================================================${plain}"
  echo -e "  ${cyan}Mirage-UI${plain}  (port ${port})"
  echo -e "${cyan}------------------------------------------------${plain}"
  echo -e "  Panel URL : http://$(server_ip):${port}/"
  if [[ -n "${SHOW_USER:-}" ]]; then
    echo -e "  Username  : ${SHOW_USER}"
    echo -e "  Password  : ${SHOW_PASS}"
  fi
  echo -e "${cyan}------------------------------------------------${plain}"
  echo -e "  Open ports in firewall AND cloud security group."
  echo -e "  Re-run this script from a terminal for the menu."
  echo -e "${cyan}================================================${plain}"
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

install_fresh() {
  need_root_checks
  if is_tty; then
    echo -e "${cyan}Mirage-UI installation${plain}  (${FORK_REPO})"
    ask_port "5566"
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
  write_unit
  say "Starting service (port ${PANEL_PORT}) ..."
  if [[ -n "$SEEDED_USER" ]]; then
    umask 077
    printf 'XUI_USERNAME=%s\nXUI_PASSWORD=%s\n' "$SEEDED_USER" "$SEEDED_PASS" > "$INIT_ENV"
  fi
  systemctl enable --now "$SERVICE"
  sleep 5
  rm -f "$INIT_ENV"
  collect_creds_after_install
  say "Done."
  print_info
}

change_port() {
  local cur; cur="$(current_port)"
  ask_port "$cur"
  write_unit
  systemctl restart "$SERVICE"
  say "Panel now on port ${PANEL_PORT}."
}

change_credentials() {
  local port; port="$(current_port)"
  local user pass
  echo "Enter the current panel login (verified against the panel; nothing is stored)."
  read -rp "Username: " user
  if ! IFS= read -rsp "Password (hidden): " pass; then echo; return; fi
  echo
  if ! panel_login "$port" "$user" "$pass"; then
    warn "Login rejected, or panel unreachable on port ${port}."
    warn "Forgot it? Reset: ${BINARY} setting -username <u> -password <p> && systemctl restart ${SERVICE}"
    return
  fi
  local nu="$user" p1="" p2=""
  read -rp "New username [${user}]: " x; [[ -n "$x" ]] && nu="$x"
  while true; do
    IFS= read -rsp "New password (min 8, hidden; Enter keeps current): " p1; echo
    [[ -z "$p1" ]] && { p1="$pass"; break; }
    [[ ${#p1} -ge 8 ]] || { warn "Too short (min 8 characters)."; continue; }
    IFS= read -rsp "Repeat password: " p2; echo
    [[ "$p1" == "$p2" ]] || { warn "Passwords do not match."; continue; }
    break
  done
  local body res
  body="$(printf '{"oldUsername":"%s","oldPassword":"%s","newUsername":"%s","newPassword":"%s"}' \
    "$(json_escape "$user")" "$(json_escape "$pass")" "$(json_escape "$nu")" "$(json_escape "$p1")")"
  res="$(panel_post "$port" "$user" "$pass" /panel/api/setting/updateUser "$body")"
  if grep -q '"success":true' <<<"$res"; then
    say "Login updated. Use ${nu} and the new password on the next sign-in."
  else
    warn "Update failed: ${res}"
  fi
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
  rm -f "$UNIT_FILE"
  systemctl daemon-reload
  if [[ "$wipe" == "YES" ]]; then
    rm -rf "$XUI_FOLDER"
    say "Removed unit and ${XUI_FOLDER}."
  else
    say "Removed unit; data kept in ${XUI_FOLDER}."
  fi
}

menu() {
  local choice
  while true; do
    clear 2>/dev/null || true
    echo -e "${cyan}Mirage-UI${plain}  $(systemctl is-active "$SERVICE" 2>/dev/null || echo stopped)  port $(current_port)  ${XUI_FOLDER}"
    echo "  1) Start                    2) Stop"
    echo "  3) Restart                  4) Status"
    echo "  5) Change port              6) Change login (user/password)"
    echo "  7) Show access info         8) Update to latest release"
    echo "  9) Uninstall                0) Quit"
    if ! IFS= read -rp "> " choice; then return; fi
    case "$choice" in
      1) systemctl start "$SERVICE";;
      2) systemctl stop "$SERVICE";;
      3) systemctl restart "$SERVICE";;
      4) systemctl status "$SERVICE" --no-pager | head -12;;
      5) change_port;;
      6) change_credentials;;
      7) print_info;;
      8) update_fork;;
      9) uninstall; return;;
      0|"") return;;
      *) warn "Unknown option.";;
    esac
    if [[ -n "$choice" ]]; then echo; read -rp "Press Enter to continue... " _ || return; fi
  done
}

# ---------- main ----------

if [[ "${1:-}" == "--from-source" || -n "$PANEL_PORT" || ! -x "$BINARY" ]]; then
  install_fresh "${1:-}"
elif is_tty; then
  need_root_checks
  menu
else
  print_info
  warn "Already installed. Run this script from a terminal for the management menu."
fi
