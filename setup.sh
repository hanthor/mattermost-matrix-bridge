#!/bin/bash
set -e

# Detect container runtime
if command -v docker &> /dev/null; then
    DOCKER_CMD="docker"
    COMPOSE_CMD="docker compose"
elif command -v podman &> /dev/null; then
    DOCKER_CMD="podman"
    # Check for podman-compose or use podman compose
    if command -v podman-compose &> /dev/null; then
        COMPOSE_CMD="podman-compose"
    else
        COMPOSE_CMD="podman compose"
    fi
else
    echo "Error: Neither docker nor podman found."
    exit 1
fi

echo "Using container runtime: $DOCKER_CMD"
echo "Using compose command: $COMPOSE_CMD"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== Mattermost-Matrix Bridge E2E Setup ===${NC}"
echo ""

# Function to check if port is in use
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 ; then
        return 1  # Port in use
    else
        return 0  # Port available
    fi
}

# Function to find next available port
find_available_port() {
    local start_port=$1
    local port=$start_port
    while ! check_port $port; do
        port=$((port + 1))
        if [ $port -gt $((start_port + 100)) ]; then
            echo "Error: Could not find available port near $start_port" >&2
            exit 1
        fi
    done
    echo $port
}

# Check and assign ports
echo -e "${BLUE}Checking port availability...${NC}"

MATTERMOST_PORT=$(find_available_port 40065)
SYNAPSE_PORT=$(find_available_port 40008)
ELEMENT_PORT=$(find_available_port 40080)

if [ "$MATTERMOST_PORT" != "40065" ]; then
    echo -e "${YELLOW}⚠ Port 40065 in use, using $MATTERMOST_PORT for Mattermost${NC}"
fi
if [ "$SYNAPSE_PORT" != "40008" ]; then
    echo -e "${YELLOW}⚠ Port 40008 in use, using $SYNAPSE_PORT for Synapse${NC}"
fi
if [ "$ELEMENT_PORT" != "40080" ]; then
    echo -e "${YELLOW}⚠ Port 40080 in use, using $ELEMENT_PORT for Element${NC}"
fi

echo "✓ Using ports: Mattermost=$MATTERMOST_PORT, Synapse=$SYNAPSE_PORT, Element=$ELEMENT_PORT"

# Update docker-compose.yaml with assigned ports (fix all occurrences)
echo -e "${BLUE}Updating docker-compose.yaml with port assignments...${NC}"

# Create backup
cp docker-compose.yaml docker-compose.yaml.bak 2>/dev/null || true

# Update Synapse image
sed -i "s|image: docker.io/matrixdotorg/synapse:latest|image: docker.io/matrixdotorg/synapse:v1.115.0|g" docker-compose.yaml
sed -i "s|image: ghcr.io/element-hq/synapse:.*|image: docker.io/matrixdotorg/synapse:v1.115.0|g" docker-compose.yaml

# Update Synapse port
sed -i "s/- \"[0-9]*:8008\"/- \"$SYNAPSE_PORT:8008\"/g" docker-compose.yaml
sed -i "s/- [0-9]*:8008$/- $SYNAPSE_PORT:8008/g" docker-compose.yaml

# Update Mattermost port  
sed -i "s/- \"[0-9]*:8065\"/- \"$MATTERMOST_PORT:8065\"/g" docker-compose.yaml
sed -i "s/- [0-9]*:8065$/- $MATTERMOST_PORT:8065/g" docker-compose.yaml

# Update Element port
sed -i "s/- \"[0-9]*:80\"/- \"$ELEMENT_PORT:80\"/g" docker-compose.yaml
sed -i "s/- [0-9]*:80$/- $ELEMENT_PORT:80/g" docker-compose.yaml


# Generate Synapse config if needed
if [ ! -f "synapse-data/homeserver.yaml" ]; then
    echo -e "${BLUE}Generating Synapse configuration...${NC}"
    mkdir -p synapse-data
    $DOCKER_CMD run --rm -v $(pwd)/synapse-data:/data:Z \
        -e SYNAPSE_SERVER_NAME=localhost \
        -e SYNAPSE_REPORT_STATS=no \
        docker.io/matrixdotorg/synapse:v1.115.0 generate 2>/dev/null || true

    # Fix permissions so host can edit
    $DOCKER_CMD run --rm -v $(pwd)/synapse-data:/data:Z docker.io/library/alpine chmod -R 777 /data

    # Patch homeserver.yaml for Postgres
    sed -i '/database:/,/database: \/data\/homeserver.db/c\database:\n  name: psycopg2\n  allow_unsafe_locale: true\n  args:\n    user: synapse\n    password: synapsepassword\n    database: synapse\n    host: postgres\n    cp_min: 5\n    cp_max: 10' synapse-data/homeserver.yaml
    
    # Ensure server_name and report_stats are set (check file exists first)
    if [ -f "synapse-data/homeserver.yaml" ]; then
        if ! grep -q "^server_name:" synapse-data/homeserver.yaml; then
            sed -i '1i server_name: "localhost"' synapse-data/homeserver.yaml
        fi
        if ! grep -q "^report_stats:" synapse-data/homeserver.yaml; then
            sed -i '2i report_stats: false' synapse-data/homeserver.yaml
        fi
        
        # Add our config
        echo "
enable_registration: true
enable_registration_without_verification: true
app_service_config_files:
  - /data/registration.yaml" >> synapse-data/homeserver.yaml
        echo "✓ Synapse config generated"
    fi
fi

# Generate registration.yaml if it doesn't exist
if [ ! -f "registration.yaml" ]; then
    echo -e "${BLUE}Generating registration.yaml...${NC}"
    cat > registration.yaml << 'EOF'
id: mattermost
url: http://bridge:8080
as_token: oaMzNZbT718MLatfu91l9HNANxkdwXrEkWxRolWQA
hs_token: mfg9u7SnZ0UXqhjs0JkeYBIeULG8lTM7u4Z9byXayh8
sender_localpart: mattermostbot
namespaces:
  users:
    - exclusive: true
      regex: '@mattermost_.*'
  aliases:
    - exclusive: true
      regex: '#mattermost_.*'
  rooms: []
rate_limited: false
EOF
    echo "✓ Registration file created"
fi

# Generate minimal config.yaml if it doesn't exist
if [ ! -f "config.yaml" ]; then
    echo -e "${BLUE}Generating config.yaml...${NC}"
cat > config.yaml << 'EOF'
# Config options that affect the central bridge module.
bridge:
    # The prefix for commands. Only required in non-management rooms.
    command_prefix: '!mattermost'
    personal_filtering_spaces: true
    relay:
        enabled: true
        admin_only: true
        default_relays: []
        message_formats:
            m.text: "<b>{{ .Sender.DisambiguatedName }}</b>: {{ .Message }}"
            m.notice: "<b>{{ .Sender.DisambiguatedName }}</b>: {{ .Message }}"
            m.emote: "* <b>{{ .Sender.DisambiguatedName }}</b> {{ .Message }}"
            m.file: "<b>{{ .Sender.DisambiguatedName }}</b> sent a file{{ if .Caption }}: {{ .Caption }}{{ end }}"
            m.image: "<b>{{ .Sender.DisambiguatedName }}</b> sent an image{{ if .Caption }}: {{ .Caption }}{{ end }}"
            m.audio: "<b>{{ .Sender.DisambiguatedName }}</b> sent an audio file{{ if .Caption }}: {{ .Caption }}{{ end }}"
            m.video: "<b>{{ .Sender.DisambiguatedName }}</b> sent a video{{ if .Caption }}: {{ .Caption }}{{ end }}"
            m.location: "<b>{{ .Sender.DisambiguatedName }}</b> sent a location{{ if .Caption }}: {{ .Caption }}{{ end }}"
        displayname_format: "{{ .DisambiguatedName }}"
    permissions:
        "*": relay
        "localhost": user
        "@admin:localhost": admin

# Database config
database:
    type: postgres
    uri: postgres://mmuser:mmpassword@postgres-mm:5432/mautrix_mattermost?sslmode=disable
    max_open_conns: 5
    max_idle_conns: 1
    max_conn_idle_time: null
    max_conn_lifetime: null

# Homeserver details
homeserver:
    address: http://synapse:8008
    domain: localhost
    software: standard
    status_endpoint: null
    message_send_checkpoint_endpoint: null
    async_media: false
    websocket: false
    ping_interval_seconds: 0

# Application service host/registration related details
appservice:
    address: http://bridge:8080
    hostname: 0.0.0.0
    port: 8080
    id: mattermost
    bot:
        username: mattermostbot
        displayname: Mattermost Bridge Bot
        avatar: mxc://maunium.net/mattermost
    ephemeral_events: true
    async_transactions: false
    as_token: "oaMzNZbT718MLatfu91l9HNANxkdwXrEkWxRolWQA"
    hs_token: "mfg9u7SnZ0UXqhjs0JkeYBIeULG8lTM7u4Z9byXayh8"
    username_template: "mattermost_{{.}}"

# Matrix config
matrix:
    message_status_events: false
    delivery_receipts: false
    message_error_notices: true
    sync_direct_chat_list: false
    federate_rooms: true

# Encryption config
encryption:
    allow: false
    default: false
    require: false
    appservice: false
    allow_key_sharing: true
    pickle_key: "generate"
    delete_keys:
        delete_outbound_on_ack: false
        dont_store_outbound: false
        ratchet_on_decrypt: false
        delete_fully_used_on_decrypt: false
        delete_prev_on_new_session: false
        delete_on_device_delete: false
        periodically_delete_expired: false
        delete_outdated_inbound: false
    verification_levels:
        receive: unverified
        send: unverified
        share: cross-signed-tofu
    rotation:
        enable_custom: false
        milliseconds: 604800000
        messages: 100
        disable_device_change_key_rotation: false

# Connector config
network:
    server_url: "http://mattermost:8065"
    admin_token: ""

# Logging config
logging:
    min_level: debug
    writers:
    - type: stdout
      format: pretty-colored
EOF
    echo "✓ Config file created"
fi

# Start services
echo -e "${BLUE}Starting Docker services...${NC}"
$COMPOSE_CMD up -d

echo ""
echo -e "\n=== Services Starting ==="
echo "Waiting for services to be healthy..."
# Initial wait for DBs to start
sleep 15
until $DOCKER_CMD exec mautrix-mattermost_mattermost_1 mmctl system ping --local > /dev/null 2>&1; do
    echo -n "m"
    sleep 2
done
echo -e "\n✓ Mattermost is ready"

# Synapse usually has wget
until $DOCKER_CMD exec mautrix-mattermost_synapse_1 curl -s http://localhost:8008/_matrix/client/versions > /dev/null 2>&1; do
    echo -n "s"
    sleep 2
done
echo -e "\n✓ Synapse is ready"

# Check service status
echo ""
$COMPOSE_CMD ps

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║        E2E Environment Ready - Access URLs             ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  ${BLUE}Mattermost${NC}                                          ${GREEN}║${NC}"
printf "${GREEN}║  ${YELLOW}➜${NC} http://localhost:%-5s                           ${GREEN}║${NC}\n" "$MATTERMOST_PORT"
echo -e "${GREEN}║    Login: sysadmin / Sys@dmin123                       ║${NC}"
echo -e "${GREEN}║    (Run ./provision_mm.sh to create user)              ║${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  ${BLUE}Element (Matrix Client)${NC}                             ${GREEN}║${NC}"
printf "${GREEN}║  ${YELLOW}➜${NC} http://localhost:%-5s                           ${GREEN}║${NC}\n" "$ELEMENT_PORT"
echo -e "${GREEN}║    (Create account after running provision script)    ║${NC}"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}║  ${BLUE}Synapse (Matrix Homeserver)${NC}                         ${GREEN}║${NC}"
printf "${GREEN}║  ${YELLOW}➜${NC} http://localhost:%-5s                           ${GREEN}║${NC}\n" "$SYNAPSE_PORT"
echo -e "${GREEN}║                                                        ║${NC}"
echo -e "${GREEN}╠════════════════════════════════════════════════════════╣${NC}"
echo -e "${GREEN}║  ${BLUE}Next Steps:${NC}                                         ${GREEN}║${NC}"
echo -e "${GREEN}║  1. Run: ${YELLOW}./provision_mm.sh${NC}                            ${GREEN}║${NC}"
echo -e "${GREEN}║  2. Login to Mattermost and send test message         ║${NC}"
echo -e "${GREEN}║  3. Check Matrix for bridged message                  ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Save URLs to a file for easy reference
cat > .env.urls << EOF
MATTERMOST_URL=http://localhost:$MATTERMOST_PORT
ELEMENT_URL=http://localhost:$ELEMENT_PORT
SYNAPSE_URL=http://localhost:$SYNAPSE_PORT
EOF

echo -e "${BLUE}📝 URLs saved to .env.urls${NC}"
echo ""
