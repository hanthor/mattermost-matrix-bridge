package mattermost

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

func (m *MattermostConnector) StartWebSocket() {
	wsURL := m.Config.ServerURL
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	// Connect to WebSocket
	wsClient, err := model.NewWebSocketClient4(wsURL, m.Client.AdminToken)
	if err != nil {
		fmt.Printf("Failed to create WebSocket client: %v\n", err)
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
				fmt.Printf("DEBUG: Received websocket event: %s\n", event.EventType())
				m.HandleWebSocketEvent(event)
			case _ = <-m.WSClient.ResponseChannel:
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

		evt := &MattermostMessageEvent{
			MattermostEvent: MattermostEvent{
				Connector: m,
				Timestamp: time.Unix(post.CreateAt/1000, (post.CreateAt%1000)*1000000),
				ChannelID: post.ChannelId,
				UserID:    post.UserId,
			},
			PostID:  post.Id,
			Content: post.Message,
			FileIds: post.FileIds,
			RootID:  post.RootId, // Thread root for replies
		}


		// We need to find the correct UserLogin to queue this event.
		// Since we are using an Admin API, we might have one primary login
		// that "receives" all events, or we might need to map it.
		
		// For now, let's assume we have a "manager" login or we find any login.
		users := m.GetUsers()
		fmt.Printf("DEBUG: Found %d users for event\n", len(users))
		for _, login := range users {
			m.Bridge.QueueRemoteEvent(login, evt)
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
				},
				PostID:  post.Id,
				Content: post.Message,
			},
		}

		// Find the user login to dispatch the event
		for _, login := range m.GetUsers() {
			m.Bridge.QueueRemoteEvent(login, evt)
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
			},
			PostID: post.Id,
		}

		// Find the user login to dispatch the event
		for _, login := range m.GetUsers() {
			m.Bridge.QueueRemoteEvent(login, evt)
		}


	}
}

