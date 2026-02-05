#!/bin/bash
set -e

# Configuration
SYNAPSE_URL=${SYNAPSE_URL:-"http://localhost:40008"}
MM_URL=${MM_URL:-"http://localhost:40065"}
# Note: MM_ADMIN_TOKEN is loaded from config.yaml below if not set
if [ -z "$MM_ADMIN_TOKEN" ]; then
    if [ -f config.yaml ]; then
        MM_ADMIN_TOKEN=$(grep -w "admin_token:" config.yaml | head -n1 | cut -d'"' -f2)
    fi
fi

if [ -z "$MM_ADMIN_TOKEN" ]; then
    echo "Error: MM_ADMIN_TOKEN not set and could not find admin_token in config.yaml"
    exit 1
fi

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}=== Bridge Verification Test ===${NC}"

# 1. Create Matrix User (using curl against Synapse)
echo -e "${BLUE}[1/5] Creating Matrix user 'verifier'...${NC}"
# Use shared secret registration or just skip if exists?
# For simplicity in this dev environment, we'll try to register using the shared secret from registration.yaml isn't for users.
# We'll use the command line registration tool in the synapse container if possible, or just standard registration if open.
# Synapse config said: enable_registration: true
# So we can just register via API.

MATRIX_USER="verifier_$(date +%s)"
MATRIX_PASS="password123"

REGISTER_RESP=$(curl -s -X POST "$SYNAPSE_URL/_matrix/client/r0/register" \
    -H "Content-Type: application/json" \
    -d "{
        \"username\": \"$MATRIX_USER\",
        \"password\": \"$MATRIX_PASS\",
        \"auth\": { \"type\": \"m.login.dummy\" }
    }")

ACCESS_TOKEN=$(echo $REGISTER_RESP | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
USER_ID=$(echo $REGISTER_RESP | sed -n 's/.*"user_id":"\([^"]*\)".*/\1/p')

if [ -z "$ACCESS_TOKEN" ]; then
    echo "Error registering Matrix user: $REGISTER_RESP"
    exit 1
fi
echo "✓ Created user $USER_ID"

# 2. Get Mattermost Channel ID
echo -e "${BLUE}[2/5] Getting Mattermost Channel ID...${NC}"
# We know the team is 'test-team' and channel is 'test-channel' from provision_mm.sh
# Need to fetch Team ID then Channel ID
MM_API="$MM_URL/api/v4"
MM_AUTH="Authorization: Bearer $MM_ADMIN_TOKEN"

TEAM_ID=$(curl -s -H "$MM_AUTH" "$MM_API/teams/name/test-team" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
CHANNEL_ID=$(curl -s -H "$MM_AUTH" "$MM_API/teams/$TEAM_ID/channels/name/test-channel" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

echo "✓ Found Channel ID: $CHANNEL_ID"

# 3. Matrix -> Mattermost Test
echo -e "${BLUE}[3/5] Testing Matrix -> Mattermost...${NC}"
ROOM_ALIAS="#mattermost_test-team_test-channel:localhost"
MSG_BODY="Hello from Matrix ($MATRIX_USER)"

# Join Room
echo "Joining $ROOM_ALIAS..."
# Encode # as %23 for URL
ENCODED_ALIAS=$(echo "$ROOM_ALIAS" | sed 's/#/%23/')
JOIN_RESP=$(curl -s -X POST "$SYNAPSE_URL/_matrix/client/r0/join/$ENCODED_ALIAS" \
    -H "Authorization: Bearer $ACCESS_TOKEN")
echo "Join Response: $JOIN_RESP"

# Send Message
echo "Sending message..."
curl -s -X POST "$SYNAPSE_URL/_matrix/client/r0/rooms/$ENCODED_ALIAS/send/m.room.message" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"msgtype\": \"m.text\",
        \"body\": \"$MSG_BODY\"
    }" > /dev/null

# Check Mattermost
echo "Verifying in Mattermost..."
sleep 2 # Wait for sync
MM_POSTS=$(curl -s -H "$MM_AUTH" "$MM_API/channels/$CHANNEL_ID/posts")
if echo "$MM_POSTS" | grep -q "$MSG_BODY"; then
    echo -e "${GREEN}✓ Success: Matrix message found in Mattermost${NC}"
else
    echo "Error: Message not found in Mattermost"
    exit 1
fi

# 4. Mattermost -> Matrix Test
echo -e "${BLUE}[4/5] Testing Mattermost -> Matrix...${NC}"
MM_MSG_BODY="Hello from Mattermost (Automated)"

# Send Message via MM API
curl -s -X POST "$MM_API/posts" \
    -H "$MM_AUTH" \
    -H "Content-Type: application/json" \
    -d "{
        \"channel_id\": \"$CHANNEL_ID\",
        \"message\": \"$MM_MSG_BODY\"
    }" > /dev/null

# Check Matrix (Sync)
echo "Verifying in Matrix..."
sleep 2
SYNC_RESP=$(curl -s -X GET "$SYNAPSE_URL/_matrix/client/r0/sync?timeout=3000" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

if echo "$SYNC_RESP" | grep -q "$MM_MSG_BODY"; then
    echo -e "${GREEN}✓ Success: Mattermost message found in Matrix${NC}"
else
    echo "Error: Message not found in Matrix"
    exit 1
fi

echo ""
echo -e "${GREEN}=== All Tests Passed ===${NC}"
