package mattermost

import (
	"context"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
)


type MattermostEvent struct {
	Connector *MattermostConnector
	Timestamp time.Time
	ChannelID string
	UserID    string
	Username  string
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
		Sender: networkid.UserID(e.Username),
	}
}

type MattermostMessageEvent struct {
	MattermostEvent
	PostID  string
	Content string
	FileIds []string
	RootID  string // Thread root post ID (empty if not a reply)
}

func (e *MattermostMessageEvent) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventMessage
}

func (e *MattermostMessageEvent) GetID() networkid.MessageID {
	return networkid.MessageID(e.PostID)
}

func (e *MattermostMessageEvent) ConvertMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI) (*bridgev2.ConvertedMessage, error) {
	// We need source user login for msgconv to download files/use client
	// bridgev2 passes intent, but we need UserLogin to access Mattermost Client if we want to download files.
	// Wait, ToMatrix needs `source *bridgev2.UserLogin`.
	// We might need to find the specific user login or use a default one.
	// Typically `e.Connector` has access to users.
	// But `MattermostEvent` doesn't have the Source UserLogin directly attached, only UserID.
	
	// Quick fix: Find the UserLogin for this event's UserID if possible, or use any valid client.
	// However, downloading files usually requires just *any* valid token if public/channel access is allowed.
	// But if it's a DM, we need the user's token.
	
	var source *bridgev2.UserLogin
	// We can try to look it up from Connector.users
	// e.UserID is the Mattermost User ID.
	// `connector.go` stores users by UserLoginID, which we set to UserID in `login.go`.
	
	e.Connector.usersLock.RLock()
	source = e.Connector.users[networkid.UserLoginID(e.Username)]
	e.Connector.usersLock.RUnlock()
	
	if source == nil {
		// Fallback: use any connected user (like the bridge bot/sysadmin if configured as a login)
		// Or creating a temporary "bot" client if we have a system admin token in Config?
		// MattermostConnector has `m.Client` which is the system client (admin token).
		// We can wrap it in a pseudo-UserLogin or just modify `ToMatrix` signature?
		// `ToMatrix` expects `*bridgev2.UserLogin` because it casts `Client` to `MattermostClientProvider`.
		
		// Let's rely on standard logic: if we don't have the user login, we might not be able to bridge perfectly if auth is strict.
		// BUT, `m.Connector.Client` is a `*Client` which matches `MattermostClientProvider`.
		// So we can mock a UserLogin or modify `ToMatrix` to accept `MattermostClientProvider`.
		
		// For now, let's just cheat and make a dummy UserLogin wrapping existing m.Connector.Client
		source = &bridgev2.UserLogin{
			Client: &MattermostAPI{Connector: e.Connector, Client: e.Connector.Client},
		}
	}
	
	// Re-construct the Post object since we only have fields in struct
	post := &model.Post{
		Id:        e.PostID,
		ChannelId: e.ChannelID,
		UserId:    e.UserID,
		Message:   e.Content,
		FileIds:   e.FileIds,
		RootId:    e.RootID, // Thread root for replies
	}
	
	msg := e.Connector.MsgConv.ToMatrix(ctx, portal, intent, source, post)
	return msg, nil
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

// MattermostReactionEvent represents a reaction added or removed from a post
type MattermostReactionEvent struct {
	MattermostEvent
	PostID    string
	EmojiName string
	Added     bool // true = reaction added, false = reaction removed
}

func (e *MattermostReactionEvent) GetType() bridgev2.RemoteEventType {
	if e.Added {
		return bridgev2.RemoteEventReaction
	}
	return bridgev2.RemoteEventReactionRemove
}

func (e *MattermostReactionEvent) GetTargetMessage() networkid.MessageID {
	return networkid.MessageID(e.PostID)
}

// GetReactionEmoji returns the emoji for bridgev2.RemoteReaction interface
func (e *MattermostReactionEvent) GetReactionEmoji() (string, networkid.EmojiID) {
	// Mattermost uses emoji names like "thumbsup", convert to Unicode if possible
	// For now, we'll use the emoji name directly; emoji conversion could be enhanced
	return e.EmojiName, networkid.EmojiID(e.EmojiName)
}

// GetRemovedEmojiID returns the emoji ID for reaction removal
func (e *MattermostReactionEvent) GetRemovedEmojiID() networkid.EmojiID {
	return networkid.EmojiID(e.EmojiName)
}
