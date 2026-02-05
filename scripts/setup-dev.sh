#!/bin/bash
# Local Development Setup Script for Mautrix-Mattermost Bridge
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$ROOT_DIR"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

print_step() { echo -e "${BLUE}==>${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}!${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }

check_prerequisites() {
    print_step "Checking prerequisites..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed."
        exit 1
    fi
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed."
        exit 1
    fi
    print_success "Prerequisites OK"
}

build_bridge() {
    print_step "Building bridge binary..."
    go build -tags nocrypto -o mautrix-mattermost .
    print_success "Bridge built"
}

start_services() {
    print_step "Starting Docker services (Mattermost & Synapse)..."
    docker compose down 2>/dev/null || true
    docker compose up -d
    
    print_step "Waiting for Mattermost..."
    until curl -s http://localhost:8066/api/v4/system/ping > /dev/null; do
        sleep 2
        echo -n "."
    done
    echo " Ready!"
    
    print_step "Waiting for Synapse..."
    until curl -s http://localhost:8899/_matrix/client/versions > /dev/null; do
        sleep 2
        echo -n "."
    done
    echo " Ready!"
}

setup_mattermost() {
    print_step "Setting up Mattermost..."
    CONTAINER=$(docker compose ps -q mattermost)
    
    # Wait for socket to be ready
    print_step "Waiting for Local Mode Socket..."
    for i in {1..30}; do
        if docker exec -u mattermost -e MM_SERVICESETTINGS_LOCALMODESOCKETLOCATION=/var/tmp/mattermost_local.socket "$CONTAINER" /mattermost/bin/mmctl --local system version >/dev/null 2>&1; then
            echo " Socket ready!"
            break
        fi
        echo -n "."
        sleep 2
    done
    
    # Create admin user
    docker exec -u mattermost -e MM_SERVICESETTINGS_LOCALMODESOCKETLOCATION=/var/tmp/mattermost_local.socket "$CONTAINER" /mattermost/bin/mmctl --local user create \
        --email admin@example.com --username admin --password "Admin123!" --system-admin \
        2>/dev/null || true
        
    # Create team
    docker exec -u mattermost -e MM_SERVICESETTINGS_LOCALMODESOCKETLOCATION=/var/tmp/mattermost_local.socket "$CONTAINER" /mattermost/bin/mmctl --local team create \
        --name test-team --display-name "Test Team" \
        2>/dev/null || true
        
    # Generate Admin Token
    print_step "Generating Mattermost Admin Token..."
    TOKEN_OUTPUT=$(docker exec -u mattermost -e MM_SERVICESETTINGS_LOCALMODESOCKETLOCATION=/var/tmp/mattermost_local.socket "$CONTAINER" /mattermost/bin/mmctl --local token generate admin bridge-setup --json)
    ADMIN_TOKEN=$(echo "$TOKEN_OUTPUT" | jq -r '(. | arrays | .[0].token) // .token // empty')
    
    if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
        print_error "Failed to generate Admin Token"
        exit 1
    fi
    print_success "Admin Token Generated: $ADMIN_TOKEN"
    export ADMIN_TOKEN
}

configure_bridge() {
    print_step "Configuring Bridge..."
    
    # Create config.yaml from example
    # Always overwrite config.yaml with example
    cp example-config.yaml config.yaml
    
    # Update config.yaml with environment specific values
    # We use yq if available, or sed
    
    # Update Homeserver
    sed -i "s|address: https://matrix.example.com|address: http://localhost:8899|" config.yaml
    sed -i "s|domain: example.com|domain: localhost|" config.yaml
    
    # Update Appservice (Listen address)
    sed -i "s|address: http://localhost:29324|address: http://localhost:8080|" config.yaml
    sed -i "s|hostname: 127.0.0.1|hostname: 0.0.0.0|" config.yaml
    
    # Update Mattermost
    sed -i "s|server_url: https://mattermost.example.com|server_url: http://localhost:8067|" config.yaml
    sed -i "s|admin_token: \"\"|admin_token: \"$ADMIN_TOKEN\"|" config.yaml
    
    # Generate Registration
    # Generate Registration
    print_step "Generating Registration..."
    echo "DEBUG: config.yaml content head:"
    head -n 5 config.yaml
    if ! ./mautrix-mattermost -g -c config.yaml -r registration.yaml; then
        echo "ERROR: mautrix-mattermost failed. Dumping config.yaml:"
        cat config.yaml
        exit 1
    fi
    
    # Fix URL in registration for Docker access
    sed -i "s|url: http://localhost:8080|url: http://host.docker.internal:8080|" registration.yaml
    
    # Update Docker volume with new registration
    cp registration.yaml docker/mattermost-bridge-registration.yaml
    
    # Restart Synapse to pick up registration
    print_step "Restarting Synapse to load registration..."
    docker compose restart synapse
    sleep 5
}

setup_synapse() {
    print_step "Creating Matrix Admin User..."
    CONTAINER=$(docker compose ps -q synapse)
    
    docker exec "$CONTAINER" register_new_matrix_user \
        -c /data/homeserver.yaml -u admin -p admin123 -a http://localhost:8008 \
        2>/dev/null || true
}

main() {
    check_prerequisites
    build_bridge
    start_services
    setup_mattermost
    configure_bridge
    setup_synapse
    
    echo ""
    print_success "Setup Complete!"
    echo "=========================================="
    echo "1. Run the bridge: ./mautrix-mattermost"
    echo "2. Mattermost: http://localhost:8067 (admin/Admin123!)"
    echo "3. Matrix: http://localhost:8899 (admin/admin123)"
    echo "   Element: http://localhost:8081"
    echo "=========================================="
}


main
