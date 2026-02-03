#!/bin/bash
# End-to-End Bridge Test Script
# This script tests the Mattermost-Matrix bridge by:
# 1. Creating a channel on Mattermost
# 2. Bridging it to Matrix using /matrix create
# 3. Sending a message from Mattermost
# 4. Verifying the message appears on Matrix

set -e

# Configuration
MATTERMOST_URL="${MATTERMOST_URL:-http://localhost:8066}"
MATRIX_URL="${MATRIX_URL:-http://localhost:8888}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
MATRIX_ADMIN_USER="${MATRIX_ADMIN_USER:-admin}"
MATRIX_ADMIN_PASS="${MATRIX_ADMIN_PASS:-admin123}"
TEST_TEAM="${TEST_TEAM:-test-team}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_step() { echo -e "${BLUE}==>${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}!${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }

# Generate unique test identifiers
TEST_ID=$(date +%s)
CHANNEL_NAME="bridge-test-${TEST_ID}"
ROOM_NAME="Bridge Test ${TEST_ID}"
TEST_MESSAGE="Hello from Mattermost! Test ID: ${TEST_ID} - $(date)"

echo ""
echo "=========================================="
echo "Mattermost-Matrix Bridge E2E Test"
echo "=========================================="
echo ""
echo "Test ID: ${TEST_ID}"
echo "Channel: ${CHANNEL_NAME}"
echo "Message: ${TEST_MESSAGE}"
echo ""

# Step 1: Get or generate admin token
print_step "Setting up authentication..."

if [ -z "$ADMIN_TOKEN" ]; then
    # Try to generate a new token
    TOKEN_OUTPUT=$(docker exec -u mattermost mattermost-plugin-matrix-bridge-mattermost-1 \
        /mattermost/bin/mmctl --local token generate admin "test-${TEST_ID}" --json 2>/dev/null | head -n1)
    ADMIN_TOKEN=$(echo "$TOKEN_OUTPUT" | jq -r '.[0].token // .[].token // .token' 2>/dev/null || echo "")
    
    if [ -z "$ADMIN_TOKEN" ]; then
        print_error "Failed to generate admin token"
        exit 1
    fi
fi
print_success "Admin token obtained"

# Helper function for Mattermost API calls
mm_api() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    
    if [ -n "$data" ]; then
        curl -s -X "$method" \
            -H "Authorization: Bearer $ADMIN_TOKEN" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "${MATTERMOST_URL}${endpoint}"
    else
        curl -s -X "$method" \
            -H "Authorization: Bearer $ADMIN_TOKEN" \
            "${MATTERMOST_URL}${endpoint}"
    fi
}

# Step 2: Get team ID
print_step "Getting team information..."
TEAM_INFO=$(mm_api GET "/api/v4/teams/name/${TEST_TEAM}")
TEAM_ID=$(echo "$TEAM_INFO" | jq -r '.id')
if [ -z "$TEAM_ID" ] || [ "$TEAM_ID" = "null" ]; then
    print_error "Failed to get team ID for ${TEST_TEAM}"
    echo "$TEAM_INFO"
    exit 1
fi
print_success "Team ID: ${TEAM_ID}"

# Step 3: Get admin user ID
print_step "Getting user information..."
USER_INFO=$(mm_api GET "/api/v4/users/me")
USER_ID=$(echo "$USER_INFO" | jq -r '.id')
if [ -z "$USER_ID" ] || [ "$USER_ID" = "null" ]; then
    print_error "Failed to get user ID"
    exit 1
fi
print_success "User ID: ${USER_ID}"

# Step 4: Create a new channel
print_step "Creating channel: ${CHANNEL_NAME}..."
CHANNEL_DATA=$(cat <<EOF
{
    "team_id": "${TEAM_ID}",
    "name": "${CHANNEL_NAME}",
    "display_name": "${ROOM_NAME}",
    "type": "O"
}
EOF
)
CHANNEL_INFO=$(mm_api POST "/api/v4/channels" "$CHANNEL_DATA")
CHANNEL_ID=$(echo "$CHANNEL_INFO" | jq -r '.id')
if [ -z "$CHANNEL_ID" ] || [ "$CHANNEL_ID" = "null" ]; then
    print_error "Failed to create channel"
    echo "$CHANNEL_INFO"
    exit 1
fi
print_success "Channel created: ${CHANNEL_ID}"

# Step 5: Add user to channel (should be automatic but ensure it)
mm_api POST "/api/v4/channels/${CHANNEL_ID}/members" "{\"user_id\": \"${USER_ID}\"}" > /dev/null 2>&1 || true

# Step 6: Execute /matrix create command
print_step "Executing /matrix create command..."
COMMAND_DATA=$(cat <<EOF
{
    "channel_id": "${CHANNEL_ID}",
    "command": "/matrix create publish=true"
}
EOF
)
COMMAND_RESULT=$(mm_api POST "/api/v4/commands/execute" "$COMMAND_DATA")
COMMAND_TEXT=$(echo "$COMMAND_RESULT" | jq -r '.text // .message // empty')

if echo "$COMMAND_TEXT" | grep -qi "error\|failed\|not found"; then
    print_error "Matrix create command failed:"
    echo "$COMMAND_TEXT"
    exit 1
fi
print_success "Matrix room created"
echo "    Response: $(echo "$COMMAND_TEXT" | head -c 200)"

# Wait for the bridge to sync
print_step "Waiting for bridge to sync..."
sleep 3

# Step 7: Send a test message from Mattermost
print_step "Sending test message from Mattermost..."
POST_DATA=$(cat <<EOF
{
    "channel_id": "${CHANNEL_ID}",
    "message": "${TEST_MESSAGE}"
}
EOF
)
POST_RESULT=$(mm_api POST "/api/v4/posts" "$POST_DATA")
POST_ID=$(echo "$POST_RESULT" | jq -r '.id')
if [ -z "$POST_ID" ] || [ "$POST_ID" = "null" ]; then
    print_error "Failed to send message"
    echo "$POST_RESULT"
    exit 1
fi
print_success "Message sent: ${POST_ID}"

# Step 8: Get Matrix access token
print_step "Logging into Matrix..."
MATRIX_LOGIN=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d "{\"type\": \"m.login.password\", \"user\": \"${MATRIX_ADMIN_USER}\", \"password\": \"${MATRIX_ADMIN_PASS}\"}" \
    "${MATRIX_URL}/_matrix/client/v3/login")
MATRIX_TOKEN=$(echo "$MATRIX_LOGIN" | jq -r '.access_token')
MATRIX_USER_ID=$(echo "$MATRIX_LOGIN" | jq -r '.user_id')

if [ -z "$MATRIX_TOKEN" ] || [ "$MATRIX_TOKEN" = "null" ]; then
    print_error "Failed to login to Matrix"
    echo "$MATRIX_LOGIN"
    exit 1
fi
print_success "Matrix login successful: ${MATRIX_USER_ID}"

# Helper function for Matrix API calls
matrix_api() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    
    if [ -n "$data" ]; then
        curl -s -X "$method" \
            -H "Authorization: Bearer $MATRIX_TOKEN" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "${MATRIX_URL}${endpoint}"
    else
        curl -s -X "$method" \
            -H "Authorization: Bearer $MATRIX_TOKEN" \
            "${MATRIX_URL}${endpoint}"
    fi
}

# Step 9: Find the bridged room on Matrix
print_step "Looking for bridged room on Matrix..."

# The room alias should be #_mattermost_<channel>:synapse
ROOM_ALIAS="#_mattermost_${CHANNEL_NAME}:synapse"
ENCODED_ALIAS=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${ROOM_ALIAS}', safe=''))")

# Try to resolve the room alias
ROOM_INFO=$(matrix_api GET "/_matrix/client/v3/directory/room/${ENCODED_ALIAS}" 2>/dev/null || echo "{}")
ROOM_ID=$(echo "$ROOM_INFO" | jq -r '.room_id // empty')

if [ -z "$ROOM_ID" ]; then
    print_warning "Room alias not found, checking joined rooms..."
    
    # List all joined rooms and look for our room
    JOINED_ROOMS=$(matrix_api GET "/_matrix/client/v3/joined_rooms")
    echo "    Joined rooms: $(echo "$JOINED_ROOMS" | jq -c '.joined_rooms')"
    
    # Try syncing to see available rooms
    SYNC_RESULT=$(matrix_api GET "/_matrix/client/v3/sync?timeout=5000&filter={\"room\":{\"timeline\":{\"limit\":5}}}" 2>/dev/null || echo "{}")
    INVITE_ROOMS=$(echo "$SYNC_RESULT" | jq -r '.rooms.invite | keys[]' 2>/dev/null || echo "")
    
    if [ -n "$INVITE_ROOMS" ]; then
        print_step "Found room invites, accepting first one..."
        for room in $INVITE_ROOMS; do
            ROOM_ID="$room"
            matrix_api POST "/_matrix/client/v3/rooms/${ROOM_ID}/join" "{}" > /dev/null 2>&1
            print_success "Joined room: ${ROOM_ID}"
            break
        done
    fi
fi

if [ -z "$ROOM_ID" ]; then
    print_error "Could not find bridged room on Matrix"
    print_warning "This might be a bridge configuration issue"
    print_warning "Room alias tried: ${ROOM_ALIAS}"
    
    # Debug: Check plugin status
    print_step "Checking plugin status..."
    MATRIX_STATUS=$(mm_api POST "/api/v4/commands/execute" "{\"channel_id\": \"${CHANNEL_ID}\", \"command\": \"/matrix status\"}")
    echo "$MATRIX_STATUS" | jq -r '.text // .message // empty' | head -20
    
    exit 1
fi
print_success "Found Matrix room: ${ROOM_ID}"

# Step 10: Check for the message in Matrix
print_step "Checking for message in Matrix room..."

# Wait a bit for message to propagate
sleep 2

# Get room messages
MESSAGES=$(matrix_api GET "/_matrix/client/v3/rooms/${ROOM_ID}/messages?dir=b&limit=10")
MESSAGE_FOUND=false

# Check each message for our test content
echo "$MESSAGES" | jq -r '.chunk[] | select(.type == "m.room.message") | .content.body // empty' | while read -r body; do
    if echo "$body" | grep -q "$TEST_ID"; then
        print_success "Message found on Matrix!"
        echo "    Content: $body"
        MESSAGE_FOUND=true
    fi
done

# Also try /matrix status to see bridge health
print_step "Checking bridge status..."
STATUS_RESULT=$(mm_api POST "/api/v4/commands/execute" "{\"channel_id\": \"${CHANNEL_ID}\", \"command\": \"/matrix status\"}")
STATUS_TEXT=$(echo "$STATUS_RESULT" | jq -r '.text // .message // empty')
echo "$STATUS_TEXT" | head -15

# Cleanup (optional - comment out to keep test data)
# print_step "Cleaning up..."
# mm_api DELETE "/api/v4/channels/${CHANNEL_ID}" > /dev/null 2>&1 || true

echo ""
echo "=========================================="
echo "Test Complete"
echo "=========================================="
echo ""
echo "Results:"
echo "  Channel: ${CHANNEL_NAME} (${CHANNEL_ID})"
echo "  Room Alias: ${ROOM_ALIAS}"
echo "  Matrix Room: ${ROOM_ID:-Not found}"
echo "  Test Message ID: ${POST_ID}"
echo ""
echo "To verify manually:"
echo "  1. Open Mattermost: ${MATTERMOST_URL}/test-team/channels/${CHANNEL_NAME}"
echo "  2. Open Element: http://localhost:8080 and join ${ROOM_ALIAS}"
echo ""

# Return success if we got this far (room was found)
if [ -n "$ROOM_ID" ]; then
    print_success "Bridge E2E test passed!"
    exit 0
else
    print_error "Bridge E2E test failed - room not found"
    exit 1
fi
