package mattermost

import (
	"context"
	"fmt"

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
	channel, _, err := m.Client.GetChannel(ctx, string(portal.ID), "")
	if err != nil {
		return nil, err
	}
	return &bridgev2.ChatInfo{
		Name:  &channel.DisplayName,
		Topic: &channel.Purpose,
	}, nil
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
	post := &model.Post{
		ChannelId: string(msg.Portal.ID),
		Message:   msg.Content.Body,
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









