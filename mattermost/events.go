package mattermost

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
)


type MattermostEvent struct {
	Connector *MattermostConnector
	Timestamp time.Time
	ChannelID string
	UserID    string
}

func (e *MattermostEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

func (e *MattermostEvent) GetPortalKey() networkid.PortalKey {
	return networkid.PortalKey{
		ID:       networkid.PortalID(e.ChannelID),
		Receiver: "", // Will be filled by bridgev2 if needed
	}
}

func (e *MattermostEvent) AddLogContext(c zerolog.Context) zerolog.Context {
	return c.Str("channel_id", e.ChannelID).Str("user_id", e.UserID)
}

func (e *MattermostEvent) GetSender() bridgev2.EventSender {
	return bridgev2.EventSender{
		Sender: networkid.UserID(e.UserID),
	}
}

type MattermostMessageEvent struct {
	MattermostEvent
	PostID  string
	Content string
}

func (e *MattermostMessageEvent) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventMessage
}

func (e *MattermostMessageEvent) GetID() networkid.MessageID {
	return networkid.MessageID(e.PostID)
}

func (e *MattermostMessageEvent) ConvertMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI) (*bridgev2.ConvertedMessage, error) {
	return &bridgev2.ConvertedMessage{
		Parts: []*bridgev2.ConvertedMessagePart{
			{
				Type: event.EventMessage,
				Content: &event.MessageEventContent{
					MsgType: event.MsgText,
					Body:    e.Content,
				},
			},
		},
	}, nil
}

func (e *MattermostMessageEvent) ShouldCreatePortal() bool {
	return true
}

type MattermostEditEvent struct {
	MattermostMessageEvent
}

func (e *MattermostEditEvent) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventEdit
}

func (e *MattermostEditEvent) GetTargetMessage() networkid.MessageID {
	return networkid.MessageID(e.PostID)
}

func (e *MattermostEditEvent) ConvertEdit(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message) (*bridgev2.ConvertedEdit, error) {
	msg, err := e.ConvertMessage(ctx, portal, intent)
	if err != nil {
		return nil, err
	}
	parts := make([]*bridgev2.ConvertedEditPart, len(existing))
	for i, dbMsg := range existing {
		parts[i] = msg.Parts[0].ToEditPart(dbMsg)
	}
	return &bridgev2.ConvertedEdit{
		ModifiedParts: parts,
	}, nil
}

type MattermostRemoveEvent struct {
	MattermostEvent
	PostID string
}

func (e *MattermostRemoveEvent) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventMessageRemove
}

func (e *MattermostRemoveEvent) GetTargetMessage() networkid.MessageID {
	return networkid.MessageID(e.PostID)
}

func (e *MattermostRemoveEvent) GetID() networkid.MessageID {
	return networkid.MessageID(e.PostID)
}

