package msgconv

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
	"maunium.net/go/mautrix/bridgev2"
)

type MessageConverter struct {
	Bridge      *bridgev2.Bridge
	ServerName  string
	MaxFileSize int64
}

func New(br *bridgev2.Bridge) *MessageConverter {
	return &MessageConverter{
		Bridge:      br,
		ServerName:  br.Matrix.ServerName(),
		MaxFileSize: 50 * 1024 * 1024, // Default to 50MB, should potentially be configurable
	}
}

type MattermostClientProvider interface {
	GetClient() *model.Client4
	GetFile(ctx context.Context, fileID string) ([]byte, error)
	UploadFile(ctx context.Context, data []byte, channelID, filename string) (*model.FileInfo, error)
}

type contextKey int

const (
	contextKeyPortal contextKey = iota
	contextKeySource
)

func GetPortal(ctx context.Context) *bridgev2.Portal {
	return ctx.Value(contextKeyPortal).(*bridgev2.Portal)
}

func GetSource(ctx context.Context) *bridgev2.UserLogin {
	return ctx.Value(contextKeySource).(*bridgev2.UserLogin)
}
