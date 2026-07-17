#!/usr/bin/env bash
# PulseNode — one-command installer
# Usage: curl -fsSL https://raw.githubusercontent.com/SakithaSamarathunga33/PulseNode/main/install.sh | bash
set -euo pipefail

G='\033[0;32m'; C='\033[0;36m'; Y='\033[1;33m'; R='\033[0;31m'; B='\033[1m'; N='\033[0m'

REPO_URL="https://github.com/SakithaSamarathunga33/PulseNode.git"
INSTALL_DIR="${PULSENODE_DIR:-$HOME/pulsenode}"
[[ "$(id -u)" == "0" ]] && INSTALL_DIR="/opt/pulsenode"

echo -e "${C}${B}"
cat << 'BANNER'
  ____        _          _   _           _
 |  _ \ _   _| |___  ___| \ | | ___   __| | ___
 | |_) | | | | / __|/ _ \  \| |/ _ \ / _` |/ _ \
 |  __/| |_| | \__ \  __/ |\  | (_) | (_| |  __/
 |_|    \__,_|_|___/\___|_| \_|\___/ \__,_|\___|
BANNER
echo -e "${N}${G}  One-command VPS monitoring dashboard installer${N}"
echo ""

# ── Prerequisites ──────────────────────────────────────────────────────────────
check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    echo -e "${R}✗ $1 is not installed.${N}  $2"
    exit 1
  fi
}
check_cmd git    "Install: sudo apt-get install git"
check_cmd curl   "Install: sudo apt-get install curl"
check_cmd docker "Install Docker: https://docs.docker.com/get-docker/"
if ! docker compose version &>/dev/null 2>&1; then
  echo -e "${R}✗ Docker Compose v2 not available.${N}"
  echo "  Update Docker or install the Compose plugin:"
  echo "  https://docs.docker.com/compose/install/"
  exit 1
fi

echo -e "${G}✓ git            $(git --version | awk '{print $3}')${N}"
echo -e "${G}✓ Docker         $(docker --version | awk '{print $3}' | tr -d ',')${N}"
echo -e "${G}✓ Docker Compose $(docker compose version --short 2>/dev/null || echo 'v2')${N}"
echo ""

# ── Clone or update ────────────────────────────────────────────────────────────
if [[ -d "$INSTALL_DIR/.git" ]]; then
  echo -e "${C}━━━  Updating existing install at ${INSTALL_DIR}  ━━━━━━━━━━━━━━━${N}"
  git -C "$INSTALL_DIR" pull --ff-only
else
  echo -e "${C}━━━  Cloning PulseNode into ${INSTALL_DIR}  ━━━━━━━━━━━━━━━━━━━━${N}"
  git clone "$REPO_URL" "$INSTALL_DIR"
fi
cd "$INSTALL_DIR"
echo ""

# ── Detect public IP ───────────────────────────────────────────────────────────
DETECTED_IP=$(
  curl -fsSL --max-time 5 https://api.ipify.org 2>/dev/null ||
  curl -fsSL --max-time 5 https://ifconfig.me   2>/dev/null ||
  hostname -I 2>/dev/null | awk '{print $1}'    ||
  echo "localhost"
)

echo -e "${C}━━━  Access URL  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}"
printf "  Detected IP: ${B}%s${N}\n" "$DETECTED_IP"
printf "  Use %s as your host? [Y/n]: " "$DETECTED_IP"
read -r CONFIRM </dev/tty || CONFIRM="y"
if [[ "${CONFIRM,,}" == "n" ]]; then
  printf "  Enter your VPS IP or domain: "
  read -r DETECTED_IP </dev/tty
fi
HOST="${DETECTED_IP#https://}"; HOST="${HOST#http://}"; HOST="${HOST%%/*}"

# ── Port selection ─────────────────────────────────────────────────────────────
port_in_use() { ss -tlnp 2>/dev/null | grep -q ":$1 " || netstat -tlnp 2>/dev/null | grep -q ":$1 "; }

LISTEN=80
if port_in_use 80; then
  echo -e "  ${Y}⚠ Port 80 is already in use on this machine.${N}"
  while true; do
    printf "  Enter a free port for PulseNode to listen on [default: 8080]: "
    read -r ALT_PORT </dev/tty || ALT_PORT=""
    LISTEN="${ALT_PORT:-8080}"
    if port_in_use "$LISTEN"; then
      echo -e "  ${R}✗ Port ${LISTEN} is also in use. Try another.${N}"
    else
      break
    fi
  done
fi

if [[ "$LISTEN" == "80" ]]; then
  BASE_URL="http://${HOST}"
  if port_in_use 443; then
    # 443 occupied — disable auto-HTTPS so Caddy doesn't try to bind it
    CADDY_SITE_ADDRESS="http://:80"
    OVERLAY="docker-compose.nossl.yml"
    echo -e "  ${Y}⚠ Port 443 is in use — running HTTP-only on port 80${N}"
  else
    CADDY_SITE_ADDRESS=":80"
    OVERLAY="docker-compose.standalone.yml"
  fi
else
  BASE_URL="http://${HOST}:${LISTEN}"
  # http:// prefix disables Caddy's automatic HTTPS (which would try to bind 443)
  CADDY_SITE_ADDRESS="http://:${LISTEN}"
  OVERLAY="docker-compose.nossl.yml"
fi
echo ""

# ── Admin login ────────────────────────────────────────────────────────────────
# PulseNode can manage Docker, processes, and deployments — the dashboard should
# never sit on a public IP without a login. Create the admin account up front.
json_escape() { local s=${1//\\/\\\\}; s=${s//\"/\\\"}; printf '%s' "$s"; }

AUTH_ENABLED=""
if [[ -f .env.local ]]; then
  OLD_PORT=$(grep '^LISTEN_PORT=' .env.local | cut -d= -f2- || true)
  if [[ -n "${OLD_PORT}" ]] && curl -fsSL --max-time 3 "http://localhost:${OLD_PORT}/go/api/auth/status" 2>/dev/null | grep -q '"enabled":true'; then
    AUTH_ENABLED=1
  fi
fi

ADMIN_USER=""
ADMIN_PASS=""
echo -e "${C}━━━  Login protection  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}"
if [[ -n "$AUTH_ENABLED" ]]; then
  echo -e "  ${G}✓ Login protection is already enabled — keeping your existing account${N}"
else
  echo -e "  Create an admin account to protect the dashboard."
  printf "  Admin username [admin]: "
  read -r ADMIN_USER </dev/tty || ADMIN_USER=""
  ADMIN_USER="${ADMIN_USER:-admin}"
  while true; do
    printf "  Admin password (min 8 chars — leave blank to skip): "
    read -rs ADMIN_PASS </dev/tty || ADMIN_PASS=""
    echo ""
    if [[ -z "$ADMIN_PASS" ]]; then
      ADMIN_USER=""
      echo -e "  ${R}⚠ No login set — the dashboard will be OPEN to anyone who can reach this server.${N}"
      echo -e "  ${Y}  You can enable it later: dashboard → Settings → Security${N}"
      break
    fi
    if (( ${#ADMIN_PASS} < 8 )); then
      echo -e "  ${R}✗ Password must be at least 8 characters.${N}"
      continue
    fi
    printf "  Confirm password: "
    read -rs ADMIN_PASS2 </dev/tty || ADMIN_PASS2=""
    echo ""
    if [[ "$ADMIN_PASS" != "$ADMIN_PASS2" ]]; then
      echo -e "  ${R}✗ Passwords don't match — try again.${N}"
      continue
    fi
    break
  done
fi
echo ""

# ── Optional integrations ──────────────────────────────────────────────────────
echo -e "${C}━━━  Optional integrations (press Enter to skip)  ━━━━━━━━━━━━${N}"

printf "  Coolify API URL   (e.g. https://coolify.example.com): "
read -r COOLIFY_URL </dev/tty || COOLIFY_URL=""

printf "  Coolify API token: "
read -r COOLIFY_TOKEN </dev/tty || COOLIFY_TOKEN=""
echo ""

# ── Write .env.local ───────────────────────────────────────────────────────────
API_SECRET=$(openssl rand -hex 32 2>/dev/null \
  || head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)

CURRENT_VERSION=$(git -C "$INSTALL_DIR" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")

echo -e "${C}━━━  Writing configuration  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}"

# Preserve existing secrets if .env.local already exists
EXISTING_JWT=""
EXISTING_AES=""
EXISTING_MASTER=""
if [[ -f .env.local ]]; then
  EXISTING_JWT=$(grep '^JWT_SECRET=' .env.local | cut -d= -f2- || echo "")
  EXISTING_AES=$(grep '^AES_KEY=' .env.local | cut -d= -f2- || echo "")
  EXISTING_MASTER=$(grep '^MASTER_ENCRYPTION_KEY=' .env.local | cut -d= -f2- || echo "")
fi
JWT_SECRET="${EXISTING_JWT:-$API_SECRET}"
AES_KEY="${EXISTING_AES:-$API_SECRET}"
MASTER_KEY="${EXISTING_MASTER:-$API_SECRET}"

cat > .env.local << EOF
# PulseNode — generated by install.sh on $(date -u '+%Y-%m-%d %H:%M UTC')
NEXT_PUBLIC_ORIGIN=${BASE_URL}
NEXT_PUBLIC_GO_API=${BASE_URL}/go
PULSENODE_VPS_IP=${DETECTED_IP}
CADDY_SITE_ADDRESS=${CADDY_SITE_ADDRESS}

WEB_PORT=127.0.0.1:3000
GO_PORT=127.0.0.1:4002
LISTEN_PORT=${LISTEN}

GO_API_AUTH=false
JWT_SECRET=${JWT_SECRET}
AES_KEY=${AES_KEY}
MASTER_ENCRYPTION_KEY=${MASTER_KEY}

COOLIFY_API_URL=${COOLIFY_URL}
COOLIFY_API_TOKEN=${COOLIFY_TOKEN}

# Update tracking
PULSENODE_VERSION=${CURRENT_VERSION}
PULSENODE_INSTALL_DIR=${INSTALL_DIR}
PULSENODE_OVERLAY=${OVERLAY}
PULSENODE_COMPOSE_BIN=docker compose
EOF
echo -e "  ${G}✓ .env.local written${N}"
echo ""

# ── Pull pre-built images or build from source ─────────────────────────────────
echo -e "${C}━━━  Starting containers  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}"

COMPOSE_CMD="docker compose --env-file .env.local -f docker-compose.yml -f $OVERLAY"

if $COMPOSE_CMD -f docker-compose.ghcr.yml pull --quiet 2>/dev/null; then
  echo -e "  ${G}✓ Using pre-built images from GitHub Container Registry${N}"
  echo ""
  $COMPOSE_CMD -f docker-compose.ghcr.yml up -d
else
  echo -e "  ${Y}Building from source — this takes a few minutes on first run ☕${N}"
  echo ""
  $COMPOSE_CMD up -d --build
fi

# ── Wait for services ──────────────────────────────────────────────────────────
echo ""
echo -e "${C}━━━  Waiting for services to be ready  ━━━━━━━━━━━━━━━━━━━━━${N}"
WAIT=0
until curl -fsSL --max-time 2 "http://localhost:${LISTEN}/health" &>/dev/null; do
  if (( WAIT >= 90 )); then
    echo -e "\n  ${Y}⚠ Taking longer than expected. Check logs: docker compose logs${N}"
    break
  fi
  printf "\r  Waiting... %ds" "$WAIT"
  sleep 3
  WAIT=$((WAIT + 3))
done
printf "\r  ${G}✓ Services ready${N}          \n"
echo ""

# ── Enable login protection ────────────────────────────────────────────────────
if [[ -n "$ADMIN_USER" && -n "$ADMIN_PASS" ]]; then
  PAYLOAD=$(printf '{"username":"%s","password":"%s"}' "$(json_escape "$ADMIN_USER")" "$(json_escape "$ADMIN_PASS")")
  if curl -fsSL --max-time 5 -X POST "http://localhost:${LISTEN}/go/api/auth/setup" \
       -H 'Content-Type: application/json' -d "$PAYLOAD" &>/dev/null; then
    echo -e "  ${G}✓ Login protection enabled — sign in as '${ADMIN_USER}'${N}"
  else
    echo -e "  ${Y}⚠ Could not create the admin account automatically.${N}"
    echo -e "    Set it up in the dashboard: Settings → Security"
  fi
  echo ""
fi

# ── Done ───────────────────────────────────────────────────────────────────────
echo -e "${G}${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}"
echo -e "${G}${B}  ✓  PulseNode is live!${N}"
echo ""
echo -e "  ${B}Open in browser  →  ${C}${BASE_URL}/${N}"
echo ""
echo -e "  ${B}Quick links:${N}"
echo -e "    ${C}${BASE_URL}/containers${N}"
echo -e "    ${C}${BASE_URL}/stats${N}"
echo -e "    ${C}${BASE_URL}/processes${N}"
echo -e "    ${C}${BASE_URL}/databases${N}"
echo ""
echo -e "  Installed at:  ${Y}${INSTALL_DIR}${N}"
echo -e "  To stop:       ${Y}${COMPOSE_CMD} down${N}"
echo -e "  To update:     ${Y}curl -fsSL https://raw.githubusercontent.com/SakithaSamarathunga33/PulseNode/main/install.sh | bash${N}"
echo -e "${G}${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${N}"
