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
RED='\033[0;31m'
NC='\033[0m'

echo -e "${RED}=== Mattermost-Matrix Bridge Cleanup ===${NC}"
echo ""

# Parse arguments
FULL_WIPE=false
if [ "$1" == "--full" ] || [ "$1" == "-f" ]; then
    FULL_WIPE=true
    echo -e "${YELLOW}⚠️  FULL WIPE MODE - Will delete all data and volumes${NC}"
    echo ""
fi

# 1. Stop and remove containers from docker-compose
echo -e "${GREEN}[1/5] Stopping docker-compose services...${NC}"
if $COMPOSE_CMD ps -q > /dev/null 2>&1; then
    $COMPOSE_CMD down
    echo "✓ Docker compose services stopped"
else
    echo "✓ No compose services running"
fi

# 2. Kill any orphaned mattermost or matrix containers
echo -e "${GREEN}[2/5] Searching for orphaned containers...${NC}"
ORPHANS=$($DOCKER_CMD ps -a --filter "name=mattermost" --filter "name=synapse" --filter "name=element" --filter "name=matrix" --format "{{.Names}}" | grep -v "^mautrix-go-bridge-" || true)

if [ -n "$ORPHANS" ]; then
    echo -e "${YELLOW}Found orphaned containers:${NC}"
    echo "$ORPHANS"
    echo "$ORPHANS" | xargs -r $DOCKER_CMD rm -f
    echo "✓ Orphaned containers removed"
else
    echo "✓ No orphaned containers found"
fi

# 3. Remove config files for fresh generation
echo -e "${GREEN}[3/5] Removing generated config files...${NC}"
rm -rf config.yaml registration.yaml .env.urls
echo "✓ Config files removed"

# 4. Optionally remove volumes and synapse data
if [ "$FULL_WIPE" = true ]; then
    echo -e "${GREEN}[4/5] Removing volumes and database data...${NC}"
    $COMPOSE_CMD down -v 2>/dev/null || true
    rm -rf synapse-data
    echo "✓ All data wiped"
else
    echo -e "${GREEN}[4/5] Keeping volumes (use --full to wipe)${NC}"
    echo "✓ Volumes preserved"
fi

# 5. Kill any stuck provision processes
echo -e "${GREEN}[5/5] Killing stuck provision processes...${NC}"
pkill -f "./provision_mm.sh" 2>/dev/null || true
pkill -f "./setup.sh" 2>/dev/null || true
echo "✓ Processes cleaned up"

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║              Cleanup Complete!                         ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
if [ "$FULL_WIPE" = true ]; then
    echo -e "${GREEN}║  Full wipe completed - fresh slate ready              ║${NC}"
else
    echo -e "${GREEN}║  Soft cleanup - database data preserved               ║${NC}"
    echo -e "${GREEN}║  Run with --full flag to wipe all data                ║${NC}"
fi
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  Next steps:                                           ║${NC}"
echo -e "${GREEN}║  1. Run: ${YELLOW}./setup.sh${NC}                                  ${GREEN}║${NC}"
echo -e "${GREEN}║  2. Run: ${YELLOW}./provision_mm.sh${NC}                           ${GREEN}║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
