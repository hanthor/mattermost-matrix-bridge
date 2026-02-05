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

	// Get file with metadata for better filename and mime type detection
	data, fileInfo, err := client.GetFileWithInfo(ctx, fileID)
	if err != nil {
		log.Err(err).Msg("Failed to get file with info from Mattermost")
		// Fallback to just downloading the file
		data, err = client.GetFile(ctx, fileID)
		if err != nil {
			log.Err(err).Msg("Failed to download file from Mattermost")
			return nil
		}
	}
	
	// Determine filename and mime type
	var fileName, mimeType string
	if fileInfo != nil {
		fileName = fileInfo.Name
		mimeType = fileInfo.MimeType
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
	} else {
		// Fallback: detect content type and generate filename
		mimeType = http.DetectContentType(data)
		fileName = "file"
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			fileName += exts[0]
		}
	}

	// Check file size against limit
	if mc.MaxFileSize > 0 && int64(len(data)) > mc.MaxFileSize {
		log.Warn().Int64("size", int64(len(data))).Int64("max", mc.MaxFileSize).Msg("File too large, skipping")
		return nil
	}

	mxc, file, err := intent.UploadMedia(ctx, portal.MXID, data, fileName, mimeType)
	if err != nil {
		log.Err(err).Msg("Failed to upload file to Matrix")
		return nil
	}

	content := &event.MessageEventContent{
		Body: fileName,
		Info: &event.FileInfo{
			MimeType: mimeType,
			Size:     len(data),
		},
	}
	
	// Add image dimensions if available
	if fileInfo != nil && (mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/gif" || strings.HasPrefix(mimeType, "image/")) {
		if fileInfo.Width > 0 && fileInfo.Height > 0 {
			content.Info.Width = fileInfo.Width
			content.Info.Height = fileInfo.Height
		}
	}
	
	if file != nil {
		content.File = file
	} else {
		content.URL = mxc
	}
	content.MsgType = mimeToMsgType(mimeType)

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
