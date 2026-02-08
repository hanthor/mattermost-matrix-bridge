package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"time"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/id"

	"github.com/mattermost/mattermost/server/public/model"
)

// SlashCommandRequest represents a request from a Mattermost slash command webhook.
// See: https://developers.mattermost.com/integrate/slash-commands/
type SlashCommandRequest struct {
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Command     string `json:"command"`
	ResponseURL string `json:"response_url"`
	TeamDomain  string `json:"team_domain"`
	TeamID      string `json:"team_id"`
	Text        string `json:"text"`
	Token       string `json:"token"`
	TriggerID   string `json:"trigger_id"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
}

// SlashCommandResponse is the JSON response sent back to Mattermost.
type SlashCommandResponse struct {
	ResponseType string `json:"response_type"` // "ephemeral" or "in_channel"
	Text         string `json:"text"`
}

// SlashCommandHandler holds the connector and token for handling slash commands.
type SlashCommandHandler struct {
	Connector *MattermostConnector
	Token     string // Expected token from Mattermost to verify requests
}

// NewSlashCommandHandler creates a new handler for Mattermost slash commands.
func NewSlashCommandHandler(connector *MattermostConnector, token string) *SlashCommandHandler {
	return &SlashCommandHandler{
		Connector: connector,
		Token:     token,
	}
}

// ServeHTTP implements http.Handler for the slash command endpoint.
func (h *SlashCommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	req := SlashCommandRequest{
		ChannelID:   r.FormValue("channel_id"),
		ChannelName: r.FormValue("channel_name"),
		Command:     r.FormValue("command"),
		ResponseURL: r.FormValue("response_url"),
		TeamDomain:  r.FormValue("team_domain"),
		TeamID:      r.FormValue("team_id"),
		Text:        r.FormValue("text"),
		Token:       r.FormValue("token"),
		TriggerID:   r.FormValue("trigger_id"),
		UserID:      r.FormValue("user_id"),
		UserName:    r.FormValue("user_name"),
	}

	// Verify token if configured
	if h.Token != "" && req.Token != h.Token {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	resp := h.handleCommand(context.Background(), &req)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Printf("ERROR: Failed to encode slash command response: %v\n", err)
	}
}

// handleCommand routes the command to the appropriate handler.
func (h *SlashCommandHandler) handleCommand(ctx context.Context, req *SlashCommandRequest) *SlashCommandResponse {
	parts := strings.Fields(req.Text)
	if len(parts) == 0 {
		return h.helpResponse()
	}

	subcommand := strings.ToLower(parts[0])
	args := parts[1:]

	switch subcommand {
	case "help":
		return h.helpResponse()
	case "status":
		return h.statusResponse(ctx)
	case "join":
		return h.joinResponse(ctx, req.UserID, args)
	case "dm":
		return h.dmResponse(ctx, req.UserID, req.TeamDomain, args)
	case "me":
		return h.meResponse(ctx, req.UserID)
	case "rooms":
		return h.roomsResponse(ctx, req.UserID)
	case "account":
		return h.accountResponse(ctx, req.UserID, req.UserName)
	default:
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("Unknown subcommand: `%s`. Use `/matrix help` for available commands.", subcommand),
		}
	}
}

// helpResponse returns the help text.
func (h *SlashCommandHandler) helpResponse() *SlashCommandResponse {
	helpText := `**Matrix Bridge Commands**

• ` + "`/matrix help`" + ` - Show this help message
• ` + "`/matrix status`" + ` - Show bridge status
• ` + "`/matrix me`" + ` - Show your Matrix user info
• ` + "`/matrix join <room>`" + ` - Join a Matrix room (e.g., ` + "`#room:matrix.org`" + `)
• ` + "`/matrix dm <user>`" + ` - Start a DM with a Matrix user (e.g., ` + "`@user:matrix.org`" + `)
• ` + "`/matrix rooms`" + ` - List your bridged Matrix rooms
• ` + "`/matrix account`" + ` - Get your Matrix account credentials`

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text:         helpText,
	}
}

// statusResponse returns the bridge status.
func (h *SlashCommandHandler) statusResponse(ctx context.Context) *SlashCommandResponse {
	var statusLines []string
	statusLines = append(statusLines, "**Matrix Bridge Status**")
	statusLines = append(statusLines, "")

	// Check connection
	if h.Connector.Client != nil {
		statusLines = append(statusLines, "• **Mattermost**: Connected to "+h.Connector.Config.ServerURL)
	} else {
		statusLines = append(statusLines, "• **Mattermost**: Not connected")
	}

	// Check WebSocket
	if h.Connector.WSClient != nil {
		statusLines = append(statusLines, "• **WebSocket**: Connected")
	} else {
		statusLines = append(statusLines, "• **WebSocket**: Not connected")
	}

	// Check mode
	mode := string(h.Connector.Config.Mode)
	if mode == "" {
		mode = "puppet"
	}
	statusLines = append(statusLines, "• **Mode**: "+mode)

	// Check logged-in users
	users := h.Connector.GetUsers()
	statusLines = append(statusLines, fmt.Sprintf("• **Logged-in users**: %d", len(users)))

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text:         strings.Join(statusLines, "\n"),
	}
}

// meResponse shows the user's Matrix info.
func (h *SlashCommandHandler) meResponse(ctx context.Context, userID string) *SlashCommandResponse {
	// Look up the user in the bridge
	users := h.Connector.GetUsers()
	for _, login := range users {
		// Check if this login maps to the requesting user
		// In mirror mode, we may have a single admin login
		username := h.Connector.GetUsername(ctx, userID)
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("**Your Matrix Info**\n\n• **Username**: `%s`\n• **Matrix ID**: `%s` \n• **Status**: ✅ Connected", username, login.User.MXID),
		}
	}

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text:         "No Matrix login found. The bridge may be using a shared admin account.",
	}
}

// joinResponse handles joining a Matrix room.
func (h *SlashCommandHandler) joinResponse(ctx context.Context, userID string, args []string) *SlashCommandResponse {
	if len(args) == 0 {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "Usage: `/matrix join <room>` - e.g., `/matrix join #test:matrix.org`",
		}
	}

	roomIdentifier := args[0]
	if !strings.HasPrefix(roomIdentifier, "#") && !strings.HasPrefix(roomIdentifier, "!") {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "Invalid room identifier. Use a room alias (e.g., `#room:server.com`) or room ID (e.g., `!abc123:server.com`).",
		}
	}

	// Check if Synapse Admin API is configured
	if h.Connector.Config.SynapseAdmin.URL == "" || h.Connector.Config.SynapseAdmin.Token == "" {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "❌ Synapse Admin API is not configured. Contact your administrator to enable this feature.",
		}
	}

	// Create Matrix Admin client
	admin := NewMatrixAdminClient(h.Connector.Config.SynapseAdmin.URL, h.Connector.Config.SynapseAdmin.Token)

	// Get Mattermost user to generate Matrix ID
	mmUser, _, err := h.Connector.Client.GetUser(ctx, userID, "")
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to get your Mattermost user info: %v", err),
		}
	}

	// Generate Matrix user ID from Mattermost username
	serverName := h.Connector.Bridge.Matrix.ServerName()
	matrixUserID := GenerateMatrixUserID(mmUser, serverName)

	// Try to ensure Matrix user exists (create if needed)
	// This is optional - if admin API isn't available or lacks permissions, we'll still try to join
	if admin != nil {
		exists, err := admin.UserExists(ctx, matrixUserID)
		if err != nil {
			// Admin API not working - log warning but continue
			// The user might already exist, or the join will fail with a clear error
			fmt.Printf("WARN: Cannot check if Matrix user exists (admin API issue): %v\n", err)
			fmt.Printf("INFO: Will attempt to join anyway - if user doesn't exist, join will fail\n")
		} else if !exists {
			// Try to create Matrix account
			displayName := mmUser.GetDisplayName(model.ShowFullName)
			if displayName == "" {
				displayName = mmUser.Username
			}
			password := GeneratePassword()

			err = admin.CreateUser(ctx, matrixUserID, password, displayName)
			if err != nil {
				fmt.Printf("WARN: Failed to create Matrix user %s: %v\n", matrixUserID, err)
				// Continue anyway - user might exist despite the check failing
			} else {
				fmt.Printf("INFO: Created Matrix user %s\n", matrixUserID)
			}
		}
	}

	// Resolve room alias to room ID if needed, capturing via servers for federation
	var roomID id.RoomID
	var viaServers []string
	if strings.HasPrefix(roomIdentifier, "#") {
		resolvedID, servers, err := admin.ResolveRoomAlias(ctx, roomIdentifier)
		if err != nil {
			return &SlashCommandResponse{
				ResponseType: "ephemeral",
				Text:         fmt.Sprintf("❌ Failed to resolve room alias `%s`: %v\n\nMake sure the room exists and is accessible.", roomIdentifier, err),
			}
		}
		roomID = resolvedID
		viaServers = servers
		fmt.Printf("DEBUG: Resolved alias %s to room %s with via servers: %v\n", roomIdentifier, roomID, viaServers)
	} else {
		roomID = id.RoomID(roomIdentifier)
		// Extract server from room ID for federation (e.g., !abc:matrix.org -> matrix.org)
		parts := strings.SplitN(string(roomID), ":", 2)
		if len(parts) == 2 {
			viaServers = []string{parts[1]}
		}
	}

	// Get room info to determine if public or private
	roomInfo, err := admin.GetRoomInfo(ctx, roomID)
	isPublic := true // Default to public
	if err == nil && roomInfo != nil {
		if joinRule, ok := roomInfo["join_rule"].(string); ok {
			isPublic = (joinRule == "public")
		}
	}

	// Get the ghost for this user so we can use their Matrix identity
	ghost, err := h.Connector.Bridge.GetGhostByID(ctx, networkid.UserID(mmUser.Username))
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to get Matrix ghost for user: %v\n\nYour Matrix account may not be set up. Try running `/matrix account` first.", err),
		}
	}

	// Get the ghost's MXID
	ghostMXID := ghost.Intent.GetMXID()
	fmt.Printf("DEBUG: Attempting to join room %s as ghost %s with via servers: %v\n", roomID, ghostMXID, viaServers)

	// Join the room using our custom federation-aware join method
	err = admin.JoinRoomVia(ctx, ghostMXID, roomID, viaServers)
	if err != nil {
		fmt.Printf("ERROR: Failed to join room %s as %s: %v\n", roomID, ghostMXID, err)
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to join Matrix room: %v\n\nThe room may be invite-only or your Matrix account may not have access.", err),
		}
	}

	// Generate Mattermost channel name
	channelName := sanitizeChannelName(roomIdentifier)

	// Get any available login for creating the portal and setting up relay
	users := h.Connector.GetUsers()
	if len(users) == 0 {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "❌ No bridge logins available. The bridge may not be fully configured.",
		}
	}
	login := users[0]

	// Create Mattermost channel
	channelType := model.ChannelTypeOpen
	if !isPublic {
		channelType = model.ChannelTypePrivate
	}

	// Get user's team - we need to know which team to create the channel in
	// For now, we'll get the first team the user is a member of
	teams, err := h.Connector.Client.GetTeamsForUser(ctx, userID)
	if err != nil || len(teams) == 0 {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "❌ Could not find a team to create the channel in. Make sure you're a member of at least one team.",
		}
	}
	teamID := teams[0].Id

	newChannel := &model.Channel{
		TeamId:      teamID,
		Type:        channelType,
		DisplayName: fmt.Sprintf("Matrix: %s", roomIdentifier),
		Name:        channelName,
		Purpose:     fmt.Sprintf("Bridged from Matrix room %s", roomID),
	}

	createdChannel, _, err := h.Connector.Client.CreateChannel(ctx, newChannel)
	if err != nil {
		// Channel might already exist, try to get it
		existingChannel, _, err2 := h.Connector.Client.GetChannelByName(ctx, channelName, teamID, "")
		if err2 == nil && existingChannel != nil {
			createdChannel = existingChannel
		} else {
			return &SlashCommandResponse{
				ResponseType: "ephemeral",
				Text:         fmt.Sprintf("❌ Failed to create Mattermost channel: %v", err),
			}
		}
	}

	// Add the requesting user to the channel
	_, _, err = h.Connector.Client.AddChannelMember(ctx, createdChannel.Id, userID)
	if err != nil {
		// Might already be a member, continue anyway
		fmt.Printf("DEBUG: Failed to add user to channel (may already be member): %v\n", err)
	}

	// Create portal mapping between Matrix room and Mattermost channel
	portalKey := networkid.PortalKey{
		ID: networkid.PortalID(createdChannel.Id),
	}

	portal, err := h.Connector.Bridge.GetPortalByKey(ctx, portalKey)
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("✅ Joined Matrix room and created channel, but failed to set up portal: %v", err),
		}
	}

	// If portal doesn't have an MXID yet, we need to set it
	if portal.MXID == "" {
		portal.MXID = roomID
		// Save the portal
		err = h.Connector.Bridge.DB.Portal.Update(ctx, portal.Portal)
		if err != nil {
			fmt.Printf("WARN: Failed to update portal MXID: %v\n", err)
		}
	}

	// Set up relay mode so the Matrix ghost account relays messages from other users
	// This is better than joining the bridge bot - the ghost can handle everything
	err = portal.SetRelay(ctx, login)
	if err != nil {
		fmt.Printf("WARN: Failed to set relay for portal %s: %v\n", portal.MXID, err)
		// Don't fail the command - basic bridging should still work
	}

	channelLink := fmt.Sprintf("/%s/channels/%s", teams[0].Name, createdChannel.Name)

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text: fmt.Sprintf("✅ **Successfully joined Matrix room!**\n\n"+
			"• **Matrix Room**: `%s`\n"+
			"• **Matrix Account**: `%s`\n"+
			"• **Mattermost Channel**: `%s` (%s)\n"+
			"• **[Open Channel](%s)**\n\n"+
			"Messages will now be bridged between the Matrix room and this Mattermost channel.",
			roomID, matrixUserID, createdChannel.DisplayName,
			map[bool]string{true: "public", false: "private"}[isPublic],
			channelLink),
	}
}

// sanitizeChannelName converts a Matrix room identifier to a valid Mattermost channel name
func sanitizeChannelName(matrixRoomID string) string {
	// Remove # or ! prefix
	name := strings.TrimPrefix(matrixRoomID, "#")
	name = strings.TrimPrefix(name, "!")

	// Replace invalid characters
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "-")

	// Add prefix to indicate it's from Matrix
	name = "mx_" + name

	// Ensure lowercase (Mattermost requirement)
	name = strings.ToLower(name)

	// Truncate if too long (Mattermost max is 64 chars)
	if len(name) > 64 {
		name = name[:64]
	}

	return name
}

// dmResponse handles starting a DM with a Matrix user.
func (h *SlashCommandHandler) dmResponse(ctx context.Context, userID, teamDomain string, args []string) *SlashCommandResponse {
	if len(args) == 0 {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "Usage: `/matrix dm <user>` - e.g., `/matrix dm @alice:matrix.org`",
		}
	}

	matrixUserID := args[0]
	if !strings.HasPrefix(matrixUserID, "@") || !strings.Contains(matrixUserID, ":") {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "Invalid Matrix user ID. Use the format `@user:server.com`.",
		}
	}

	// Get any available login to perform the operation
	users := h.Connector.GetUsers()
	if len(users) == 0 {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "❌ No bridge logins available. The bridge may not be fully configured.",
		}
	}
	login := users[0]

	// 1. Get/Provision the Mattermost user for this Matrix user
	// This returns the Mattermost UUID for the ghost
	mmRecipientID, err := h.getOrProvisionGhost(ctx, matrixUserID)
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to provision ghost user: %v", err),
		}
	}

	// 3. Get the ghost object using the Matrix User ID (the network ID)
	ghost, err := h.Connector.Bridge.GetGhostByID(ctx, networkid.UserID(matrixUserID))
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to resolve ghost: %v", err),
		}
	}

	// 4. Update ghost metadata with proper Mattermost UUID to ensure calls like CreateDirectChannelWithBoth work
	if ghost.Metadata == nil {
		ghost.Metadata = make(map[string]any)
	}
	// We need to handle the map type assertion safely
	var meta map[string]any
	if m, ok := ghost.Metadata.(map[string]any); ok {
		meta = m
	} else {
		meta = make(map[string]any)
	}

	meta["mm_id"] = mmRecipientID
	ghost.Metadata = meta

	// Persist the metadata
	if ghost.Ghost != nil {
		err = h.Connector.Bridge.DB.Ghost.Update(ctx, ghost.Ghost)
		if err != nil {
			// Log but continue, as in-memory metadata might be enough for this request
			fmt.Printf("DEBUG: Failed to update ghost metadata in DB: %v\n", err)
		}
	}

	// Try to create a DM with this ghost using the existing API
	api, ok := login.Client.(*MattermostAPI)
	if !ok || api == nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "❌ Bridge API not available.",
		}
	}

	// Attempt to create a DM channel
	// CreateChatWithGhost will use the mm_id from metadata
	chatResp, err := api.CreateChatWithGhost(ctx, ghost)
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to create DM channel: %v", err),
		}
	}

	// Post a starter message to make the channel visible in the UI
	post := &model.Post{
		ChannelId: string(chatResp.PortalKey.ID),
		Message:   fmt.Sprintf("Bridged DM with `%s` established. You can now chat with this user.", matrixUserID),
	}
	_, _, err = api.Client.CreatePost(ctx, post)
	if err != nil {
		// Log error but don't fail the command
		fmt.Printf("Failed to post starter message: %v\n", err)
	}

	channelID := string(chatResp.PortalKey.ID)
	channelLink := fmt.Sprintf("/%s/channels/%s", teamDomain, channelID)

	// Invite the real Matrix user to the room (async)
	// We use context.Background() so it doesn't get canceled when the slash command response is sent
	bgCtx := context.Background()
	portalKey := chatResp.PortalKey
	go func() {
		var portal *bridgev2.Portal
		var err error

		// Poll for up to 30 seconds for the portal to be created and have an MXID
		for i := 0; i < 60; i++ {
			time.Sleep(500 * time.Millisecond)
			portal, err = h.Connector.Bridge.GetPortalByKey(bgCtx, portalKey)
			if err == nil && portal != nil && portal.MXID != "" {
				break
			}
		}

		if portal != nil && portal.MXID != "" {
			// Invite the native Matrix user
			// We use the user's puppet intent (ghost or double puppet) to perform the invite
			// as it looks more natural than using the bridge bot and avoids the 403 error
			// if the bot isn't in the room yet.
			intent := login.User.DoublePuppet(bgCtx)
			err = intent.InviteUser(bgCtx, portal.MXID, id.UserID(matrixUserID))
			if err != nil {
				fmt.Printf("DEBUG: Failed to invite %s to %s: %v\n", matrixUserID, portal.MXID, err)
			} else {
				fmt.Printf("DEBUG: Successfully invited %s to %s\n", matrixUserID, portal.MXID)
			}

			// Set the portal relay so that the remote Matrix user can reply without being logged in
			err = portal.SetRelay(bgCtx, login)
			if err != nil {
				fmt.Printf("DEBUG: Failed to set relay for %s: %v\n", portal.MXID, err)
			} else {
				fmt.Printf("DEBUG: Successfully set relay for %s to %s\n", portal.MXID, login.ID)
			}
		} else {
			fmt.Printf("DEBUG: Failed to resolve portal MXID for invite after timeout\n")
		}
	}()

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text: fmt.Sprintf("✅ **DM created with Matrix user!**\n\n"+
			"• **Matrix User**: `%s`\n"+
			"• **Channel ID**: `%s`\n"+
			"• **[Open Direct Message](%s)**\n\n"+
			"A starter message has been posted to ensure the channel appears in your sidebar.",
			matrixUserID, channelID, channelLink),
	}
}

// roomsResponse lists the user's bridged Matrix rooms.
func (h *SlashCommandHandler) roomsResponse(ctx context.Context, userID string) *SlashCommandResponse {
	// Get all portals from the bridge
	users := h.Connector.GetUsers()
	if len(users) == 0 {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         "❌ No bridge logins available.",
		}
	}

	// Build a list of bridged rooms
	var roomLines []string
	roomLines = append(roomLines, "**Your Bridged Matrix Rooms**")
	roomLines = append(roomLines, "")

	// In mirror mode, rooms are created for each synced channel
	h.Connector.usersLock.RLock()
	userCount := len(h.Connector.users)
	h.Connector.usersLock.RUnlock()

	if userCount == 0 {
		roomLines = append(roomLines, "_No rooms are currently bridged._")
	} else {
		// For each portal the bridge knows about, list it
		// This is simplified - a full implementation would query the database
		roomLines = append(roomLines, "The bridge is active with "+fmt.Sprintf("%d", userCount)+" logged-in user(s).")
		roomLines = append(roomLines, "")
		roomLines = append(roomLines, "Bridged channels appear in your Mattermost sidebar with Matrix counterparts.")
		roomLines = append(roomLines, "")
		roomLines = append(roomLines, "_Use `/matrix join <room>` to bridge additional Matrix rooms._")
	}

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text:         strings.Join(roomLines, "\n"),
	}
}

// accountResponse returns the user's Matrix account credentials.
func (h *SlashCommandHandler) accountResponse(ctx context.Context, userID, userName string) *SlashCommandResponse {
	// Get the homeserver domain from the bridge config
	domain := h.Connector.Bridge.Matrix.ServerName()

	// Generate the Matrix user ID for this Mattermost user
	matrixUserID := id.NewUserID(userName, string(domain))

	// Check if Synapse Admin API is configured
	if h.Connector.Config.SynapseAdmin.URL == "" || h.Connector.Config.SynapseAdmin.Token == "" {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text: fmt.Sprintf("**Your Matrix Account**\n\n"+
				"• **Matrix ID**: `%s`\n"+
				"• **Homeserver**: `%s`\n\n"+
				"_Note: Synapse Admin API is not configured. Contact your administrator for login credentials._",
				matrixUserID, domain),
		}
	}

	// Create Synapse Admin client
	admin := NewMatrixAdminClient(h.Connector.Config.SynapseAdmin.URL, h.Connector.Config.SynapseAdmin.Token)

	// Check if user exists
	exists, err := admin.UserExists(ctx, matrixUserID)
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to check Matrix account status: %v", err),
		}
	}

	if exists {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text: fmt.Sprintf("**Your Matrix Account**\n\n"+
				"• **Matrix ID**: `%s`\n"+
				"• **Homeserver**: `%s`\n"+
				"• **Status**: ✅ Account exists\n\n"+
				"You can log in to any Matrix client (e.g., Element) using your Matrix ID.\n\n"+
				"_If you need to reset your password, contact your administrator._",
				matrixUserID, domain),
		}
	}

	// Account doesn't exist - create it
	password := GeneratePassword()

	// Get the user's display name from Mattermost if possible
	displayName := userName
	if h.Connector.Client != nil {
		mmUser, _, err := h.Connector.Client.GetUser(ctx, userID, "")
		if err == nil && mmUser != nil {
			if mmUser.FirstName != "" || mmUser.LastName != "" {
				displayName = strings.TrimSpace(mmUser.FirstName + " " + mmUser.LastName)
			} else if mmUser.Nickname != "" {
				displayName = mmUser.Nickname
			}
		}
	}

	err = admin.CreateUser(ctx, matrixUserID, password, displayName)
	if err != nil {
		return &SlashCommandResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Failed to create Matrix account: %v", err),
		}
	}

	return &SlashCommandResponse{
		ResponseType: "ephemeral",
		Text: fmt.Sprintf("✅ **Matrix Account Created!**\n\n"+
			"• **Matrix ID**: `%s`\n"+
			"• **Homeserver**: `%s`\n"+
			"• **Password**: `%s`\n\n"+
			"⚠️ **Save this password!** It will not be shown again.\n\n"+
			"You can log in to any Matrix client (e.g., Element Web, Element Desktop, FluffyChat) using these credentials.",
			matrixUserID, domain, password),
	}
}

// getOrProvisionGhost resolves a Matrix User ID to a Mattermost User ID.
// If the user doesn't exist on Mattermost, it creates it.
func (h *SlashCommandHandler) getOrProvisionGhost(ctx context.Context, mxid string) (string, error) {
	// Delegate to the shared helper in Connector
	// This ensures consistent username encoding and provisioning logic
	userid, err := h.Connector.EnsureGhost(ctx, mxid)
	if err != nil {
		return "", err
	}

	// EnsureGhost returns the Mattermost User ID (UUID).
	// We return this directly so callers can use it for ID-based API calls.
	return userid, nil
}
