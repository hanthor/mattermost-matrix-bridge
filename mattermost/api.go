package mattermost

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"github.com/mattermost/mattermost/server/public/model"
)

type MattermostAPI struct {
	Login     *bridgev2.UserLogin
	Connector *MattermostConnector
	Client    *Client
}

func (m *MattermostAPI) getMMID(ctx context.Context, ghostID networkid.UserID) string {
	// Try metadata first
	ghost, err := m.Connector.Bridge.GetGhostByID(ctx, ghostID)
	if err == nil && ghost != nil && ghost.Metadata != nil {
		if meta, ok := ghost.Metadata.(map[string]any); ok {
			if id, _ := meta["mm_id"].(string); id != "" {
				return id
			}
		}
	}

	// Fallback to EnsureGhost which guarantees a UUID or error
	mmID, err := m.Connector.EnsureGhost(ctx, string(ghostID))
	if err != nil {
		m.Connector.Bridge.Log.Warn().Err(err).Str("ghost_id", string(ghostID)).Msg("Failed to resolve MM ID for ghost")
		return ""
	}
	return mmID
}

func (m *MattermostAPI) getOwnMMID() string {
	if m.Login == nil || m.Login.Metadata == nil {
		return ""
	}
	meta, ok := m.Login.Metadata.(map[string]any)
	if !ok {
		return string(m.Login.ID)
	}
	id, _ := meta["mm_id"].(string)
	if id == "" {
		return string(m.Login.ID)
	}
	return id
}

func (m *MattermostAPI) GetClient() *model.Client4 {
	return m.Client.GetClient()
}

func (m *MattermostAPI) GetFile(ctx context.Context, fileID string) ([]byte, error) {
	return m.Client.GetFile(ctx, fileID)
}

func (m *MattermostAPI) UploadFile(ctx context.Context, data []byte, channelID, filename string) (*model.FileInfo, error) {
	return m.Client.UploadFile(ctx, data, channelID, filename)
}

func (m *MattermostAPI) Connect(ctx context.Context) error {
	if m.Login == nil {
		return nil
	}

	// Fetch our own user details to resolve UUID
	user, _, err := m.Client.GetMe(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get own user details: %w", err)
	}

	m.Connector.Bridge.Log.Info().Str("username", user.Username).Str("user_id", user.Id).Msg("Connected to Mattermost as")

	// Update metadata if needed
	if m.Login.Metadata == nil {
		m.Login.Metadata = make(map[string]any)
	}

	meta, ok := m.Login.Metadata.(map[string]any)
	if !ok {
		// Should be a map, force reset if not
		meta = make(map[string]any)
	}

	if meta["mm_id"] != user.Id {
		meta["mm_id"] = user.Id
		m.Login.Metadata = meta
		err = m.Login.Save(ctx)
		if err != nil {
			m.Connector.Bridge.Log.Warn().Err(err).Msg("Failed to save login metadata")
		}
	}

	return nil
}

func (m *MattermostAPI) Disconnect() {
}

func (m *MattermostAPI) IsConnected() bool {
	return m.Client != nil
}

func (m *MattermostAPI) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *bridgev2.NetworkRoomCapabilities {
	return &bridgev2.NetworkRoomCapabilities{
		FormattedText: true,
	}
}

func (m *MattermostAPI) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	// Try as channel first
	channel, _, err := m.Client.GetChannel(ctx, string(portal.ID), "")
	if err == nil {
		ci := &bridgev2.ChatInfo{
			Name:    &channel.DisplayName,
			Topic:   &channel.Purpose,
			Members: &bridgev2.ChatMemberList{},
		}

		if channel.Type == model.ChannelTypeOpen {
			ci.Type = ptr.Ptr(database.RoomTypeDefault)
			ci.ParentID = ptr.Ptr(networkid.PortalID(channel.TeamId))
		} else if channel.Type == model.ChannelTypePrivate {
			ci.Type = ptr.Ptr(database.RoomTypeDefault) // Or RoomTypePrivate if bridge supports it specifically? Usually Default is fine.
			ci.ParentID = ptr.Ptr(networkid.PortalID(channel.TeamId))
		} else if channel.Type == model.ChannelTypeDirect {
			ci.Type = ptr.Ptr(database.RoomTypeDM)
			// For DMs, name is often empty or just usernames.
			// We might want to clear name so bridge generates it from members.
			ci.Name = nil
			// We need to fetch members for DMs to work properly
			members, _, err := m.Client.GetChannelMembers(ctx, channel.Id, 0, 10, "")
			if err == nil {
				ci.Members.IsFull = true
				ci.Members.Members = make([]bridgev2.ChatMember, 0, len(members))
				for _, member := range members {
					if m.isGhost(ctx, member.UserId) {
						continue
					}
					ci.Members.Members = append(ci.Members.Members, bridgev2.ChatMember{
						EventSender: bridgev2.EventSender{Sender: networkid.UserID(m.Connector.GetUsername(ctx, member.UserId))},
					})
				}
			}
		} else if channel.Type == model.ChannelTypeGroup {
			ci.Type = ptr.Ptr(database.RoomTypeGroupDM)
			ci.Name = nil // Let bridge generate
			// Fetch members similar to DM
			members, _, err := m.Client.GetChannelMembers(ctx, channel.Id, 0, 10, "")
			if err == nil {
				ci.Members.IsFull = true
				ci.Members.Members = make([]bridgev2.ChatMember, len(members))
				for i, member := range members {
					ci.Members.Members[i] = bridgev2.ChatMember{
						EventSender: bridgev2.EventSender{Sender: networkid.UserID(m.Connector.GetUsername(ctx, member.UserId))},
					}
				}
			}
		}

		return ci, nil
	}

	// If not channel, try Team
	team, err := m.Client.GetTeam(ctx, string(portal.ID))
	if err == nil {
		return &bridgev2.ChatInfo{
			Name:  &team.DisplayName,
			Topic: &team.Description,
			Type:  ptr.Ptr(database.RoomTypeSpace),
		}, nil
	}

	return nil, fmt.Errorf("item not found (tried channel and team)")
}

func (m *MattermostAPI) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	user, _, err := m.Client.GetUser(ctx, m.getMMID(ctx, ghost.ID), "")
	if err != nil {
		return nil, err
	}
	name := user.Username
	var parts []string
	if user.FirstName != "" && user.FirstName != "()" {
		parts = append(parts, strings.TrimSpace(user.FirstName))
	}
	if user.LastName != "" && user.LastName != "()" {
		parts = append(parts, strings.TrimSpace(user.LastName))
	}
	fullName := strings.Join(parts, " ")

	if fullName != "" {
		name = fullName
	} else if user.Nickname != "" {
		name = user.Nickname
	}

	m.Connector.Bridge.Log.Debug().
		Str("username", user.Username).
		Str("first_name", user.FirstName).
		Str("last_name", user.LastName).
		Str("nickname", user.Nickname).
		Str("calc_fullname", fullName).
		Str("final_name", name).
		Int64("last_picture_update", user.LastPictureUpdate).
		Msg("GetUserInfo name components")

	return &bridgev2.UserInfo{
		Name: &name,
		Avatar: &bridgev2.Avatar{
			ID: networkid.AvatarID(fmt.Sprintf("%d-force3", user.LastPictureUpdate)),
			Get: func(ctx context.Context) ([]byte, error) {
				data, _, err := m.Client.GetProfileImage(ctx, user.Id, "")
				return data, err
			},
		},
	}, nil
}

func (m *MattermostAPI) IsLoggedIn() bool {
	return m.Login != nil
}

func (m *MattermostAPI) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	if m.Login == nil {
		return false
	}
	return string(userID) == string(m.Login.ID)
}

func (m *MattermostAPI) isGhost(ctx context.Context, userID string) bool {
	user, _, err := m.Client.GetUser(ctx, userID, "")
	if err != nil {
		return false
	}
	return strings.HasPrefix(user.Username, "mx.")
}

func (m *MattermostAPI) LogoutRemote(ctx context.Context) {}

func (m *MattermostAPI) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (*bridgev2.MatrixMessageResponse, error) {
	post, err := m.Connector.MsgConv.ToMattermost(ctx, m.Client, msg.Portal, msg.Content)
	if err != nil {
		return nil, err
	}

	post.ChannelId = string(msg.Portal.ID)
	// post.Message is already set by ToMattermost

	// Workaround: Strip "User: " prefix if added by bridge core (relay mode artifact)
	// Matches "**User**: message"
	if strings.HasPrefix(post.Message, "**") {
		parts := strings.SplitN(post.Message, ": ", 2)
		if len(parts) == 2 && strings.HasSuffix(parts[0], "**") {
			post.Message = parts[1]
		}
	}

	// post.FileIds is already set by ToMattermost

	// Handle thread replies: if there's a thread root, set RootId
	if msg.ThreadRoot != nil {
		post.RootId = string(msg.ThreadRoot.ID)
	}

	// Get the sender's Matrix user ID
	senderMXID := msg.Event.Sender

	// Get authenticated client for the ghost user and their MM ID
	userClient, mmUserID, err := m.Connector.GetClientForUser(ctx, senderMXID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get client for ghost: %w", err)
	}

	// Update ghost profile if needed (avatar/name)
	ghost, err := m.Connector.Bridge.GetGhostByID(ctx, networkid.UserID(senderMXID.String()))
	if err == nil {
		m.Connector.Bridge.Log.Info().Str("mxid", senderMXID.String()).Msg("Calling UpdateGhost from HandleMatrixMessage")
		err = m.UpdateGhost(ctx, ghost)
		if err != nil {
			m.Connector.Bridge.Log.Warn().Err(err).Msg("Failed to update ghost profile")
		}
	} else {
		m.Connector.Bridge.Log.Warn().Err(err).Str("mxid", senderMXID.String()).Msg("Failed to get ghost for profile update")
	}

	m.Connector.Bridge.Log.Info().Str("matrix_user", senderMXID.String()).Str("mm_user_id", mmUserID).Msg("Ghost Puppeting with Token")

	// Set the post's UserId (though the token implies it)
	post.UserId = mmUserID

	// Mark the post as coming from Matrix to prevent loops
	if post.Props == nil {
		post.Props = make(map[string]any)
	}
	post.Props["from_matrix"] = true

	// Ensure ghost is a member of the team and channel before posting
	// This is needed for joined Matrix rooms where ghosts may not be members yet
	channel, _, err := m.Client.GetChannel(ctx, post.ChannelId, "")
	if err == nil && channel.TeamId != "" {
		_, _, err = m.Client.AddTeamMember(ctx, channel.TeamId, mmUserID)
		if err != nil {
			// Log but don't fail - they might already be a member
			m.Connector.Bridge.Log.Debug().Err(err).Str("team", channel.TeamId).Str("user", mmUserID).Msg("Could not add ghost to team (may already be member)")
		}
	}

	_, _, err = m.Client.AddChannelMember(ctx, post.ChannelId, mmUserID)
	if err != nil {
		// Log but don't fail - they might already be a member
		m.Connector.Bridge.Log.Debug().Err(err).Str("channel", post.ChannelId).Str("user", mmUserID).Msg("Could not add ghost to channel (may already be member)")
	}

	// Use the USER'S client to create the post
	createdPost, _, err := userClient.CreatePost(ctx, post)
	if err != nil {
		return nil, err
	}

	return &bridgev2.MatrixMessageResponse{
		DB: &database.Message{
			ID: networkid.MessageID(createdPost.Id),
		},
	}, nil
}

func (m *MattermostAPI) ResolveIdentifier(ctx context.Context, identifier string, createChat bool) (*bridgev2.ResolveIdentifierResponse, error) {
	var user *model.User
	var err error

	// Check if identifier is an email
	if strings.Contains(identifier, "@") {
		user, err = m.Client.GetUserByEmail(ctx, identifier)
	} else {
		user, err = m.Client.GetUserByUsername(ctx, identifier)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find user by identifier %s: %w", identifier, err)
	}

	// Ghost ID is now the Username for readability
	ghostID := networkid.UserID(user.Username)
	ghost, err := m.Connector.Bridge.GetGhostByID(ctx, ghostID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ghost: %w", err)
	}
	// Ensure UUID is in metadata
	if ghost.Metadata == nil {
		ghost.Metadata = make(map[string]any)
	}
	meta, ok := ghost.Metadata.(map[string]any)
	if ok && meta["mm_id"] != user.Id {
		meta["mm_id"] = user.Id
		err = m.Connector.Bridge.DB.Ghost.Update(ctx, ghost.Ghost)
		if err != nil {
			fmt.Printf("DEBUG: Failed to update ghost metadata: %v\n", err)
		}
	}

	var chatResp *bridgev2.CreateChatResponse
	if createChat {
		chatResp, err = m.CreateChatWithGhost(ctx, ghost)
		if err != nil {
			return nil, err
		}
	}

	// Create minimal UserInfo for response if needed,
	// typically bridgev2 handles updating ghost info if we return it?
	// But ResolveIdentifierResponse has UserInfo field.
	// We can use GetUserInfo to fill it.
	userInfo, err := m.GetUserInfo(ctx, ghost)
	if err != nil {
		// Log error but proceed?
	}

	return &bridgev2.ResolveIdentifierResponse{
		Ghost:    ghost,
		UserID:   ghostID,
		Chat:     chatResp,
		UserInfo: userInfo,
	}, nil
}

func (m *MattermostAPI) CreateChatWithGhost(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.CreateChatResponse, error) {
	// We need our own UserID.
	if m.Login == nil {
		return nil, bridgev2.ErrNotLoggedIn
	}
	myUserID := m.getOwnMMID()

	// Resolve otherUserID from the passed ghost object directly if possible
	var otherUserID string
	if ghost.Metadata != nil {
		if meta, ok := ghost.Metadata.(map[string]any); ok {
			if id, ok := meta["mm_id"].(string); ok && id != "" {
				otherUserID = id
			}
		}
	}

	if otherUserID == "" {
		otherUserID = m.getMMID(ctx, ghost.ID)
	}

	fmt.Printf("DEBUG: CreateDirectChannelWithBoth myID=%s otherID=%s ghostID=%s\n", myUserID, otherUserID, ghost.ID)

	// Ensure we don't pass the Matrix ID (username) if we failed to resolve UUID
	if otherUserID == string(ghost.ID) && strings.Contains(otherUserID, "matrix_") {
		// Try to resolve username to ID one last time
		user, err := m.Client.GetUserByUsername(ctx, otherUserID)
		if err == nil && user != nil {
			otherUserID = user.Id
			fmt.Printf("DEBUG: Resolved username %s to UUID %s\n", string(ghost.ID), otherUserID)
		}
	}

	channel, err := m.Client.CreateDirectChannelWithBoth(ctx, myUserID, otherUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to create direct channel: %w", err)
	}

	// Wrap into PortalInfo
	portalID := networkid.PortalID(channel.Id)

	ci := &bridgev2.ChatInfo{
		Name: nil,
		Type: ptr.Ptr(database.RoomTypeDM),
		Members: &bridgev2.ChatMemberList{
			IsFull: true,
			Members: []bridgev2.ChatMember{
				{EventSender: bridgev2.EventSender{Sender: networkid.UserID(myUserID)}},
			},
		},
	}

	// Only add other user if they are NOT a ghost (i.e. not a Matrix user)
	if !m.isGhost(ctx, otherUserID) {
		ci.Members.Members = append(ci.Members.Members, bridgev2.ChatMember{
			EventSender: bridgev2.EventSender{Sender: networkid.UserID(otherUserID)},
		})
	}

	return &bridgev2.CreateChatResponse{
		PortalKey: networkid.PortalKey{
			ID:       portalID,
			Receiver: "",
		},
		PortalInfo: ci,
	}, nil
}

// HandleMatrixEdit handles edit events from Matrix, updating the corresponding Mattermost post
func (m *MattermostAPI) HandleMatrixEdit(ctx context.Context, edit *bridgev2.MatrixEdit) error {
	if edit.EditTarget == nil {
		return fmt.Errorf("no edit target")
	}

	// Get the post ID from the edit target
	postID := string(edit.EditTarget.ID)

	// Fetch the existing post to update it
	existingPost, _, err := m.Client.GetPost(ctx, postID, "")
	if err != nil {
		return fmt.Errorf("failed to get post for edit: %w", err)
	}

	// Convert the new content
	newPost, err := m.Connector.MsgConv.ToMattermost(ctx, m.Client, edit.Portal, edit.Content)
	if err != nil {
		return fmt.Errorf("failed to convert edit content: %w", err)
	}

	// Ensure the post has the correct UserId (for ghost puppeting)
	// Get the sender's Matrix user ID
	senderMXID := edit.Event.Sender
	ghost, err := m.Connector.Bridge.GetGhostByID(ctx, networkid.UserID(senderMXID.String()))
	if err != nil {
		return fmt.Errorf("failed to get ghost for sender: %w", err)
	}
	mmUserID := m.getMMID(ctx, ghost.ID)
	existingPost.UserId = mmUserID

	// Update the post message
	existingPost.Message = newPost.Message

	// Update the post in Mattermost
	_, _, err = m.Client.UpdatePost(ctx, postID, existingPost)
	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	return nil
}

// HandleMatrixMessageRemove handles redaction events from Matrix, deleting the corresponding Mattermost post
func (m *MattermostAPI) HandleMatrixMessageRemove(ctx context.Context, remove *bridgev2.MatrixMessageRemove) error {
	if remove.TargetMessage == nil {
		return fmt.Errorf("no target message")
	}

	// Get the post ID from the target message
	postID := string(remove.TargetMessage.ID)

	// Delete the post in Mattermost
	_, err := m.Client.DeletePost(ctx, postID)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	return nil
}

// HandleMatrixReaction handles reaction events from Matrix, adding the reaction to the Mattermost post
func (m *MattermostAPI) HandleMatrixReaction(ctx context.Context, reaction *bridgev2.MatrixReaction) (reactionInfo *database.Reaction, err error) {
	if reaction.TargetMessage == nil {
		return nil, fmt.Errorf("no target message")
	}

	postID := string(reaction.TargetMessage.ID)

	// Get the emoji - bridgev2 provides the emoji via Content.RelatesTo.Key
	emoji := reaction.Content.RelatesTo.Key
	// Get the sender's Matrix user ID for ghost puppeting
	senderMXID := reaction.Event.Sender
	userClient, mmUserID, err := m.Connector.GetClientForUser(ctx, senderMXID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get client for ghost: %w", err)
	}

	// Create the reaction in Mattermost
	mmReaction := &model.Reaction{
		UserId:    mmUserID,
		PostId:    postID,
		EmojiName: emoji, // Mattermost uses emoji names like "thumbsup"
	}

	savedReaction, _, err := userClient.SaveReaction(ctx, mmReaction)
	if err != nil {
		return nil, fmt.Errorf("failed to save reaction: %w", err)
	}

	return &database.Reaction{
		EmojiID: networkid.EmojiID(savedReaction.EmojiName),
		Emoji:   savedReaction.EmojiName,
	}, nil
}

// HandleMatrixReactionRemove handles reaction removal events from Matrix
func (m *MattermostAPI) HandleMatrixReactionRemove(ctx context.Context, reaction *bridgev2.MatrixReactionRemove) error {
	if reaction.TargetReaction == nil {
		return fmt.Errorf("no target reaction")
	}

	// Get the post ID and emoji from the target reaction
	postID := string(reaction.TargetReaction.MessageID)
	emoji := string(reaction.TargetReaction.EmojiID)

	// Get the sender's Matrix user ID for ghost puppeting
	senderMXID := reaction.Event.Sender
	userClient, mmUserID, err := m.Connector.GetClientForUser(ctx, senderMXID.String())
	if err != nil {
		return fmt.Errorf("failed to get client for ghost: %w", err)
	}

	// Delete the reaction in Mattermost
	_, err = userClient.DeleteReaction(ctx, &model.Reaction{
		UserId:    mmUserID,
		PostId:    postID,
		EmojiName: emoji,
	})
	if err != nil {
		return fmt.Errorf("failed to delete reaction: %w", err)
	}

	return nil
}

// FetchMessages implements BackfillingNetworkAPI to support historical message backfill
func (m *MattermostAPI) FetchMessages(ctx context.Context, params bridgev2.FetchMessagesParams) (*bridgev2.FetchMessagesResponse, error) {
	channelID := string(params.Portal.ID)
	count := params.Count
	if count <= 0 {
		count = 50
	}

	// Get posts for channel
	var postList *model.PostList
	var err error

	if params.Forward {
		// Forward backfill: get messages after the anchor
		if params.AnchorMessage != nil {
			// Get posts after this message
			postList, _, err = m.Client.GetPostsAfter(ctx, channelID, string(params.AnchorMessage.ID), 0, count, "", false, false)
		} else {
			// No anchor, get latest posts
			postList, _, err = m.Client.GetPostsForChannel(ctx, channelID, 0, count, "", false, false)
		}
	} else {
		// Backward backfill: get messages before the anchor
		if params.AnchorMessage != nil {
			postList, _, err = m.Client.GetPostsBefore(ctx, channelID, string(params.AnchorMessage.ID), 0, count, "", false, false)
		} else {
			// No anchor, get latest posts for initial backfill
			postList, _, err = m.Client.GetPostsForChannel(ctx, channelID, 0, count, "", false, false)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get posts for backfill: %w", err)
	}

	// Convert posts to BackfillMessages
	// Posts need to be in chronological order (oldest first)
	messages := make([]*bridgev2.BackfillMessage, 0, len(postList.Order))

	// postList.Order is newest first, so process in reverse
	for i := len(postList.Order) - 1; i >= 0; i-- {
		postID := postList.Order[i]
		post := postList.Posts[postID]

		// Skip system messages
		if post.Type != "" && !strings.HasPrefix(post.Type, "custom_") {
			continue
		}

		// For backfill, we convert text directly without file uploads
		// Files would require intent which we don't have here, so we just create text parts
		converted := &bridgev2.ConvertedMessage{}

		// Handle text content
		if post.Message != "" {
			content := &event.MessageEventContent{
				Body:    post.Message,
				MsgType: event.MsgText,
			}
			converted.Parts = append(converted.Parts, &bridgev2.ConvertedMessagePart{
				Type:    event.EventMessage,
				Content: content,
			})
		}

		// Note: File attachments in backfill would need async handling
		// For now, we add a note about attachments
		if len(post.FileIds) > 0 && post.Message == "" {
			content := &event.MessageEventContent{
				Body:    fmt.Sprintf("[%d file attachment(s)]", len(post.FileIds)),
				MsgType: event.MsgNotice,
			}
			converted.Parts = append(converted.Parts, &bridgev2.ConvertedMessagePart{
				Type:    event.EventMessage,
				Content: content,
			})
		}

		if len(converted.Parts) == 0 {
			continue
		}

		// Build BackfillMessage
		bfMsg := &bridgev2.BackfillMessage{
			ConvertedMessage: converted,
			Sender: bridgev2.EventSender{
				Sender: networkid.UserID(m.Connector.GetUsername(ctx, post.UserId)),
			},
			ID:        networkid.MessageID(post.Id),
			Timestamp: time.UnixMilli(post.CreateAt),
		}

		// Handle thread root
		if post.RootId != "" {
			rootID := networkid.MessageID(post.RootId)
			bfMsg.ConvertedMessage.ThreadRoot = &rootID
		}

		// Fetch reactions for this post
		reactions, _, err := m.Client.GetReactions(ctx, post.Id)
		if err == nil && len(reactions) > 0 {
			bfMsg.Reactions = make([]*bridgev2.BackfillReaction, 0, len(reactions))
			for _, reaction := range reactions {
				bfMsg.Reactions = append(bfMsg.Reactions, &bridgev2.BackfillReaction{
					Sender: bridgev2.EventSender{
						Sender: networkid.UserID(m.Connector.GetUsername(ctx, reaction.UserId)),
					},
					EmojiID:   networkid.EmojiID(reaction.EmojiName),
					Emoji:     reaction.EmojiName,
					Timestamp: time.UnixMilli(reaction.CreateAt),
				})
			}
		}

		messages = append(messages, bfMsg)
	}

	// Determine if there are more messages
	hasMore := len(postList.Order) >= count

	return &bridgev2.FetchMessagesResponse{
		Messages: messages,
		HasMore:  hasMore,
		Forward:  params.Forward,
		MarkRead: params.Forward, // Mark read for forward backfill
	}, nil
}
