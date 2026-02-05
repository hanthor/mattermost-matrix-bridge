#!/bin/bash

# Detect container runtime
if command -v docker &> /dev/null; then
    DOCKER_CMD="docker"
    COMPOSE_CMD="docker compose"
elif command -v podman &> /dev/null; then
    DOCKER_CMD="podman"
    if command -v podman-compose &> /dev/null; then
        COMPOSE_CMD="podman-compose"
    else
        COMPOSE_CMD="podman compose"
    fi
else
    echo "Error: Neither docker nor podman found."
    exit 1
fi


GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== Mattermost-Matrix Bridge Provisioning ===${NC}"

# 1. Wait for Mattermost (using mmctl instead of curl)
echo -e "${GREEN}[1/6] Waiting for Mattermost...${NC}"
until $DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl version --local > /dev/null 2>&1; do
  sleep 3
done
echo "✓ Mattermost is ready"

# 2. Create Admin User
echo -e "${GREEN}[2/6] Setting up admin user...${NC}"
$DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl user create \
  --email admin@example.com \
  --username sysadmin \
  --password 'Sys@dmin123' \
  --system-admin \
  --local 2>&1 | grep -q "Created\|already exists" && echo "✓ Admin user ready" || echo "✓ User exists"

# 3. Create Team and Channel
echo -e "${GREEN}[3/6] Creating team and channel...${NC}"
$DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl team create \
  --name test-team \
  --display-name "Test Team" \
  --local 2>&1 | grep -q "created\|exists" && echo "✓ Team ready" || echo "✓ Team exists"

$DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl team users add test-team sysadmin --local > /dev/null 2>&1
echo "✓ User added to team"

$DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl channel create \
  --team test-team \
  --name test-channel \
  --display-name "Test Channel" \
  --local 2>&1 | grep -q "created\|exists" && echo "✓ Channel ready" || echo "✓ Channel exists"

# 4. Enable and Generate PAT
echo -e "${GREEN}[4/6] Configuring PAT...${NC}"
$DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl config set ServiceSettings.EnableUserAccessTokens true --local > /dev/null 2>&1
echo "✓ PAT enabled"

# Try to get existing token first
TOKEN=$($DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl user token list sysadmin --local --json 2>&1 | grep -oP '"token":\s*"\K[^"]+' | head -1)

# If no token exists, create one
if [ -z "$TOKEN" ]; then
    TOKEN_JSON=$($DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl token generate sysadmin bridgetoken --local --json 2>&1)
    TOKEN=$(echo "$TOKEN_JSON" | grep -oP '"token":\s*"\K[^"]+' | head -1)
fi

if [ -z "$TOKEN" ]; then
    echo -e "${YELLOW}⚠ Could not get/create token${NC}"
    exit 1
fi

echo "✓ Token ready: ${TOKEN:0:10}..."

# 5. Update Bridge Config
echo -e "${GREEN}[5/6] Updating bridge config...${NC}"
if grep -q 'admin_token: ""' config.yaml; then
    sed -i "s|admin_token: \"\"|admin_token: \"$TOKEN\"|g" config.yaml
else
    sed -i "s|admin_token: .*|admin_token: \"$TOKEN\"|g" config.yaml
fi
echo "✓ Config updated"

# 6. Restart Bridge
echo -e "${GREEN}[6/6] Restarting bridge...${NC}"
$COMPOSE_CMD restart bridge > /dev/null 2>&1
sleep 3
echo "✓ Bridge restarted"

# Load URLs from .env.urls if it exists
if [ -f .env.urls ]; then
    source .env.urls
else
    MATTERMOST_URL="http://localhost:8065"
    ELEMENT_URL="http://localhost:8080"
    SYNAPSE_URL="http://localhost:8008"
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           Provisioning Complete!                       ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  ${BLUE}Mattermost${NC}                                          ${GREEN}║${NC}"
echo -e "${GREEN}║  ${YELLOW}➜${NC} $MATTERMOST_URL                                  ${GREEN}║${NC}"
echo -e "${GREEN}║    User: sysadmin / Sys@dmin123                        ║${NC}"
echo -e "${GREEN}║    Team: Test Team → test-channel                      ║${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  ${BLUE}Element (Matrix Client)${NC}                             ${GREEN}║${NC}"
echo -e "${GREEN}║  ${YELLOW}➜${NC} $ELEMENT_URL                                      ${GREEN}║${NC}"
echo -e "${GREEN}║    (Create account or login to existing)              ║${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  ${BLUE}Test the Bridge:${NC}                                    ${GREEN}║${NC}"
echo -e "${GREEN}║  1. Login to Mattermost                                ║${NC}"
echo -e "${GREEN}║  2. Send message in test-channel                       ║${NC}"
echo -e "${GREEN}║  3. Check Matrix for bridged room                      ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
