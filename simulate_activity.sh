#!/bin/bash
set -e

# Simulate activity on LOCAL test server
MATTERMOST_URL="http://localhost:40065"
ADMIN_USER="sysadmin"
ADMIN_PASSWORD="Sys@dmin123"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}=== Local Mattermost Activity Simulation ===${NC}"
echo -e "Target: ${BLUE}${MATTERMOST_URL}${NC}\n"

# Function to login and get token
login_user() {
    local user=$1
    local pass=$2
    TOKEN=$(curl -s -i -X POST "${MATTERMOST_URL}/api/v4/users/login" \
      -H "Content-Type: application/json" \
      -d "{\"login_id\":\"${user}\",\"password\":\"${pass}\"}" 2>/dev/null | grep -i "^token:" | awk '{print $2}' | tr -d '\r')
    echo "$TOKEN"
}

# Function to get user ID
get_user_id() {
    local token=$1
    local response=$(curl -s -X GET "${MATTERMOST_URL}/api/v4/users/me" \
      -H "Authorization: Bearer ${token}" 2>/dev/null)
    echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null || echo ""
}

# Function to get team ID
get_team_id() {
    local team=$1
    local token=$2
    
    RESPONSE=$(curl -s -X GET "${MATTERMOST_URL}/api/v4/teams/name/${team}" \
      -H "Authorization: Bearer ${token}" 2>/dev/null)
    echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null || echo ""
}

# Function to get channel ID
get_channel_id() {
    local team=$1
    local channel=$2
    local token=$3
    
    RESPONSE=$(curl -s -X GET "${MATTERMOST_URL}/api/v4/teams/name/${team}/channels/name/${channel}" \
      -H "Authorization: Bearer ${token}" 2>/dev/null)
    echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null || echo ""
}

# Function to post message
post_message() {
    local channel_id=$1
    local message=$2
    local token=$3
    local root_id=$4
    
    if [ -n "$root_id" ]; then
        POST_JSON="{\"channel_id\":\"${channel_id}\",\"message\":\"${message}\",\"root_id\":\"${root_id}\"}"
    else
        POST_JSON="{\"channel_id\":\"${channel_id}\",\"message\":\"${message}\"}"
    fi
    
    RESPONSE=$(curl -s -X POST "${MATTERMOST_URL}/api/v4/posts" \
      -H "Authorization: Bearer ${token}" \
      -H "Content-Type: application/json" \
      -d "$POST_JSON" 2>/dev/null)
    echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null || echo ""
}

# Function to add reaction
add_reaction() {
    local user_id=$1
    local post_id=$2
    local emoji=$3
    local token=$4
    
    curl -s -X POST "${MATTERMOST_URL}/api/v4/reactions" \
      -H "Authorization: Bearer ${token}" \
      -H "Content-Type: application/json" \
      -d "{\"user_id\":\"${user_id}\",\"post_id\":\"${post_id}\",\"emoji_name\":\"${emoji}\"}" > /dev/null 2>&1
}

# Step 1: Login as admin
echo -e "${BLUE}[1/5] Logging in as ${ADMIN_USER}...${NC}"
TOKEN=$(login_user "$ADMIN_USER" "$ADMIN_PASSWORD")
if [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ Failed to login. Is Mattermost running?${NC}"
    echo "  Try: podman-compose up -d && ./provision_mm.sh"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Logged in successfully"

USER_ID=$(get_user_id "$TOKEN")
echo -e "  ${GREEN}✓${NC} User ID: ${USER_ID:0:8}..."

# Step 2: Get team and channel info
echo -e "\n${BLUE}[2/5] Finding test-team and test-channel...${NC}"
TEAM_ID=$(get_team_id "test-team" "$TOKEN")
if [ -z "$TEAM_ID" ]; then
    echo -e "${RED}✗ Team 'test-team' not found. Run ./provision_mm.sh first${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Team ID: ${TEAM_ID:0:8}..."

CHANNEL_ID=$(get_channel_id "test-team" "test-channel" "$TOKEN")
if [ -z "$CHANNEL_ID" ]; then
    echo -e "${RED}✗ Channel 'test-channel' not found${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓${NC} Channel ID: ${CHANNEL_ID:0:8}..."

# Also get town-square (default channel)
TOWN_SQUARE_ID=$(get_channel_id "test-team" "town-square" "$TOKEN")
echo -e "  ${GREEN}✓${NC} Town Square ID: ${TOWN_SQUARE_ID:0:8}..."

# Step 3: Post messages to test-channel
echo -e "\n${BLUE}[3/5] Posting messages to test-channel...${NC}"

MESSAGES=(
    "👋 Hello from the bridge test! This message should appear in Matrix."
    "Testing **bold** and *italics* markdown formatting."
    "Here's a code block: \`console.log('Hello Matrix!');\`"
    "Let's test some emoji: 🎉 🚀 ✨ 🔥"
    "This is a longer message to test how the bridge handles multi-line content.\n\nWith multiple paragraphs!"
)

POST_IDS=()
for msg in "${MESSAGES[@]}"; do
    POST_ID=$(post_message "$CHANNEL_ID" "$msg" "$TOKEN")
    if [ -n "$POST_ID" ]; then
        POST_IDS+=("$POST_ID")
        echo -e "  ${GREEN}✓${NC} Posted: ${msg:0:50}..."
    else
        echo -e "  ${RED}✗${NC} Failed to post message"
    fi
    sleep 0.5
done

# Step 4: Test threads and reactions
echo -e "\n${BLUE}[4/5] Creating thread replies and reactions...${NC}"

if [ -n "${POST_IDS[0]}" ]; then
    # Reply to first message as a thread
    REPLY_ID=$(post_message "$CHANNEL_ID" "This is a **thread reply** to test threading support! 🧵" "$TOKEN" "${POST_IDS[0]}")
    if [ -n "$REPLY_ID" ]; then
        echo -e "  ${GREEN}✓${NC} Created thread reply"
    fi
    
    # Add reactions
    add_reaction "$USER_ID" "${POST_IDS[0]}" "thumbsup" "$TOKEN"
    echo -e "  ${GREEN}✓${NC} Added 👍 reaction"
    
    add_reaction "$USER_ID" "${POST_IDS[1]}" "heart" "$TOKEN"
    echo -e "  ${GREEN}✓${NC} Added ❤️ reaction"
    
    add_reaction "$USER_ID" "${POST_IDS[2]}" "rocket" "$TOKEN"
    echo -e "  ${GREEN}✓${NC} Added 🚀 reaction"
fi

# Step 5: Post to town-square
echo -e "\n${BLUE}[5/5] Posting to town-square...${NC}"

if [ -n "$TOWN_SQUARE_ID" ]; then
    TS_POST_ID=$(post_message "$TOWN_SQUARE_ID" "📢 Announcement: Bridge testing in progress! Check test-channel for activity." "$TOKEN")
    if [ -n "$TS_POST_ID" ]; then
        echo -e "  ${GREEN}✓${NC} Posted announcement to town-square"
    fi
fi

# Summary
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✓ Activity simulation complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "\n${YELLOW}Summary:${NC}"
echo "  - Posted ${#MESSAGES[@]} messages to test-channel"
echo "  - Created 1 thread reply"
echo "  - Added 3 reactions"
echo "  - Posted 1 message to town-square"
echo ""
echo -e "${BLUE}Verify in Mattermost:${NC} http://localhost:40065"
echo -e "${BLUE}Verify in Element:${NC} http://localhost:40080"
echo ""
echo -e "${YELLOW}Check bridge logs:${NC}"
echo "  podman logs -f mautrix-mattermost_bridge_1"
echo ""
