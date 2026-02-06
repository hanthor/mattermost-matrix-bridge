package msgconv

import (
	"context"
	"fmt"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"
)

var converter *md.Converter

func init() {
	converter = md.NewConverter("", true, nil)
}

func (mc *MessageConverter) ToMattermost(
	ctx context.Context,
	client MattermostClientProvider,
	portal *bridgev2.Portal,
	content *event.MessageEventContent,
) (*model.Post, error) {
	log := zerolog.Ctx(ctx)

	post := &model.Post{}

	// Convert Text
	var body string
	if content.Format == event.FormatHTML {
		var err error
		body, err = converter.ConvertString(content.FormattedBody)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to convert HTML to Markdown, falling back to plain text")
			body = content.Body
		}
	} else {
		body = content.Body
	}
	post.Message = body
	log.Info().Str("body", body).Msg("ToMattermost converted body")

	// Handle Media
	if content.MsgType == event.MsgImage || content.MsgType == event.MsgFile || content.MsgType == event.MsgVideo || content.MsgType == event.MsgAudio {
		data, err := mc.Bridge.Bot.DownloadMedia(ctx, content.URL, content.File)
		if err != nil {
			return nil, fmt.Errorf("failed to download media from Matrix: %w", err)
		}

		fileName := content.FileName
		if fileName == "" {
			fileName = content.Body
		}
		if fileName == "" {
			fileName = "file" // TODO: guess extension
		}

		fileInfo, err := client.UploadFile(ctx, data, string(portal.ID), fileName)
		if err != nil {
			return nil, fmt.Errorf("failed to upload file to Mattermost: %w", err)
		}
		if fileInfo != nil {
			post.FileIds = []string{fileInfo.Id}
		}
	}

	return post, nil
}
