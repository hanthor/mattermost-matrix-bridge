package mattermost

import (
	"context"
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

// SyncEngine handles full server synchronization in mirror mode
type SyncEngine struct {
	Connector *MattermostConnector
	// Track synced entities to avoid duplicates
	syncedTeams    map[string]bool
	syncedChannels map[string]bool
	syncedUsers    map[string]bool
}

// NewSyncEngine creates a new sync engine for mirror mode
func NewSyncEngine(connector *MattermostConnector) *SyncEngine {
	return &SyncEngine{
		Connector:      connector,
		syncedTeams:    make(map[string]bool),
		syncedChannels: make(map[string]bool),
		syncedUsers:    make(map[string]bool),
	}
}

// startMirrorSync is called at startup in mirror mode to sync all teams/channels/users
func (m *MattermostConnector) startMirrorSync(ctx context.Context) {
	// Wait for bridge to be fully ready
	time.Sleep(5 * time.Second)
	
	engine := NewSyncEngine(m)
	
	if err := engine.SyncAll(ctx); err != nil {
		fmt.Printf("ERROR: Mirror sync failed: %v\n", err)
	}
}

// SyncAll performs a full synchronization of the Mattermost server to Matrix
func (s *SyncEngine) SyncAll(ctx context.Context) error {
	fmt.Printf("INFO: Starting full server sync...\n")
	
	// First sync users so ghosts exist for channel members
	if s.Connector.Config.Mirror.SyncAllUsers {
		if err := s.SyncUsers(ctx); err != nil {
			fmt.Printf("WARN: Failed to sync users: %v\n", err)
			// Continue anyway - ghosts will be created on demand
		}
	}
	
	// Sync teams (which creates spaces) and their channels
	if s.Connector.Config.Mirror.SyncAllTeams {
		if err := s.SyncTeams(ctx); err != nil {
			return fmt.Errorf("failed to sync teams: %w", err)
		}
	}
	
	fmt.Printf("INFO: Full server sync complete\n")
	return nil
}

// SyncTeams synchronizes all Mattermost teams to Matrix Spaces
func (s *SyncEngine) SyncTeams(ctx context.Context) error {
	fmt.Printf("INFO: Syncing teams...\n")
	
	// Get all teams from Mattermost
	teams, _, err := s.Connector.Client.GetAllTeams(ctx, "", 0, 100)
	if err != nil {
		return fmt.Errorf("failed to get teams: %w", err)
	}
	
	fmt.Printf("INFO: Found %d teams to sync\n", len(teams))
	
	for _, team := range teams {
		if err := s.SyncTeam(ctx, team); err != nil {
			fmt.Printf("WARN: Failed to sync team %s: %v\n", team.Name, err)
			continue
		}
	}
	
	return nil
}

// SyncTeam synchronizes a single team and its channels
func (s *SyncEngine) SyncTeam(ctx context.Context, team *model.Team) error {
	if s.syncedTeams[team.Id] {
		return nil // Already synced
	}
	
	fmt.Printf("INFO: Syncing team: %s (%s)\n", team.DisplayName, team.Id)
	
	// Create portal for team (as Space)
	portalKey := networkid.PortalKey{
		ID: networkid.PortalID(team.Id),
	}
	
	// Get the first available login to use for portal creation
	login := s.getAnyLogin()
	if login == nil {
		return fmt.Errorf("no logged-in user available for portal creation")
	}
	
	// Get or create the portal
	portal, err := s.Connector.Bridge.GetPortalByKey(ctx, portalKey)
	if err != nil {
		return fmt.Errorf("failed to get portal for team: %w", err)
	}
	
	if portal.MXID == "" {
		// Portal doesn't exist in Matrix yet - create it
		fmt.Printf("INFO: Creating Matrix Space for team: %s\n", team.DisplayName)
		
		// Create a synthetic event to trigger room creation
		evt := &TeamSyncEvent{
			MattermostEvent: MattermostEvent{
				Connector: s.Connector,
				Timestamp: time.Now(),
				ChannelID: team.Id,
				UserID:    string(login.ID),
			},
			Team: team,
		}
		
		// Queue the event to create the portal
		s.Connector.Bridge.QueueRemoteEvent(login, evt)
	}
	
	s.syncedTeams[team.Id] = true
	
	// Sync channels in this team
	if s.Connector.Config.Mirror.SyncAllChannels {
		if err := s.SyncChannels(ctx, team.Id); err != nil {
			fmt.Printf("WARN: Failed to sync channels for team %s: %v\n", team.Name, err)
		}
	}
	
	return nil
}

// SyncChannels synchronizes all channels in a team
func (s *SyncEngine) SyncChannels(ctx context.Context, teamID string) error {
	fmt.Printf("INFO: Syncing channels for team %s...\n", teamID)
	
	// Get public channels
	publicChannels, _, err := s.Connector.Client.GetPublicChannelsForTeam(ctx, teamID, 0, 200, "")
	if err != nil {
		return fmt.Errorf("failed to get public channels: %w", err)
	}
	
	// Get private channels (requires admin)
	privateChannels, _, err := s.Connector.Client.GetPrivateChannelsForTeam(ctx, teamID, 0, 200, "")
	if err != nil {
		fmt.Printf("WARN: Failed to get private channels (may need admin): %v\n", err)
		privateChannels = []*model.Channel{}
	}
	
	allChannels := append(publicChannels, privateChannels...)
	fmt.Printf("INFO: Found %d channels to sync in team %s\n", len(allChannels), teamID)
	
	for _, channel := range allChannels {
		if err := s.SyncChannel(ctx, channel); err != nil {
			fmt.Printf("WARN: Failed to sync channel %s: %v\n", channel.Name, err)
			continue
		}
	}
	
	return nil
}

// SyncChannel synchronizes a single channel
func (s *SyncEngine) SyncChannel(ctx context.Context, channel *model.Channel) error {
	if s.syncedChannels[channel.Id] {
		return nil // Already synced
	}
	
	// Skip DM and Group DM channels in team sync
	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return nil
	}
	
	fmt.Printf("INFO: Syncing channel: %s (%s)\n", channel.DisplayName, channel.Id)
	
	// Create portal for channel
	portalKey := networkid.PortalKey{
		ID: networkid.PortalID(channel.Id),
	}
	
	login := s.getAnyLogin()
	if login == nil {
		return fmt.Errorf("no logged-in user available for portal creation")
	}
	
	// Get or create the portal
	portal, err := s.Connector.Bridge.GetPortalByKey(ctx, portalKey)
	if err != nil {
		return fmt.Errorf("failed to get portal for channel: %w", err)
	}
	
	if portal.MXID == "" {
		fmt.Printf("INFO: Creating Matrix room for channel: %s\n", channel.DisplayName)
		
		// Create a synthetic event to trigger room creation
		evt := &ChannelSyncEvent{
			MattermostEvent: MattermostEvent{
				Connector: s.Connector,
				Timestamp: time.Now(),
				ChannelID: channel.Id,
				UserID:    string(login.ID),
			},
			Channel: channel,
		}
		
		s.Connector.Bridge.QueueRemoteEvent(login, evt)
	}
	
	// Auto-invite users if configured
	if s.Connector.Config.Mirror.AutoInviteUsers && portal.MXID != "" {
		if err := s.inviteChannelMembers(ctx, channel.Id, portal); err != nil {
			fmt.Printf("WARN: Failed to invite members to channel %s: %v\n", channel.Name, err)
		}
	}
	
	s.syncedChannels[channel.Id] = true
	return nil
}

// SyncUsers synchronizes all Mattermost users to Matrix ghosts
func (s *SyncEngine) SyncUsers(ctx context.Context) error {
	fmt.Printf("INFO: Syncing users...\\n")
	
	page := 0
	perPage := 200
	totalUsers := 0
	createdMatrixUsers := 0
	
	// Create Matrix Admin client if needed
	var matrixAdmin *MatrixAdminClient
	if s.Connector.Config.Mirror.CreateMatrixAccounts && s.Connector.Config.SynapseAdmin.Token != "" {
		matrixAdmin = NewMatrixAdminClient(
			s.Connector.Config.SynapseAdmin.URL,
			s.Connector.Config.SynapseAdmin.Token,
		)
	}
	
	for {
		users, _, err := s.Connector.Client.GetUsers(ctx, page, perPage, "")
		if err != nil {
			return fmt.Errorf("failed to get users page %d: %w", page, err)
		}
		
		if len(users) == 0 {
			break
		}
		
		for _, user := range users {
			if s.syncedUsers[user.Id] {
				continue
			}
			
			// Ensure ghost exists for this user
			ghostID := networkid.UserID(user.Id)
			_, err := s.Connector.Bridge.GetGhostByID(ctx, ghostID)
			if err != nil {
				fmt.Printf("WARN: Failed to get/create ghost for user %s: %v\\n", user.Username, err)
				continue
			}
			
			// Optionally create a real Matrix account for the user
			if matrixAdmin != nil {
				if created := s.CreateMatrixUserIfNeeded(ctx, matrixAdmin, user); created {
					createdMatrixUsers++
				}
			}
			
			s.syncedUsers[user.Id] = true
			totalUsers++
		}
		
		page++
		if len(users) < perPage {
			break
		}
	}
	
	fmt.Printf("INFO: Synced %d users, created %d Matrix accounts\\n", totalUsers, createdMatrixUsers)
	return nil
}

// CreateMatrixUserIfNeeded creates a Matrix account for a Mattermost user if it doesn't exist
func (s *SyncEngine) CreateMatrixUserIfNeeded(ctx context.Context, admin *MatrixAdminClient, mmUser *model.User) bool {
	serverName := s.Connector.Bridge.Matrix.ServerName()
	mxid := GenerateMatrixUserID(mmUser, serverName)
	
	// Check if user already exists
	exists, err := admin.UserExists(ctx, mxid)
	if err != nil {
		fmt.Printf("WARN: Failed to check if Matrix user exists for %s: %v\\n", mmUser.Username, err)
		return false
	}
	
	if exists {
		// User exists, just update display name if needed
		displayName := mmUser.GetDisplayName(model.ShowFullName)
		if displayName == "" {
			displayName = mmUser.Username
		}
		_ = admin.UpdateUserDisplayName(ctx, mxid, displayName)
		return false
	}
	
	// Create the user
	displayName := mmUser.GetDisplayName(model.ShowFullName)
	if displayName == "" {
		displayName = mmUser.Username
	}
	password := GeneratePassword()
	
	if err := admin.CreateUser(ctx, mxid, password, displayName); err != nil {
		fmt.Printf("WARN: Failed to create Matrix user for %s: %v\\n", mmUser.Username, err)
		return false
	}
	
	fmt.Printf("INFO: Created Matrix user %s for Mattermost user %s\\n", mxid, mmUser.Username)
	return true
}

// SyncHistoricalMessages syncs message history for a channel
func (s *SyncEngine) SyncHistoricalMessages(ctx context.Context, channelID string, limit int) error {
	fmt.Printf("INFO: Syncing history for channel %s (limit: %d)...\n", channelID, limit)
	
	if limit == 0 {
		limit = s.Connector.Config.Mirror.HistoryLimit
	}
	if limit == 0 {
		limit = 100 // Default
	}
	
	login := s.getAnyLogin()
	if login == nil {
		return fmt.Errorf("no logged-in user available for backfill")
	}
	
	// Get posts for channel
	postList, _, err := s.Connector.Client.GetPostsForChannel(ctx, channelID, 0, limit, "", false, false)
	if err != nil {
		return fmt.Errorf("failed to get posts: %w", err)
	}
	
	fmt.Printf("INFO: Found %d posts to backfill\n", len(postList.Posts))
	
	// Posts need to be processed in order (oldest first)
	// postList.Order is newest first, so reverse it
	syncedCount := 0
	for i := len(postList.Order) - 1; i >= 0; i-- {
		postID := postList.Order[i]
		post := postList.Posts[postID]
		
		// Skip system messages
		if post.Type != "" && post.Type != "custom_post" {
			continue
		}
		
		// Create event for this historical message
		evt := &MattermostMessageEvent{
			MattermostEvent: MattermostEvent{
				Connector: s.Connector,
				Timestamp: time.Unix(post.CreateAt/1000, (post.CreateAt%1000)*1000000),
				ChannelID: post.ChannelId,
				UserID:    post.UserId,
			},
			PostID:  post.Id,
			Content: post.Message,
			FileIds: post.FileIds,
			RootID:  post.RootId,
		}
		
		// Queue the event for processing
		s.Connector.Bridge.QueueRemoteEvent(login, evt)
		syncedCount++
	}
	
	fmt.Printf("INFO: Queued %d historical messages for channel %s\n", syncedCount, channelID)
	return nil
}

// BackfillChannel performs a complete backfill of a channel including messages and members
func (s *SyncEngine) BackfillChannel(ctx context.Context, channelID string) error {
	fmt.Printf("INFO: Starting full backfill for channel %s\n", channelID)
	
	// Get portal for channel
	portalKey := networkid.PortalKey{
		ID: networkid.PortalID(channelID),
	}
	
	portal, err := s.Connector.Bridge.GetPortalByKey(ctx, portalKey)
	if err != nil {
		return fmt.Errorf("failed to get portal: %w", err)
	}
	
	// Sync channel memberships first
	if err := s.SyncChannelMemberships(ctx, channelID, portal); err != nil {
		fmt.Printf("WARN: Failed to sync memberships for channel %s: %v\n", channelID, err)
	}
	
	// Then backfill historical messages
	if s.Connector.Config.Mirror.SyncHistory {
		if err := s.SyncHistoricalMessages(ctx, channelID, 0); err != nil {
			fmt.Printf("WARN: Failed to backfill messages for channel %s: %v\n", channelID, err)
		}
	}
	
	return nil
}

// SyncChannelMemberships syncs all channel members to the Matrix room
func (s *SyncEngine) SyncChannelMemberships(ctx context.Context, channelID string, portal *bridgev2.Portal) error {
	members, _, err := s.Connector.Client.GetChannelMembers(ctx, channelID, 0, 1000, "")
	if err != nil {
		return fmt.Errorf("failed to get channel members: %w", err)
	}
	
	fmt.Printf("INFO: Syncing %d members for channel %s\n", len(members), channelID)
	
	// Create Matrix Admin client if available for direct room joins
	var matrixAdmin *MatrixAdminClient
	if s.Connector.Config.SynapseAdmin.Token != "" {
		matrixAdmin = NewMatrixAdminClient(
			s.Connector.Config.SynapseAdmin.URL,
			s.Connector.Config.SynapseAdmin.Token,
		)
	}
	
	serverName := s.Connector.Bridge.Matrix.ServerName()
	joinedCount := 0
	
	for _, member := range members {
		// Get Mattermost user info
		user, _, err := s.Connector.Client.GetUser(ctx, member.UserId, "")
		if err != nil {
			fmt.Printf("WARN: Failed to get user %s: %v\n", member.UserId, err)
			continue
		}
		
		// Ensure ghost exists
		ghostID := networkid.UserID(user.Id)
		_, err = s.Connector.Bridge.GetGhostByID(ctx, ghostID)
		if err != nil {
			fmt.Printf("WARN: Failed to get ghost for user %s: %v\n", user.Username, err)
			continue
		}
		
		// If we have Matrix admin access and create_matrix_accounts is enabled,
		// join the real Matrix user to the room
		if matrixAdmin != nil && s.Connector.Config.Mirror.CreateMatrixAccounts {
			mxid := GenerateMatrixUserID(user, serverName)
			if err := matrixAdmin.JoinUserToRoom(ctx, mxid, portal.MXID); err != nil {
				fmt.Printf("DEBUG: Could not join %s to %s: %v\n", mxid, portal.MXID, err)
			} else {
				joinedCount++
			}
		}
	}
	
	fmt.Printf("INFO: Joined %d Matrix users to room %s\n", joinedCount, portal.MXID)
	return nil
}

// BackfillAllChannels backfills all synced channels
func (s *SyncEngine) BackfillAllChannels(ctx context.Context) error {
	fmt.Printf("INFO: Starting backfill for all synced channels...\n")
	
	backfilledCount := 0
	for channelID := range s.syncedChannels {
		if err := s.BackfillChannel(ctx, channelID); err != nil {
			fmt.Printf("WARN: Failed to backfill channel %s: %v\n", channelID, err)
			continue
		}
		backfilledCount++
	}
	
	fmt.Printf("INFO: Backfilled %d channels\n", backfilledCount)
	return nil
}

// getAnyLogin returns any available logged-in user
func (s *SyncEngine) getAnyLogin() *bridgev2.UserLogin {
	s.Connector.usersLock.RLock()
	defer s.Connector.usersLock.RUnlock()
	
	for _, login := range s.Connector.users {
		return login
	}
	return nil
}

// inviteChannelMembers invites all channel members to the Matrix room
func (s *SyncEngine) inviteChannelMembers(ctx context.Context, channelID string, portal *bridgev2.Portal) error {
	members, _, err := s.Connector.Client.GetChannelMembers(ctx, channelID, 0, 200, "")
	if err != nil {
		return fmt.Errorf("failed to get channel members: %w", err)
	}
	
	fmt.Printf("INFO: Would invite %d members to portal %s\n", len(members), portal.MXID)
	
	// Phase 7 will implement actual Matrix user invitation via Synapse Admin API
	// For now, just log what would happen
	for _, member := range members {
		fmt.Printf("DEBUG: Would invite user %s to room\n", member.UserId)
	}
	
	return nil
}

// TeamSyncEvent is a synthetic event for creating team spaces
type TeamSyncEvent struct {
	MattermostEvent
	Team *model.Team
}

func (e *TeamSyncEvent) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventChatInfoChange
}

func (e *TeamSyncEvent) GetChatInfoChange(ctx context.Context) (*bridgev2.ChatInfoChange, error) {
	return &bridgev2.ChatInfoChange{
		ChatInfo: &bridgev2.ChatInfo{
			Name:  &e.Team.DisplayName,
			Topic: &e.Team.Description,
		},
	}, nil
}

// ChannelSyncEvent is a synthetic event for creating channel rooms
type ChannelSyncEvent struct {
	MattermostEvent
	Channel *model.Channel
}

func (e *ChannelSyncEvent) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventChatInfoChange
}

func (e *ChannelSyncEvent) GetChatInfoChange(ctx context.Context) (*bridgev2.ChatInfoChange, error) {
	return &bridgev2.ChatInfoChange{
		ChatInfo: &bridgev2.ChatInfo{
			Name:  &e.Channel.DisplayName,
			Topic: &e.Channel.Purpose,
		},
	}, nil
}
