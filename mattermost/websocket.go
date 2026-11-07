package mattermost

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

func (m *MattermostConnector) StartWebSocket() {
	wsURL := m.Config.ServerURL
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	// Connect to WebSocket
	wsClient, err := model.NewWebSocketClient4(wsURL, m.Client.AdminToken)
	if err != nil {
		m.Bridge.Log.Error().Msgf("Failed to create WebSocket client: %v", err)
		return
	}
	m.WSClient = wsClient
	m.WSClient.Listen()

	go func() {
		for {
			select {
			case event, ok := <-m.WSClient.EventChannel:
				if !ok {
					return
				}
				m.Bridge.Log.Debug().Msgf("Received websocket event: %s", event.EventType())
				m.HandleWebSocketEvent(event)
			case <-m.WSClient.ResponseChannel:
				// Handle responses if needed
			}
		}
	}()
}

func (m *MattermostConnector) HandleWebSocketEvent(event *model.WebSocketEvent) {
	switch event.EventType() {
	case model.WebsocketEventPosted:
		postStr, ok := event.GetData()["post"].(string)
		if !ok {
			return
		}
		var post model.Post
		err := json.Unmarshal([]byte(postStr), &post)
		if err != nil {
			return
		}

		// Discard events from the bridge itself if necessary
		// But bridgev2 handles some of this via SenderLogin/Sender

		// Filter out system messages
		if post.Type != "" && !strings.HasPrefix(post.Type, "custom_") {
			return
		}

		evt := &MattermostMessageEvent{
			MattermostEvent: MattermostEvent{
				Connector: m,
				Timestamp: time.Unix(post.CreateAt/1000, (post.CreateAt%1000)*1000000),
				ChannelID: post.ChannelId,
				UserID:    post.UserId,
				Username:  m.GetUsername(m.ctx, post.UserId),
			},
			PostID:  post.Id,
			Content: post.Message,
			FileIds: post.FileIds,
			RootID:  post.RootId, // Thread root for replies
		}

		// We need to find the correct UserLogin to queue this event.
		// Since we are using an Admin API, we might have one primary login
		// that "receives" all events, or we might need to map it.

		// Dispatch to logins
		logins := m.GetUsers()
		m.Bridge.Log.Debug().Msgf("Found %d logins for event", len(logins))
		if m.IsMirrorMode() {
			// In mirror mode, any login can process the event
			if len(logins) > 0 {
				m.Bridge.QueueRemoteEvent(logins[0], evt)
			}
		} else {
			// In puppet mode, we might need to find the specific login
			for _, login := range logins {
				m.Bridge.QueueRemoteEvent(login, evt)
			}
		}

	case model.WebsocketEventPostEdited:
		postStr, ok := event.GetData()["post"].(string)
		if !ok {
			return
		}
		var post model.Post
		err := json.Unmarshal([]byte(postStr), &post)
		if err != nil {
			return
		}

		evt := &MattermostEditEvent{
			MattermostMessageEvent: MattermostMessageEvent{
				MattermostEvent: MattermostEvent{
					Connector: m,
					Timestamp: time.Unix(post.EditAt/1000, (post.EditAt%1000)*1000000),
					ChannelID: post.ChannelId,
					UserID:    post.UserId,
					Username:  m.GetUsername(m.ctx, post.UserId),
				},
				PostID:  post.Id,
				Content: post.Message,
				FileIds: post.FileIds,
				RootID:  post.RootId,
			},
		}

		// Find the user login to dispatch the event
		logins := m.GetUsers()
		if m.IsMirrorMode() {
			if len(logins) > 0 {
				m.Bridge.QueueRemoteEvent(logins[0], evt)
			}
		} else {
			for _, login := range logins {
				m.Bridge.QueueRemoteEvent(login, evt)
			}
		}

	case model.WebsocketEventPostDeleted:
		postStr, ok := event.GetData()["post"].(string)
		if !ok {
			return
		}
		var post model.Post
		err := json.Unmarshal([]byte(postStr), &post)
		if err != nil {
			return
		}

		evt := &MattermostRemoveEvent{
			MattermostEvent: MattermostEvent{
				Connector: m,
				Timestamp: time.Unix(post.DeleteAt/1000, (post.DeleteAt%1000)*1000000),
				ChannelID: post.ChannelId,
				UserID:    post.UserId,
				Username:  m.GetUsername(m.ctx, post.UserId),
			},
			PostID: post.Id,
		}

		// Find the user login to dispatch the event
		logins := m.GetUsers()
		if m.IsMirrorMode() {
			if len(logins) > 0 {
				m.Bridge.QueueRemoteEvent(logins[0], evt)
			}
		} else {
			for _, login := range logins {
				m.Bridge.QueueRemoteEvent(login, evt)
			}
		}

	case model.WebsocketEventReactionAdded:
		reactionStr, ok := event.GetData()["reaction"].(string)
		if !ok {
			return
		}
		var reaction model.Reaction
		err := json.Unmarshal([]byte(reactionStr), &reaction)
		if err != nil {
			return
		}

		evt := &MattermostReactionEvent{
			MattermostEvent: MattermostEvent{
				Connector: m,
				Timestamp: time.Unix(reaction.CreateAt/1000, (reaction.CreateAt%1000)*1000000),
				ChannelID: reaction.ChannelId,
				UserID:    reaction.UserId,
				Username:  m.GetUsername(m.ctx, reaction.UserId),
			},
			PostID:    reaction.PostId,
			EmojiName: reaction.EmojiName,
			Added:     true,
		}

		logins := m.GetUsers()
		if m.IsMirrorMode() {
			if len(logins) > 0 {
				m.Bridge.QueueRemoteEvent(logins[0], evt)
			}
		} else {
			for _, login := range logins {
				m.Bridge.QueueRemoteEvent(login, evt)
			}
		}

	case model.WebsocketEventReactionRemoved:
		reactionStr, ok := event.GetData()["reaction"].(string)
		if !ok {
			return
		}
		var reaction model.Reaction
		err := json.Unmarshal([]byte(reactionStr), &reaction)
		if err != nil {
			return
		}

		evt := &MattermostReactionEvent{
			MattermostEvent: MattermostEvent{
				Connector: m,
				Timestamp: time.Now(), // DeleteAt not always available
				ChannelID: reaction.ChannelId,
				UserID:    reaction.UserId,
				Username:  m.GetUsername(m.ctx, reaction.UserId),
			},
			PostID:    reaction.PostId,
			EmojiName: reaction.EmojiName,
			Added:     false,
		}

		logins := m.GetUsers()
		if m.IsMirrorMode() {
			if len(logins) > 0 {
				m.Bridge.QueueRemoteEvent(logins[0], evt)
			}
		} else {
			for _, login := range logins {
				m.Bridge.QueueRemoteEvent(login, evt)
			}
		}

	case model.WebsocketEventUserUpdated:
		userStr, ok := event.GetData()["user"].(string)
		if !ok {
			return
		}
		var user model.User
		err := json.Unmarshal([]byte(userStr), &user)
		if err != nil {
			return
		}

		ghost, err := m.Bridge.GetGhostByID(m.ctx, networkid.UserID(user.Username))
		if err == nil && ghost != nil {
			logins := m.GetUsers()
			if len(logins) > 0 && logins[0].Client != nil {
				if api, ok := logins[0].Client.(*MattermostAPI); ok {
					info, err := api.GetUserInfo(m.ctx, ghost)
					if err == nil {
						ghost.UpdateInfo(m.ctx, info)
						m.Bridge.Log.Info().Msgf("Syncing profile for updated user %s", user.Username)
					}
				}
			}
		}

	}
}
