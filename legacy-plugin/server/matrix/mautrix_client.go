package matrix

import (
	"context"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// MautrixClient is a wrapper around the mautrix-go client to be explored.
type MautrixClient struct {
	client *mautrix.Client
}

// NewMautrixClient creates a new mautrix-go client wrapper.
func NewMautrixClient(homeserverURL string, userID id.UserID, accessToken string) (*MautrixClient, error) {
	client, err := mautrix.NewClient(homeserverURL, userID, accessToken)
	if err != nil {
		return nil, err
	}
	return &MautrixClient{client: client}, nil
}

// SendMessage sends a message using the mautrix-go client.
func (m *MautrixClient) SendMessage(ctx context.Context, roomID id.RoomID, message string) (*mautrix.RespSendEvent, error) {
	return m.client.SendMessageEvent(ctx, roomID, event.EventMessage, event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	})
}
