package msgconv

import (
	"context"
	"mime"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
)

func (mc *MessageConverter) ToMatrix(
	ctx context.Context,
	portal *bridgev2.Portal,
	intent bridgev2.MatrixAPI,
	source *bridgev2.UserLogin,
	post *model.Post,
) *bridgev2.ConvertedMessage {
	ctx = context.WithValue(ctx, contextKeyPortal, portal)
	ctx = context.WithValue(ctx, contextKeySource, source)
	
	output := &bridgev2.ConvertedMessage{}

	// Handle Reply
	if post.RootId != "" {
		output.ThreadRoot = ptr(networkid.MessageID(post.RootId))
	} else {
		// Verify if it is really a root post or if Mattermost handles threads differently
		// Mattermost posts with RootId="" are root posts.
	}

	// Handle Text
	if post.Message != "" {
		content := format.RenderMarkdown(post.Message, true, false)
		output.Parts = append(output.Parts, &bridgev2.ConvertedMessagePart{
			Type:    event.EventMessage,
			Content: &content,
		})
	}

	// Handle Files
	if len(post.FileIds) > 0 {
		client := source.Client.(MattermostClientProvider)
		for _, fileID := range post.FileIds {
			partID := networkid.PartID(fileID)
			filePart := mc.fileToMatrix(ctx, portal, intent, client, partID, fileID)
			if filePart != nil {
				output.Parts = append(output.Parts, filePart)
			}
		}
	}

	// If post has message and files, we might want to merge caption
	if len(output.Parts) > 1 && post.Message != "" {
		// Logic to merge caption if the first part is text and second is file
		// bridgev2.MergeCaption can be used if we want to attach text as caption to the first file
		// For now, let's keep them separate or use MergeCaption helper
		if output.MergeCaption() {
			// merged
		}
	}

	return output
}

func (mc *MessageConverter) fileToMatrix(
	ctx context.Context,
	portal *bridgev2.Portal,
	intent bridgev2.MatrixAPI,
	client MattermostClientProvider,
	partID networkid.PartID,
	fileID string,
) *bridgev2.ConvertedMessagePart {
	log := zerolog.Ctx(ctx).With().Str("file_id", fileID).Logger()

	// We need file info first? Or just download it?
	// Mattermost API GetFile usually returns the bytes.
	// But we also need metadata (filename, mime type).
	// Ideally we fetch info then download.
	// client.GetFile(fileID) -> returns bytes. `client.GetFileInfo(fileID)` returns metadata.
	// However, our wrapper `MattermostClientProvider` currently only has `GetFile` returning bytes.
	// We might need to update that interface or just download and sniff mime type.
	// BUT, `GetFile` in Mattermost driver usually implies fetching content.
	
	// Let's assume we update `MattermostClientProvider` or `Client` to provide file info. 
	// For now, let's try to just download it and guess.
	// Actually, `model.FileInfo` exists in Mattermost.
	
	data, err := client.GetFile(ctx, fileID)
	if err != nil {
		log.Err(err).Msg("Failed to download file from Mattermost")
		return nil
	}
	
	// We need to guess filename/mimetype if not available.
	// Wait, we can get file info from client if we implement it.
	// Let's assume we can get it. For now, simplistic approach.
	
	mimeType := http.DetectContentType(data)
	fileName := "file" 
	// Try to get extension from mime
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		fileName += exts[0]
	}

	uploadMime := mimeType
	uploadFileName := fileName

	mxc, file, err := intent.UploadMedia(ctx, portal.MXID, data, uploadFileName, uploadMime)
	if err != nil {
		log.Err(err).Msg("Failed to upload file to Matrix")
		return nil
	}

	content := &event.MessageEventContent{
		Body: uploadFileName,
		Info: &event.FileInfo{
			MimeType: uploadMime,
			Size:     len(data),
		},
	}
	if file != nil {
		content.File = file
	} else {
		content.URL = mxc
	}
	content.MsgType = mimeToMsgType(uploadMime)

	return &bridgev2.ConvertedMessagePart{
		ID:      partID,
		Type:    event.EventMessage,
		Content: content,
	}
}

func mimeToMsgType(mime string) event.MessageType {
	if strings.HasPrefix(mime, "image/") {
		return event.MsgImage
	} else if strings.HasPrefix(mime, "video/") {
		return event.MsgVideo
	} else if strings.HasPrefix(mime, "audio/") {
		return event.MsgAudio
	}
	return event.MsgFile
}

func ptr[T any](v T) *T {
	return &v
}
