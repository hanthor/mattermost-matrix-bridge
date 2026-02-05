package mattermost

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"github.com/mattermost/mattermost/server/public/model"
)





type MattermostAPI struct {
	Login     *bridgev2.UserLogin
	Connector *MattermostConnector
	Client    *Client
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
				ci.Members.Members = make([]bridgev2.ChatMember, len(members))
				for i, member := range members {
					ci.Members.Members[i] = bridgev2.ChatMember{
						EventSender: bridgev2.EventSender{Sender: networkid.UserID(member.UserId)},
					}
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
						EventSender: bridgev2.EventSender{Sender: networkid.UserID(member.UserId)},
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
			Name: &team.DisplayName,
			Topic: &team.Description,
			Type: ptr.Ptr(database.RoomTypeSpace),
		}, nil
	}

	return nil, fmt.Errorf("item not found (tried channel and team)")
}


func (m *MattermostAPI) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	user, _, err := m.Client.GetUser(ctx, string(ghost.ID), "")
	if err != nil {
		return nil, err
	}
	name := user.Username
	if user.FirstName != "" || user.LastName != "" {
		name = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	}
	return &bridgev2.UserInfo{
		Name: &name,
		Avatar: &bridgev2.Avatar{
			ID: networkid.AvatarID(fmt.Sprintf("%d", user.LastPictureUpdate)),
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

func (m *MattermostAPI) LogoutRemote(ctx context.Context) {
}

func (m *MattermostAPI) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (*bridgev2.MatrixMessageResponse, error) {
	post, err := m.Connector.MsgConv.ToMattermost(ctx, m.Client, msg.Portal, msg.Content)
	if err != nil {
		return nil, err
	}

	post.ChannelId = string(msg.Portal.ID)
	// post.Message is already set by ToMattermost
	// post.FileIds is already set by ToMattermost

	// Handle thread replies: if there's a thread root, set RootId
	if msg.ThreadRoot != nil {
		post.RootId = string(msg.ThreadRoot.ID)
	}

	createdPost, _, err := m.Client.CreatePost(ctx, post)
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
	
	// Ghost ID is just the UserID
	ghostID := networkid.UserID(user.Id)
	ghost, err := m.Connector.Bridge.GetGhostByID(ctx, ghostID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ghost: %w", err)
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
	myUserID := string(m.Login.ID)
	otherUserID := string(ghost.ID)
	
	channel, err := m.Client.CreateDirectChannelWithBoth(ctx, myUserID, otherUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to create direct channel: %w", err)
	}
	
	// Wrap into PortalInfo
	// Reuse GetChatInfo logic or manually construct?
	// GetChatInfo expects a Portal, we don't have one yet fully formed but we can make a dummy one for theID
	// Or just construct ChatInfo manually.
	
	// Actually, CreateDirectChannel returns a channel object which is enough.
	portalID := networkid.PortalID(channel.Id)
	
	return &bridgev2.CreateChatResponse{
		PortalKey: networkid.PortalKey{
			ID: portalID,
			Receiver: m.Login.ID, // DMs usually depend on receiver if we want separate portals per login? 
			// Mattermost DMs have constant IDs (ChannelID).
			// So Receiver should be empty if we want a shared portal.
			// But DMs in bridgev2 are often per-user if encryption or mapping matters.
			// Let's use empty receiver for shared processing if possible, 
			// BUT `GetChatInfo` needs to be able to fetch it. `GetChannel` works with any user.
			// However, for DMs, visibility might be restricted.
			// Let's try shared portal first (Receiver: "").
		},
		PortalInfo: &bridgev2.ChatInfo{
			Name: nil, // DMs don't have names usually
			Type: ptr.Ptr(database.RoomTypeDM),
			Members: &bridgev2.ChatMemberList{
				IsFull: true,
				Members: []bridgev2.ChatMember{
					{EventSender: bridgev2.EventSender{Sender: networkid.UserID(myUserID)}},
					{EventSender: bridgev2.EventSender{Sender: networkid.UserID(otherUserID)}},
				},
			},
		},
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





