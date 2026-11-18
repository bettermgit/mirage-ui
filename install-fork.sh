#!/usr/bin/env bash
#
# Mirage-UI installer & management script
# Mirage-UI 安装 & 管理脚本（交互界面默认中文）
#
#   curl -fsSL https://raw.githubusercontent.com/bettermgit/mirage-ui/main/install-fork.sh | bash
#
# Language: Chinese UI by default; run with XUI_LANG=en for English.
# 语言：默认中文交互；XUI_LANG=en 切换为英文。
#
# Like the 3x-ui original: after installing, this script is placed at
# /usr/bin/m-ui, so a bare  m-ui  opens the management menu and
#   m-ui start|stop|restart|status|settings|enable|disable|log|update|uninstall
# work directly. Non-interactive when stdin is not a terminal or when all
# answers are provided up front:
#
#   FORK_REPO=bettermgit/mirage-ui  Releases to download from
#   FORK_VERSION=v1.0.4             release tag (default: latest)
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

# ---------- i18n ----------
# Server locales are typically en_US even for Chinese admins, so zh is the
# default; English stays one env var away (values with %s fill in via
# ${msg//%s/value}).
# 服务器 LANG 多为 en_US，无法靠 locale 自动判断，故默认中文；
# 英文随时可用 XUI_LANG=en 切换。
L=zh
case "${XUI_LANG:-}" in [Ee][Nn]|[Ee]nglish) L=en;; esac

t() {
  local key="$1"
  if [[ "$L" == en ]]; then printf '%s' "${EN[$key]-}"; else printf '%s' "${ZH[$key]-}"; fi
}

fmt() {   # fmt <key> <value>: replace %s placeholder
  local msg; msg="$(t "$1")"; printf '%s' "${msg//%s/$2}"
}

declare -A EN=(
  [v_required]="A value is required."
  [need_root]="Run as root (or via sudo)."
  [need_systemd]="systemd is required."
  [need_curl]="curl is required (apt install curl)."
  [arch_bad]="Unsupported architecture: %s (amd64/arm64 only)"
  [need_go]="Go 1.27+ required for --from-source (https://go.dev/dl)."
  [need_npm]="Node 26+ / npm required for --from-source."
  [port_q]="Would you like to customize the Panel Port settings? (If not, a random port will be applied)"
  [port_prompt]="Please set up the panel port"
  [port_bad]="Invalid port. Enter a number between 1 and 65535."
  [port_used]="Port %s is already in use. Choose another panel port."
  [port_yours]="Your Panel Port is: %s"
  [port_rand]="Generated random port: %s"
  [port_norand]="Could not find an available random port."
  [env_port_bad]="XUI_PANEL_PORT must be a number between 1 and 65535."
  [env_pass_bad]="XUI_PASSWORD must be at least 8 characters."
  [env_pair]="Set both XUI_USERNAME and XUI_PASSWORD, or neither."
  [creds_q]="Set your own username and password? (N = random, shown at the end)"
  [username]="Username"
  [username_colon]="Username: "
  [pass_hidden]="Password (min 8 chars, hidden): "
  [pass_again]="Repeat password: "
  [pass_short]="Too short (min 8 characters)."
  [pass_nomatch]="Passwords do not match."
  [install_hdr]="Mirage-UI installation"
  [cloning]="Cloning %s ..."
  [dl_fail1]="Download failed: %s"
  [dl_fail2]="Release the prebuilt binary once (see README), or run with --from-source."
  [downloading]="Downloading"
  [latest_rel]="latest release"
  [xray_dl]="Downloading xray core (latest release) ..."
  [xray_dl_fail]="xray download failed; skipped (panel Settings can install it later)."
  [xray_ext_fail]="xray extract failed; skipped."
  [xray_tool]="Neither unzip nor python3 available; xray skipped (panel Settings can install it later)."
  [xray_ok]="xray core installed."
  [installing_into]="Installing into %s ..."
  [starting_svc]="Starting service (port %s) ..."
  [finished]="Mirage-UI %s installation finished, it is running now..."
  [mgr_installed]="Management command installed: run  m-ui  for the menu."
  [mgr_fetch]="Could not fetch the management script; re-run the installer file for the menu."
  [not_installed]="Not installed yet; run the installer first."
  [already_installed]="Already installed. Run m-ui from a terminal for the management menu."
  [pi_url]="Panel URL"
  [pi_user]="Username"
  [pi_pass]="Password"
  [pi_manage]="Manage"
  [pi_manage_desc]="(menu) / m-ui start|stop|status|log ..."
  [pi_https]="HTTPS"
  [pi_https_desc]="(Let's Encrypt, one shot)"
  [pi_firewall]="  Open the port in firewall AND cloud security group."
  [ssl_title]="SSL configuration:"
  [ssl_opt1]="  1) Issue a free Let's Encrypt certificate for a domain (HTTP-01, port 80)"
  [ssl_opt2]="  2) Use existing certificate files (fullchain + key paths)"
  [ssl_opt3]="  3) Skip (HTTP for now; configure later via menu item 6)"
  [ssl_choice]="Choose an option"
  [ssl_skipped]="Skipped. HTTPS can be set anytime from the management menu (option 6)"
  [https_intro1]="Free HTTPS certificate via Let's Encrypt (acme.sh, HTTP-01 challenge)."
  [https_intro2]="The domain must already resolve to this server and port 80 must be free."
  [domain_prompt]="Domain"
  [domain_bad]="Invalid domain."
  [email_prompt]="Email for the ACME account"
  [email_bad]="Enter a valid email address."
  [port80_used]="Something already listens on port 80; stop it first (the challenge binds it)."
  [acme_installing]="Installing acme.sh ..."
  [acme_fail_install]="acme.sh installation failed; check network to get.acme.sh"
  [acme_requesting]="Requesting certificate for %s (port 80 must stay reachable from the Internet) ..."
  [acme_failed]="Issuance failed. Last lines:"
  [acme_check]="Check: firewall/cloud security group for port 80, DNS points here, ISP does not block 80."
  [acme_exists]="A valid certificate already exists; installing it as-is."
  [acme_installcert_fail]="install-cert failed; see %s"
  [cert_py_missing]="python3 missing; set cert paths manually in Settings (files: %s)."
  [cert_applying]="Applying the certificate in panel settings ..."
  [cert_db_fail]="Could not write the settings; set the paths manually in Settings (files: %s)."
  [cert_live]="HTTPS is live."
  [cert_notlive]="Not answering on https yet: journalctl -u %s -n 30"
  [cert_renew]="Renewals run automatically (acme.sh cron) and reload this service."
  [crt_file]="Certificate file (fullchain .crt / .pem)"
  [crt_key]="Private key file"
  [crt_missing]="File not found."
  [restart_q]="Restart the panel, Attention: Restarting the panel will also restart xray"
  [login_intro]="Enter the current panel login (verified against the panel; nothing is stored)."
  [pass_hidden2]="Password (hidden): "
  [login_fail]="Login rejected, or panel unreachable on port %s."
  [login_forget]="Forgot it? Reset: %s setting -username <u> -password <p> && systemctl restart %s"
  [new_user_q]="New username [%s]: "
  [new_pass_q]="New password (min 8, hidden; Enter keeps current): "
  [login_ok]="Login updated. Use %s and the new password on the next sign-in."
  [login_upd_fail]="Update failed: %s"
  [set_title]="Current settings:"
  [set_folder]="Folder"
  [set_svc]="Service"
  [set_cert]="Certificate"
  [upd_running]="Updating binary (previous kept at %s) ..."
  [upd_ok]="Updated to release %s."
  [upd_fail]="New binary failed to start; rolling back."
  [uninst_warn]="This stops and removes Mirage-UI."
  [uninst_q]="Type YES to also delete the data folder (%s, incl. database); anything else keeps it"
  [uninst_all]="Removed unit, management command and %s."
  [uninst_keep]="Removed unit and management command; data kept in %s."
  [enter_back]="Press enter to return to the main menu: "
  [menu_mgmt]="Management Script"
  [m1]="Update to latest release"
  [m2]="Uninstall"
  [m3]="Reset Username & Password"
  [m4]="Change Port"
  [m5]="View Current Settings"
  [m6]="SSL Certificate (issue / renew / set)"
  [m7]="Start"
  [m8]="Stop"
  [m9]="Restart"
  [m10]="Check Status"
  [m11]="Check Logs"
  [m12]="Enable/Disable Autostart"
  [m0]="Exit Script"
  [menu_prompt]="Enter an option: "
  [opt_unknown]="Unknown option."
  [log_hint]="Ctrl+C stops following; showing the last 30 lines:"
  [autostart_off]="Autostart disabled."
  [autostart_on]="Autostart enabled."
)

declare -A ZH=(
  [v_required]="必须输入一个值。"
  [need_root]="请以 root 身份运行（或使用 sudo）。"
  [need_systemd]="需要 systemd。"
  [need_curl]="需要 curl（apt install curl）。"
  [arch_bad]="不支持的架构：%s（仅支持 amd64/arm64）"
  [need_go]="使用 --from-source 需要 Go 1.27+（https://go.dev/dl）。"
  [need_npm]="使用 --from-source 需要 Node 26+ / npm。"
  [port_q]="是否自定义面板端口？（不定制则自动选择随机端口）"
  [port_prompt]="请设置面板端口"
  [port_bad]="端口无效，请输入 1-65535 之间的数字。"
  [port_used]="端口 %s 已被占用，请换一个面板端口。"
  [port_yours]="您的面板端口是：%s"
  [port_rand]="已生成随机端口：%s"
  [port_norand]="找不到可用的随机端口。"
  [env_port_bad]="XUI_PANEL_PORT 必须是 1-65535 之间的数字。"
  [env_pass_bad]="XUI_PASSWORD 至少要有 8 位。"
  [env_pair]="XUI_USERNAME 和 XUI_PASSWORD 要同时设置，或者都不设置。"
  [creds_q]="是否自定义用户名和密码？（选 N 则随机生成并在结束时显示）"
  [username]="用户名"
  [username_colon]="用户名: "
  [pass_hidden]="密码（至少 8 位，输入不回显）: "
  [pass_again]="再次输入密码: "
  [pass_short]="太短（至少 8 位）。"
  [pass_nomatch]="两次输入的密码不一致。"
  [install_hdr]="Mirage-UI 安装"
  [cloning]="正在克隆 %s ..."
  [dl_fail1]="下载失败：%s"
  [dl_fail2]="请先发布预编译二进制（见 README），或使用 --from-source 从源码编译。"
  [downloading]="正在下载"
  [latest_rel]="最新 release"
  [xray_dl]="正在下载 xray 内核（最新 release）..."
  [xray_dl_fail]="xray 下载失败，已跳过（之后可在面板 Settings 页安装）。"
  [xray_ext_fail]="xray 解包失败，已跳过。"
  [xray_tool]="unzip 和 python3 都不可用，已跳过 xray（之后可在面板 Settings 页安装）。"
  [xray_ok]="xray 内核已安装。"
  [installing_into]="正在安装到 %s ..."
  [starting_svc]="正在启动服务（端口 %s）..."
  [finished]="Mirage-UI %s 安装完成，服务已在运行..."
  [mgr_installed]="管理命令已安装：运行  m-ui  即可打开菜单。"
  [mgr_fetch]="无法获取管理脚本；重新运行安装脚本文件即可打开菜单。"
  [not_installed]="尚未安装；请先运行安装脚本。"
  [already_installed]="已安装。在终端里运行 x-ui 可打开管理菜单。"
  [pi_url]="面板地址"
  [pi_user]="用户名"
  [pi_pass]="密码"
  [pi_manage]="管理"
  [pi_manage_desc]="（菜单）/ m-ui start|stop|status|log …"
  [pi_https]="HTTPS"
  [pi_https_desc]="（一键申请 Let's Encrypt 证书）"
  [pi_firewall]="  请在防火墙和云安全组中同时放行该端口。"
  [ssl_title]="SSL 证书配置："
  [ssl_opt1]="  1) 为域名签发免费的 Let's Encrypt 证书（HTTP-01，需 80 端口）"
  [ssl_opt2]="  2) 使用已有的证书文件（fullchain + 私钥路径）"
  [ssl_opt3]="  3) 跳过（暂时 HTTP，之后通过菜单选项 6 配置）"
  [ssl_choice]="请选择"
  [ssl_skipped]="已跳过。之后可随时在管理菜单（选项 6）配置 HTTPS"
  [https_intro1]="通过 Let's Encrypt 免费申请 HTTPS 证书（acme.sh，HTTP-01 验证）。"
  [https_intro2]="域名必须已解析到本机，且 80 端口必须空闲。"
  [domain_prompt]="域名"
  [domain_bad]="域名格式不正确。"
  [email_prompt]="ACME 账户邮箱"
  [email_bad]="请输入有效的邮箱地址。"
  [port80_used]="80 端口已有程序在监听；请先停掉它（验证时会临时占用 80）。"
  [acme_installing]="正在安装 acme.sh ..."
  [acme_fail_install]="acme.sh 安装失败；请检查到 get.acme.sh 的网络连通性"
  [acme_requesting]="正在为 %s 申请证书（80 端口需保持公网可达）..."
  [acme_failed]="签发失败，最后几行日志："
  [acme_check]="排查方向：80 端口的防火墙/云安全组、DNS 是否指向本机、运营商是否封锁 80。"
  [acme_exists]="已有有效证书，直接安装使用。"
  [acme_installcert_fail]="install-cert 失败；请查看 %s"
  [cert_py_missing]="缺少 python3；请在面板 Settings 中手动设置证书路径（文件：%s）。"
  [cert_applying]="正在把证书写入面板设置 ..."
  [cert_db_fail]="写入设置失败；请在 Settings 中手动设置（文件：%s）。"
  [cert_live]="HTTPS 已生效。"
  [cert_notlive]="https 尚无响应：journalctl -u %s -n 30"
  [cert_renew]="证书续期由 acme.sh 定时任务自动完成，并自动重启本服务。"
  [crt_file]="证书文件（fullchain .crt / .pem）"
  [crt_key]="私钥文件"
  [crt_missing]="找不到文件。"
  [restart_q]="重启面板，注意：重启面板会同时重启 xray"
  [login_intro]="输入当前面板账号（向面板实时验证；不做任何存储）。"
  [pass_hidden2]="密码（不回显）: "
  [login_fail]="登录被拒绝，或面板在端口 %s 上不可达。"
  [login_forget]="忘记密码？重置命令：%s setting -username <u> -password <p> && systemctl restart %s"
  [new_user_q]="新用户名 [%s]: "
  [new_pass_q]="新密码（至少 8 位，不回显；直接回车保持当前密码）: "
  [login_ok]="账号已更新。下次登录请使用 %s 和新密码。"
  [login_upd_fail]="更新失败：%s"
  [set_title]="当前设置："
  [set_folder]="安装目录"
  [set_svc]="服务"
  [set_cert]="证书"
  [upd_running]="正在更新二进制（旧版本保留在 %s）..."
  [upd_ok]="已更新到 release %s。"
  [upd_fail]="新二进制无法启动，正在回滚。"
  [uninst_warn]="即将停止并移除 Mirage-UI。"
  [uninst_q]="输入 YES 同时删除数据目录（%s，含数据库）；输入其他内容则保留数据"
  [uninst_all]="已移除服务单元、管理命令和 %s。"
  [uninst_keep]="已移除服务单元和管理命令；数据保留在 %s。"
  [enter_back]="按回车返回主菜单: "
  [menu_mgmt]="管理脚本"
  [m1]="更新到最新 release"
  [m2]="卸载"
  [m3]="重置账号密码"
  [m4]="更换端口"
  [m5]="查看当前设置"
  [m6]="SSL 证书（签发 / 续期 / 设置）"
  [m7]="启动"
  [m8]="停止"
  [m9]="重启"
  [m10]="查看状态"
  [m11]="查看日志"
  [m12]="开/关开机自启"
  [m0]="退出脚本"
  [menu_prompt]="请输入选项: "
  [opt_unknown]="无效选项。"
  [log_hint]="按 Ctrl+C 结束；显示最近 30 行日志："
  [autostart_off]="开机自启已关闭。"
  [autostart_on]="开机自启已开启。"
)

red='\033[0;31m'; green='\033[0;32m'; yellow='\033[0;33m'; blue='\033[0;34m'; cyan='\033[0;36m'; plain='\033[0m'
say()  { echo -e "${green}==> $*${plain}"; }
warn() { echo -e "${yellow}==> $*${plain}"; }
die()  { echo -e "${red}==> $*${plain}" >&2; exit 1; }

is_tty() { [[ -t 0 && -t 1 ]]; }

installed_version() { [[ -f "$VERSION_FILE" ]] && cat "$VERSION_FILE" || echo "unknown"; }

ask() {
  local __var="$1" __prompt="$2" __default="${3:-}" __val
  while true; do
    if ! IFS= read -rp "${__prompt}${__default:+ [${__default}]}: " __val; then echo; __val=""; fi
    __val="${__val:-$__default}"
    [[ -n "$__val" || -n "$__default" ]] || { warn "$(t v_required)"; continue; }
    printf -v "$__var" '%s' "$__val"
    return 0
  done
}

confirm_yes() { [[ "${1,,}" =~ ^(y|yes)$ ]]; }

port_in_use() { ss -Hltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]${1}\$"; }

# Sets PANEL_PORT after a validated prompt (range only; in-use checks differ
# between fresh install, where nothing listens, and change, where we do).
read_port() {
  while true; do
    ask PANEL_PORT "$(t port_prompt)" "$1"
    [[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) && return
    warn "$(t port_bad)"
  done
}

# Official flow: y/N customize, N picks a random free port.
select_panel_port() {
  local confirm=""
  ask confirm "$(t port_q)" "n"
  if confirm_yes "$confirm"; then
    while true; do
      read_port 5566
      if port_in_use "$PANEL_PORT"; then
        warn "$(fmt port_used "$PANEL_PORT")"
        continue
      fi
      echo -e "$(fmt port_yours "$PANEL_PORT")"
      return 0
    done
  fi
  local attempt
  for attempt in {1..100}; do
    PANEL_PORT=$(( RANDOM % (62000 - 1024) + 1024 ))
    if ! port_in_use "$PANEL_PORT"; then
      echo -e "$(fmt port_rand "$PANEL_PORT")"
      return 0
    fi
  done
  die "$(t port_norand)"
}

ask_creds() {
  [[ -n "$SEEDED_USER" ]] && return 0
  local fix=""
  ask fix "$(t creds_q)" "n"
  confirm_yes "$fix" || return 0
  ask SEEDED_USER "$(t username)" "admin"
  local p1="" p2=""
  while true; do
    IFS= read -rsp "$(t pass_hidden)" p1 || { echo; exit 1; }; echo
    IFS= read -rsp "$(t pass_again)" p2 || { echo; exit 1; }; echo
    [[ ${#p1} -ge 8 ]] || { warn "$(t pass_short)"; continue; }
    [[ "$p1" == "$p2" ]] || { warn "$(t pass_nomatch)"; continue; }
    SEEDED_PASS="$p1"; return 0
  done
}

server_ip() { curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}'; }

current_port() {
  [[ -f "$UNIT_FILE" ]] || { echo "2053"; return; }
  grep -m1 -oE 'XUI_PORT=[0-9]+' "$UNIT_FILE" | cut -d= -f2 || echo 2053
}

# panel_api <port> <user> <pass> <path> [json-body]: login with CSRF and POST
# the path; prints the response. No body = login check only.
panel_api() {
  local port="$1" user="$2" pass="$3" path="$4" body="${5:-}"
  local base="http://127.0.0.1:$port" jar tok
  jar="$(mktemp)"
  tok="$(curl -s -m 10 -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
  curl -s -m 10 -b "$jar" -c "$jar" -X POST "$base/login" -H "X-CSRF-Token: $tok" \
    --data-urlencode "username=$user" --data-urlencode "password=$pass"
  if [[ -n "$body" ]]; then
    tok="$(curl -s -m 10 -b "$jar" -c "$jar" "$base/csrf-token" | sed -n 's/.*"obj":"\([^"]*\)".*/\1/p')"
    curl -s -m 15 -b "$jar" -X POST "$base$path" -H "X-CSRF-Token: $tok" \
      -H "Content-Type: application/json" -d "$body"
  fi
  rm -f "$jar"
}

panel_login() { local out; out="$(panel_api "$1" "$2" "$3" "")"; grep -q '"success":true' <<<"$out"; }

json_escape() { local s="$1"; s="${s//\\/\\\\}"; s="${s//\"/\\\"}"; printf '%s' "$s"; }

# configure_panel_https <crt> <key>: write only these two settings rows.
# The full settings API is unsafe here: its webPort reflects the DB value,
# not the XUI_PORT override, and a full roundtrip would persist that drift.
configure_panel_https() {
  CRT="$1" KEY="$2" DB="${XUI_DB_FOLDER:-$XUI_FOLDER}/x-ui.db" python3 - <<'PY'
import os, sqlite3
db = sqlite3.connect(os.environ["DB"])
for k, v in (("webCertFile", os.environ["CRT"]), ("webKeyFile", os.environ["KEY"])):
    if not db.execute("UPDATE settings SET value=? WHERE key=?", (v, k)).rowcount:
        db.execute("INSERT INTO settings (key, value) VALUES (?,?)", (k, v))
db.commit()
PY
}

apply_cert_paths() {
  local crt="$1" key="$2" port
  if ! command -v python3 >/dev/null; then
    warn "$(fmt cert_py_missing "$crt")"
    return 1
  fi
  say "$(t cert_applying)"
  if ! configure_panel_https "$crt" "$key"; then
    warn "$(fmt cert_db_fail "$crt")"
    return 1
  fi
  # Apply now regardless of the renewal hook; acme.sh reloadcmd also restarts later.
  systemctl restart "$SERVICE"
  sleep 8
  port="$(current_port)"
  if curl -ks -o /dev/null -m 10 "https://127.0.0.1:${port}/"; then
    say "$(t cert_live)"
  else
    warn "$(fmt cert_notlive "$SERVICE")"
  fi
}

need_root_checks() {
  [[ $EUID -eq 0 ]] || die "$(t need_root)"
  command -v systemctl >/dev/null || die "$(t need_systemd)"
  command -v curl >/dev/null || die "$(t need_curl)"
}

detect_asset() {
  case "$(uname -m)" in
    x86_64)  ASSET=x-ui-linux-amd64;  XRAY_ASSET=Xray-linux-64.zip        ;;
    aarch64) ASSET=x-ui-linux-arm64;  XRAY_ASSET=Xray-linux-arm64-v8a.zip ;;
    *) die "$(fmt arch_bad "$(uname -m)")" ;;
  esac
}

# install_xray_core: best effort; the panel also offers it later in Settings.
install_xray_core() {
  local dest="${XUI_FOLDER}/bin/xray-linux-amd64" tmp
  [[ "$(uname -m)" == aarch64 ]] && dest="${XUI_FOLDER}/bin/xray-linux-arm64"
  tmp="$(mktemp -d)"
  say "$(t xray_dl)"
  if ! curl -fsSL --max-time 120 \
      "https://github.com/XTLS/Xray-core/releases/latest/download/${XRAY_ASSET}" -o "$tmp/xray.zip"; then
    rm -rf "$tmp"
    warn "$(t xray_dl_fail)"
    return 0
  fi
  # The geo .dat files must ship with the binary: panel routing uses geoip:/geosite:
  # rules and xray refuses to start without them.
  if command -v unzip >/dev/null; then
    unzip -p "$tmp/xray.zip" xray > "$dest" || { rm -rf "$tmp"; warn "$(t xray_ext_fail)"; return 0; }
    unzip -o -q "$tmp/xray.zip" "*.dat" -d "${XUI_FOLDER}/bin/" 2>/dev/null || true
  elif command -v python3 >/dev/null; then
    python3 - "$tmp/xray.zip" "$dest" "${XUI_FOLDER}/bin" <<'PY' || { rm -rf "$tmp"; warn "$(t xray_ext_fail)"; return 0; }
import os, sys, zipfile
z, dest, bin_dir = zipfile.ZipFile(sys.argv[1]), sys.argv[2], sys.argv[3]
open(dest, "wb").write(z.read("xray"))
for n in z.namelist():
    if n.endswith(".dat"):
        open(os.path.join(bin_dir, os.path.basename(n)), "wb").write(z.read(n))
PY
  else
    rm -rf "$tmp"
    warn "$(t xray_tool)"
    return 0
  fi
  chmod 0755 "$dest"
  rm -rf "$tmp"
  say "$(t xray_ok)"
}

download_binary() {
  if [[ "${1:-}" == "--from-source" ]]; then
    command -v go  >/dev/null || die "$(t need_go)"
    command -v npm >/dev/null || die "$(t need_npm)"
    TMP_SRC="$(mktemp -d)"; trap 'rm -rf "$TMP_SRC"' EXIT
    say "$(fmt cloning "$FORK_REPO")"
    git clone --depth 1 "https://github.com/${FORK_REPO}.git" "$TMP_SRC"
    ( cd "$TMP_SRC/frontend" && npm ci && npm run build )
    ( cd "$TMP_SRC" && CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o x-ui . )
    BINARY_SRC="$TMP_SRC/x-ui"; SCRIPT_SRC="$TMP_SRC/install-fork.sh"
    return
  fi
  if [[ "$FORK_VERSION" == "latest" ]]; then
    if TAG_JSON="$(curl -fsS --max-time 15 "https://api.github.com/repos/${FORK_REPO}/releases/latest" 2>/dev/null)"; then
      FORK_VERSION="$(printf '%s' "$TAG_JSON" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)"
    fi
    [[ "$FORK_VERSION" == v* ]] || FORK_VERSION="latest"
  fi
  local label
  if [[ "$FORK_VERSION" == "latest" ]]; then
    URL="https://github.com/${FORK_REPO}/releases/latest/download/${ASSET}"
    label="($(t latest_rel))"
  else
    URL="https://github.com/${FORK_REPO}/releases/download/${FORK_VERSION}/${ASSET}"
    label="(${FORK_VERSION})"
  fi
  say "$(t downloading) ${ASSET} ${label} ..."
  TMP_BIN="$(mktemp)"; trap 'rm -f "$TMP_BIN"' EXIT
  curl -fL "$URL" -o "$TMP_BIN" \
    || die "$(fmt dl_fail1 "$URL")
$(t dl_fail2)"
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

# Post-install card: address, credentials, x-ui command pointer.
print_info() {
  local port; port="$(current_port)"
  echo
  echo -e "┌───────────────────────────────────────────────────────┐"
  echo -e "│  ${blue}Mirage-UI$(installed_version | sed 's/^/ /')${plain}"
  echo -e "│  $(t pi_url) : ${cyan}http://$(server_ip):${port}/${plain}"
  if [[ -n "${SHOW_USER:-}" ]]; then
    echo -e "│  $(t pi_user)  : ${SHOW_USER}"
    echo -e "│  $(t pi_pass)  : ${SHOW_PASS}"
  fi
  echo -e "│  $(t pi_manage)    : ${blue}x-ui${plain} $(t pi_manage_desc)"
  echo -e "│  $(t pi_https)     : ${blue}x-ui${plain} $(t pi_https_desc)"
  echo -e "└───────────────────────────────────────────────────────┘"
  echo "$(t pi_firewall)"
}

collect_creds_after_install() {
  if [[ -n "$SEEDED_USER" ]]; then
    SHOW_USER="$SEEDED_USER"; SHOW_PASS="$SEEDED_PASS"
    return
  fi
  local creds
  # --since: a reinstall over the same unit must not print the previous
  # first-run credentials found further down the journal.
  creds="$(journalctl -u "$SERVICE" --since "-2 min" --no-pager 2>/dev/null | grep -m1 'Username:')"
  SHOW_USER="$(sed -n 's/.*Username: *\([^ ]*\).*/\1/p' <<<"$creds")"
  SHOW_PASS="$(sed -n 's/.*Password: *\([^ ]*\).*/\1/p' <<<"$creds")"
}

# Install this script itself as the management command (official pattern).
install_manager() {
  local src="${1:-}" mgr=/usr/bin/m-ui
  if [[ -z "$src" && -f "$0" && "$0" != "bash" && "$(readlink -f "$0" 2>/dev/null)" != "$mgr" ]]; then
    src="$0"
  fi
  if [[ -z "$src" ]]; then
    src="$(mktemp)"
    curl -fsSL --max-time 30 \
      "https://raw.githubusercontent.com/${FORK_REPO}/main/install-fork.sh" -o "$src" \
      || { warn "$(t mgr_fetch)"; return 0; }
  fi
  install -m 0755 "$src" "$mgr"
  sed -i "s|^XUI_FOLDER_DEFAULT=.*|XUI_FOLDER_DEFAULT=${XUI_FOLDER}|;s|^SERVICE_DEFAULT=.*|SERVICE_DEFAULT=${SERVICE}|" "$mgr"
  rm -f /usr/bin/x-ui   # take over the legacy command name
  say "$(t mgr_installed)"
}

# ---------- HTTPS ----------

apply_https() {
  local domain email acme_home="$HOME/.acme.sh" acme rc=0
  echo "$(t https_intro1)"
  echo "$(t https_intro2)"
  ask domain "$(t domain_prompt)"
  [[ "$domain" =~ ^[A-Za-z0-9.-]+$ ]] || { warn "$(t domain_bad)"; return; }
  ask email "$(t email_prompt)"
  [[ "$email" == *@* ]] || { warn "$(t email_bad)"; return; }
  if port_in_use 80; then
    warn "$(t port80_used)"
    return
  fi
  if [[ ! -x "$acme_home/acme.sh" ]]; then
    say "$(t acme_installing)"
    # Official one-liner form; --install only works from a cloned repo dir.
    if ! curl -fsSL --max-time 60 https://get.acme.sh | sh -s "email=$email" >/dev/null 2>&1 \
      && ! curl -fsSL --max-time 60 https://raw.githubusercontent.com/acmesh-official/acme.sh/master/acme.sh \
           | sh -s "email=$email" >/dev/null 2>&1; then
      warn "$(t acme_fail_install)"
      return
    fi
  fi
  acme="$acme_home/acme.sh"
  say "$(fmt acme_requesting "$domain")"
  "$acme" --home "$acme_home" --server letsencrypt --issue --standalone --keylength ec-256 \
      -d "$domain" > /tmp/acme-issue.log 2>&1 || rc=$?
  # acme.sh exits 2 when a valid cert exists and issuance is skipped.
  if (( rc != 0 && rc != 2 )); then
    warn "$(t acme_failed)"
    grep -vE "^\s*$" /tmp/acme-issue.log | tail -3 | sed 's/^/    /'
    warn "$(t acme_check)"
    return
  fi
  if (( rc == 2 )); then say "$(t acme_exists)"; fi
  mkdir -p "$XUI_FOLDER/ssl"
  "$acme" --home "$acme_home" --install-cert -d "$domain" --ecc \
    --key-file "${XUI_FOLDER}/ssl/${domain}.key" \
    --fullchain-file "${XUI_FOLDER}/ssl/${domain}.crt" \
    --reloadcmd "systemctl restart ${SERVICE}" >/dev/null 2>&1 \
    || { warn "$(fmt acme_installcert_fail "$acme_home/account.conf")"; return; }
  chmod 600 "${XUI_FOLDER}/ssl/${domain}.key" "${XUI_FOLDER}/ssl/${domain}.crt" 2>/dev/null || true
  apply_cert_paths "${XUI_FOLDER}/ssl/${domain}.crt" "${XUI_FOLDER}/ssl/${domain}.key"
  echo -e "  $(t pi_url) : ${cyan}https://${domain}:$(current_port)/${plain}"
  echo "$(t cert_renew)"
}

# Official-style SSL choice.
ssl_choice_menu() {
  local choice
  echo -e "${blue}$(t ssl_title)${plain}"
  echo "$(t ssl_opt1)"
  echo "$(t ssl_opt2)"
  echo "$(t ssl_opt3)"
  ask choice "$(t ssl_choice)" "3"
  case "$choice" in
    1) apply_https;;
    2)
      local crt key
      ask crt "$(t crt_file)"
      ask key "$(t crt_key)"
      [[ -s "$crt" && -s "$key" ]] || { warn "$(t crt_missing)"; return; }
      apply_cert_paths "$crt" "$key";;
    *) echo "$(t ssl_skipped)";;
  esac
}

# ---------- install ----------

install_fresh() {
  need_root_checks
  local script_src="${SCRIPT_SRC:-}"
  if is_tty; then
    echo -e "${cyan}$(t install_hdr)${plain}  (${FORK_REPO})"
    select_panel_port
    ask_creds
  else
    PANEL_PORT="${PANEL_PORT:-5566}"
    [[ "$PANEL_PORT" =~ ^[0-9]+$ ]] && (( PANEL_PORT >= 1 && PANEL_PORT <= 65535 )) \
      || die "$(t env_port_bad)"
    [[ -z "$SEEDED_PASS" || ${#SEEDED_PASS} -ge 8 ]] || die "$(t env_pass_bad)"
    [[ -n "$SEEDED_USER" || -z "$SEEDED_PASS" ]] || die "$(t env_pair)"
    [[ -n "$SEEDED_PASS" || -z "$SEEDED_USER" ]] || die "$(t env_pair)"
  fi
  detect_asset
  download_binary "${1:-}"
  say "$(fmt installing_into "$XUI_FOLDER")"
  mkdir -p "$XUI_FOLDER"/{bin,logs,acc}
  install -m 0755 "$BINARY_SRC" "$BINARY"
  touch "$XUI_FOLDER/x-ui-access.log"
  install_xray_core
  write_unit
  say "$(fmt starting_svc "$PANEL_PORT")"
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
  say "$(fmt finished "$(installed_version)")"
  print_info
  if is_tty; then echo; ssl_choice_menu; fi
}

# ---------- management ----------

confirm_restart() {
  local r=""
  ask r "$(t restart_q)" "y"
  # if/then, not "&& systemctl": a bare && list returning false under
  # set -e would end the menu when the user answers n.
  if confirm_yes "$r"; then systemctl restart "$SERVICE"; fi
}

change_port() {
  read_port "$(current_port)"
  write_unit
  confirm_restart
  echo -e "$(fmt port_yours "$PANEL_PORT")"
}

read_login() {
  echo "$(t login_intro)"
  LOGIN_U=""; LOGIN_P=""
  read -rp "$(t username_colon)" LOGIN_U
  IFS= read -rsp "$(t pass_hidden2)" LOGIN_P || { echo; return 1; }
  echo
  panel_login "$(current_port)" "$LOGIN_U" "$LOGIN_P"
}

change_credentials() {
  local port nu p1="" p2="" body res
  port="$(current_port)"
  if ! read_login; then
    warn "$(fmt login_fail "$port")"
    warn "$(t login_forget | sed "s|%s|${BINARY}|;s|%s|${SERVICE}|")"
    return
  fi
  nu="$LOGIN_U"
  read -rp "$(t new_user_q | sed "s|%s|$LOGIN_U|")" x; [[ -n "$x" ]] && nu="$x"
  while true; do
    IFS= read -rsp "$(t new_pass_q)" p1; echo
    [[ -z "$p1" ]] && { p1="$LOGIN_P"; break; }
    [[ ${#p1} -ge 8 ]] || { warn "$(t pass_short)"; continue; }
    IFS= read -rsp "$(t pass_again)" p2; echo
    [[ "$p1" == "$p2" ]] || { warn "$(t pass_nomatch)"; continue; }
    break
  done
  body="$(printf '{"oldUsername":"%s","oldPassword":"%s","newUsername":"%s","newPassword":"%s"}' \
    "$(json_escape "$LOGIN_U")" "$(json_escape "$LOGIN_P")" "$(json_escape "$nu")" "$(json_escape "$p1")")"
  res="$(panel_api "$port" "$LOGIN_U" "$LOGIN_P" /panel/api/setting/updateUser "$body")"
  if grep -q '"success":true' <<<"$res"; then
    say "$(fmt login_ok "$nu")"
  else
    warn "$(fmt login_upd_fail "$res")"
  fi
}

show_settings() {
  echo -e "${blue}$(t set_title)${plain}"
  echo "  Version    : $(installed_version)"
  echo "  $(t set_folder)    : ${XUI_FOLDER}"
  echo "  $(t set_svc)      : ${SERVICE} ($(systemctl is-active "$SERVICE" 2>/dev/null || echo stopped), autostart: $(systemctl is-enabled "$SERVICE" 2>/dev/null || echo ?))"
  echo "  $(t pi_url) : http://$(server_ip):$(current_port)/"
  local crt
  crt="$(DB="$XUI_FOLDER/x-ui.db" python3 -c 'import os,sqlite3; print((sqlite3.connect(os.environ["DB"]).execute("select value from settings where key=?", ("webCertFile",)).fetchone() or ("",))[0])' 2>/dev/null || true)"
  if [[ -n "$crt" && -s "$crt" ]]; then
    echo "  $(t set_cert) : ${crt} ($(openssl x509 -enddate -noout -in "$crt" 2>/dev/null | cut -d= -f2 || echo unknown))"
  fi
}

update_fork() {
  need_root_checks
  detect_asset
  local backup="${BINARY}.bak"
  FORK_VERSION="latest"
  download_binary
  say "$(fmt upd_running "$backup")"
  systemctl stop "$SERVICE"
  cp -f "$BINARY" "$backup" 2>/dev/null || true
  install -m 0755 "$BINARY_SRC" "$BINARY"
  echo "${FORK_VERSION}" > "$VERSION_FILE"
  systemctl start "$SERVICE"
  sleep 4
  if systemctl is-active --quiet "$SERVICE"; then
    say "$(fmt upd_ok "$FORK_VERSION")"
  else
    warn "$(t upd_fail)"
    mv -f "$backup" "$BINARY"
    systemctl start "$SERVICE"
  fi
}

uninstall() {
  local wipe=""
  echo -e "${red}$(t uninst_warn)${plain}"
  ask wipe "$(fmt uninst_q "$XUI_FOLDER")" ""
  systemctl stop "$SERVICE" 2>/dev/null || true
  systemctl disable "$SERVICE" 2>/dev/null || true
  rm -f "$UNIT_FILE" /usr/bin/x-ui /usr/bin/m-ui
  systemctl daemon-reload
  if [[ "$wipe" == "YES" ]]; then
    rm -rf "$XUI_FOLDER"
    say "$(fmt uninst_all "$XUI_FOLDER")"
  else
    say "$(fmt uninst_keep "$XUI_FOLDER")"
  fi
}

before_show_menu() {
  echo; read -rp "$(t enter_back)" _ || return
}

show_menu() {
  clear 2>/dev/null || true
  echo -e "╔══════════════════════════════════════════════════╗"
  echo -e "║  ${green}Mirage-UI$(installed_version | sed 's/^/ /') $(t menu_mgmt)${plain}"
  echo -e "║  $(systemctl is-active "$SERVICE" 2>/dev/null || echo stopped)  http://$(server_ip):$(current_port)/"
  echo -e "╟──────────────────────────────────────────────────╢"
  echo -e "║  ${green} 1.${plain} $(t m1)"
  echo -e "║  ${green} 2.${plain} $(t m2)"
  echo -e "║  ${green} 3.${plain} $(t m3)"
  echo -e "║  ${green} 4.${plain} $(t m4)"
  echo -e "║  ${green} 5.${plain} $(t m5)"
  echo -e "║  ${green} 6.${plain} $(t m6)"
  echo -e "╟──────────────────────────────────────────────────╢"
  echo -e "║  ${green} 7.${plain} $(t m7)          ${green} 8.${plain} $(t m8)"
  echo -e "║  ${green} 9.${plain} $(t m9)        ${green}10.${plain} $(t m10)"
  echo -e "║  ${green}11.${plain} $(t m11)     ${green}12.${plain} $(t m12)"
  echo -e "╟──────────────────────────────────────────────────╢"
  echo -e "║  ${green} 0.${plain} $(t m0)"
  echo -e "╚══════════════════════════════════════════════════╝"
}

menu() {
  local choice
  while true; do
    show_menu
    if ! IFS= read -rp "$(t menu_prompt)" choice; then return; fi
    case "$choice" in
      1) update_fork;;
      2) uninstall; return;;
      3) change_credentials;;
      4) change_port;;
      5) show_settings;;
      6) ssl_choice_menu; continue;;
      7) systemctl start "$SERVICE";;
      8) systemctl stop "$SERVICE";;
      9) confirm_restart;;
      10) systemctl status "$SERVICE" --no-pager | head -12;;
      11) echo "$(t log_hint)"; journalctl -u "$SERVICE" -n 30 --no-pager; choice="";;
      12)
        if [[ "$(systemctl is-enabled "$SERVICE" 2>/dev/null || echo ?)" == enabled ]]; then
          systemctl disable "$SERVICE" && say "$(t autostart_off)"
        else
          systemctl enable "$SERVICE" && say "$(t autostart_on)"
        fi;;
      0|"") return;;
      *) warn "$(t opt_unknown)";;
    esac
    [[ -n "$choice" ]] && before_show_menu
  done
}

show_usage() {
  if [[ "$L" == en ]]; then
    echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}m-ui control menu usages (subcommands):${plain}              │
│                                                       │
│  ${blue}m-ui${plain}              - Admin Management Script          │
│  ${blue}m-ui start${plain}        - Start                            │
│  ${blue}m-ui stop${plain}         - Stop                             │
│  ${blue}m-ui restart${plain}      - Restart                          │
│  ${blue}m-ui status${plain}       - Current Status                   │
│  ${blue}m-ui settings${plain}     - Current Settings                 │
│  ${blue}m-ui enable${plain}       - Enable Autostart on OS Startup   │
│  ${blue}m-ui disable${plain}      - Disable Autostart on OS Startup  │
│  ${blue}m-ui log${plain}          - Check logs                       │
│  ${blue}m-ui update${plain}       - Update                           │
│  ${blue}m-ui install${plain}      - Install                          │
│  ${blue}m-ui uninstall${plain}    - Uninstall                        │
└───────────────────────────────────────────────────────┘"
  else
    echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}m-ui 子命令用法：${plain}                                    │
│                                                       │
│  ${blue}m-ui${plain}              - 打开管理菜单                     │
│  ${blue}m-ui start${plain}        - 启动                             │
│  ${blue}m-ui stop${plain}         - 停止                             │
│  ${blue}m-ui restart${plain}      - 重启                             │
│  ${blue}m-ui status${plain}       - 查看状态                         │
│  ${blue}m-ui settings${plain}     - 查看当前设置                     │
│  ${blue}m-ui enable${plain}       - 开启开机自启                     │
│  ${blue}m-ui disable${plain}      - 关闭开机自启                     │
│  ${blue}m-ui log${plain}          - 查看日志                         │
│  ${blue}m-ui update${plain}       - 更新                             │
│  ${blue}m-ui install${plain}      - 安装                             │
│  ${blue}m-ui uninstall${plain}    - 卸载                             │
└───────────────────────────────────────────────────────┘"
  fi
}

# ---------- main ----------

CMD="${1:-}"
case "$CMD" in
  help|--help|-h) show_usage; exit 0;;
  start|stop|restart|status|settings|enable|disable|log|update|uninstall)
    [[ -x "$BINARY" ]] || die "$(t not_installed)"
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
  # x-ui-era migration: install m-ui (fresh copy) when missing.
  [[ -x /usr/bin/m-ui ]] || install_manager ""
  menu
else
  print_info
  [[ -x /usr/bin/m-ui ]] || install_manager ""
  warn "$(t already_installed)"
fi
