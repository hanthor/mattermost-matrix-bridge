#!/bin/bash
# Local Development Setup Script for Mattermost-Matrix Bridge
# This script sets up the complete local development environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! command -v docker compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        print_warning "Go is not installed. You won't be able to build the plugin."
    fi
    
    if ! command -v npm &> /dev/null; then
        print_warning "npm is not installed. You won't be able to build the webapp."
    fi
    
    print_success "Prerequisites check complete"
}

# Build the plugin
build_plugin() {
    print_step "Building the Mattermost Matrix Bridge plugin..."
    
    if command -v go &> /dev/null && command -v npm &> /dev/null; then
        make dist
        print_success "Plugin built successfully: dist/com.mattermost.plugin-matrix-bridge-*.tar.gz"
    else
        print_warning "Skipping plugin build - Go or npm not available"
        print_warning "Make sure you have a pre-built plugin in the dist/ directory"
    fi
}

# Generate tokens
generate_tokens() {
    print_step "Generating secure tokens..."
    
    AS_TOKEN=$(openssl rand -hex 32)
    HS_TOKEN=$(openssl rand -hex 32)
    
    echo "$AS_TOKEN" > .as_token
    echo "$HS_TOKEN" > .hs_token
    
    print_success "Tokens generated and saved to .as_token and .hs_token"
    echo ""
    echo "Application Service Token: $AS_TOKEN"
    echo "Homeserver Token: $HS_TOKEN"
    echo ""
}

# Update registration file with tokens
update_registration() {
    print_step "Updating bridge registration file with tokens..."
    
    AS_TOKEN=$(cat .as_token)
    HS_TOKEN=$(cat .hs_token)
    
    # Use sed to update the registration file
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/as_token: CHANGE_ME_AS_TOKEN/as_token: $AS_TOKEN/" docker/mattermost-bridge-registration.yaml
        sed -i '' "s/hs_token: CHANGE_ME_HS_TOKEN/hs_token: $HS_TOKEN/" docker/mattermost-bridge-registration.yaml
    else
        # Linux
        sed -i "s/as_token: CHANGE_ME_AS_TOKEN/as_token: $AS_TOKEN/" docker/mattermost-bridge-registration.yaml
        sed -i "s/hs_token: CHANGE_ME_HS_TOKEN/hs_token: $HS_TOKEN/" docker/mattermost-bridge-registration.yaml
    fi
    
    print_success "Registration file updated"
}

# Start Docker services
start_services() {
    print_step "Starting Docker services..."
    
    docker compose down -v 2>/dev/null || true
    docker compose up -d
    
    print_success "Docker services started"
    print_step "Waiting for services to be healthy..."
    
    # Wait for Mattermost
    echo -n "Waiting for Mattermost..."
    for i in {1..60}; do
        if curl -s http://localhost:8065/api/v4/system/ping > /dev/null 2>&1; then
            echo " Ready!"
            break
        fi
        echo -n "."
        sleep 2
    done
    
    # Wait for Synapse
    echo -n "Waiting for Synapse..."
    for i in {1..60}; do
        if curl -s http://localhost:8888/_matrix/client/versions > /dev/null 2>&1; then
            echo " Ready!"
            break
        fi
        echo -n "."
        sleep 2
    done
    
    print_success "All services are running"
}

# Setup Mattermost
setup_mattermost() {
    print_step "Setting up Mattermost..."
    
    CONTAINER=$(docker compose ps -q mattermost)
    
    # Wait a bit more for Mattermost to be fully ready
    sleep 5
    
    # Create admin user
    print_step "Creating admin user..."
    docker exec -u mattermost "$CONTAINER" /mattermost/bin/mmctl --local user create \
        --email admin@example.com \
        --username admin \
        --password "Admin123!" \
        --system-admin 2>/dev/null || print_warning "Admin user may already exist"
    
    # Create a team
    print_step "Creating test team..."
    docker exec -u mattermost "$CONTAINER" /mattermost/bin/mmctl --local team create \
        --name test-team \
        --display-name "Test Team" 2>/dev/null || print_warning "Team may already exist"
    
    # Add admin to team
    docker exec -u mattermost "$CONTAINER" /mattermost/bin/mmctl --local team users add test-team admin 2>/dev/null || true
    
    print_success "Mattermost setup complete"
}

# Create Matrix admin user
setup_synapse() {
    print_step "Setting up Synapse..."
    
    CONTAINER=$(docker compose ps -q synapse)
    
    # Wait for Synapse to be fully ready
    sleep 5
    
    # Register admin user
    print_step "Creating Matrix admin user..."
    docker exec "$CONTAINER" register_new_matrix_user \
        -c /data/homeserver.yaml \
        -u admin \
        -p admin123 \
        -a \
        http://localhost:8008 2>/dev/null || print_warning "Admin user may already exist"
    
    print_success "Synapse setup complete"
}

# Install plugin
install_plugin() {
    print_step "Installing plugin to Mattermost..."
    
    CONTAINER=$(docker compose ps -q mattermost)
    PLUGIN_FILE=$(ls dist/com.mattermost.plugin-matrix-bridge-*.tar.gz 2>/dev/null | head -n1)
    
    if [ -z "$PLUGIN_FILE" ]; then
        print_error "Plugin file not found in dist/. Run 'make dist' first."
        return 1
    fi
    
    # Copy plugin to container
    docker cp "$PLUGIN_FILE" "$CONTAINER":/tmp/plugin.tar.gz
    
    # Install via API (using admin token)
    # First, generate a token
    TOKEN_OUTPUT=$(docker exec -u mattermost "$CONTAINER" /mattermost/bin/mmctl --local token generate admin bridge-setup --json 2>/dev/null | head -n1)
    ADMIN_TOKEN=$(echo "$TOKEN_OUTPUT" | jq -r '.[0].token // .[].token // .token' 2>/dev/null || echo "")
    
    if [ -n "$ADMIN_TOKEN" ]; then
        # Upload plugin via API
        curl -s -X POST \
            -H "Authorization: Bearer $ADMIN_TOKEN" \
            -F "plugin=@$PLUGIN_FILE" \
            http://localhost:8065/api/v4/plugins > /dev/null
        
        # Enable plugin
        curl -s -X POST \
            -H "Authorization: Bearer $ADMIN_TOKEN" \
            -H "Content-Type: application/json" \
            http://localhost:8065/api/v4/plugins/com.mattermost.plugin-matrix-bridge/enable > /dev/null
        
        print_success "Plugin installed and enabled"
    else
        print_warning "Could not get admin token. Install plugin manually via System Console."
    fi
}

# Print final instructions
print_instructions() {
    echo ""
    echo "=========================================="
    echo -e "${GREEN}Local Development Environment Ready!${NC}"
    echo "=========================================="
    echo ""
    echo "Services running:"
    echo "  - Mattermost:    http://localhost:8065"
    echo "  - Matrix/Synapse: http://localhost:8888"
    echo "  - Element Web:   http://localhost:8080"
    echo ""
    echo "Credentials:"
    echo "  Mattermost: admin / Admin123!"
    echo "  Matrix:     admin / admin123"
    echo ""
    echo "Next steps:"
    echo "  1. Log into Mattermost at http://localhost:8065"
    echo "  2. Go to System Console → Plugins → Matrix Bridge"
    echo "  3. Set Matrix Server URL to: http://synapse:8008"
    echo "  4. Copy the tokens from .as_token and .hs_token files"
    echo "  5. Enable Message Sync"
    echo "  6. Create a channel and use /matrix create \"Room Name\""
    echo ""
    echo "To test Matrix directly:"
    echo "  - Open Element Web at http://localhost:8080"
    echo "  - Log in with the Matrix admin credentials"
    echo ""
    echo "Tokens (also saved in files):"
    echo "  AS Token: $(cat .as_token 2>/dev/null || echo 'Not generated')"
    echo "  HS Token: $(cat .hs_token 2>/dev/null || echo 'Not generated')"
    echo ""
}

# Main execution
main() {
    echo ""
    echo "=========================================="
    echo "Mattermost-Matrix Bridge Local Setup"
    echo "=========================================="
    echo ""
    
    check_prerequisites
    
    # Check if tokens exist, if not generate them
    if [ ! -f .as_token ] || [ ! -f .hs_token ]; then
        generate_tokens
        update_registration
    else
        print_step "Using existing tokens from .as_token and .hs_token"
    fi
    
    # Build if dist doesn't exist
    if [ ! -d dist ] || [ -z "$(ls dist/*.tar.gz 2>/dev/null)" ]; then
        build_plugin
    else
        print_step "Using existing plugin build in dist/"
    fi
    
    start_services
    setup_mattermost
    setup_synapse
    install_plugin
    print_instructions
}

# Run main function
main "$@"
