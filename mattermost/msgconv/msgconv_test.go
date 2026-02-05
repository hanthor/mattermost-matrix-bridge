package msgconv

import (
	"context"
	"testing"

	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// MockAPI implements bridgev2.NetworkAPI and MattermostClientProvider
type MockAPI struct {
	mock.Mock
}

func (m *MockAPI) Connect(ctx context.Context) error { return nil }
func (m *MockAPI) Disconnect() {}
func (m *MockAPI) IsConnected() bool { return true }
func (m *MockAPI) IsLoggedIn() bool { return true }
func (m *MockAPI) IsThisUser(ctx context.Context, userID networkid.UserID) bool { return false }
func (m *MockAPI) LogoutRemote(ctx context.Context) {}
func (m *MockAPI) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *bridgev2.NetworkRoomCapabilities {
	return nil
}
func (m *MockAPI) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	return nil, nil
}
func (m *MockAPI) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	return nil, nil
}
func (m *MockAPI) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (*bridgev2.MatrixMessageResponse, error) {
	return nil, nil
}

func (m *MockAPI) GetClient() *model.Client4 { return nil }
func (m *MockAPI) GetFile(ctx context.Context, fileID string) ([]byte, error) {
	args := m.Called(ctx, fileID)
	return args.Get(0).([]byte), args.Error(1)
}
func (m *MockAPI) GetFileInfo(ctx context.Context, fileID string) (*model.FileInfo, error) {
	args := m.Called(ctx, fileID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.FileInfo), args.Error(1)
}
func (m *MockAPI) GetFileWithInfo(ctx context.Context, fileID string) ([]byte, *model.FileInfo, error) {
	args := m.Called(ctx, fileID)
	var data []byte
	var info *model.FileInfo
	if args.Get(0) != nil {
		data = args.Get(0).([]byte)
	}
	if args.Get(1) != nil {
		info = args.Get(1).(*model.FileInfo)
	}
	return data, info, args.Error(2)
}
func (m *MockAPI) UploadFile(ctx context.Context, data []byte, channelID, filename string) (*model.FileInfo, error) {
	args := m.Called(ctx, data, channelID, filename)
	return args.Get(0).(*model.FileInfo), args.Error(1)
}

// MockMatrixAPI implements bridgev2.MatrixAPI
type MockMatrixAPI struct {
	mock.Mock
}

func (m *MockMatrixAPI) GetMXID() id.UserID { return "" }
func (m *MockMatrixAPI) UploadMedia(ctx context.Context, roomID id.RoomID, data []byte, fileName, mimeType string) (id.ContentURIString, *event.EncryptedFileInfo, error) {
	args := m.Called(ctx, roomID, data, fileName, mimeType)
	var fileInfo *event.EncryptedFileInfo
	if args.Get(1) != nil {
		fileInfo = args.Get(1).(*event.EncryptedFileInfo)
	}
	return id.ContentURIString(args.String(0)), fileInfo, args.Error(2)
}
// Add other MatrixAPI methods as stubs if needed, but ToMatrix mainly uses UploadMedia (via fileToMatrix)
func (m *MockMatrixAPI) SendMessage(ctx context.Context, roomID id.RoomID, eventType event.Type, content *event.Content, extra *bridgev2.MatrixSendExtra) (*mautrix.RespSendEvent, error) { return nil, nil }
func (m *MockMatrixAPI) SendState(ctx context.Context, roomID id.RoomID, eventType event.Type, stateKey string, content *event.Content, ts time.Time) (*mautrix.RespSendEvent, error) { return nil, nil }
func (m *MockMatrixAPI) MarkRead(ctx context.Context, roomID id.RoomID, eventID id.EventID, ts time.Time) error { return nil }
func (m *MockMatrixAPI) MarkUnread(ctx context.Context, roomID id.RoomID, unread bool) error { return nil }
func (m *MockMatrixAPI) MarkTyping(ctx context.Context, roomID id.RoomID, typingType bridgev2.TypingType, timeout time.Duration) error { return nil }
func (m *MockMatrixAPI) DownloadMedia(ctx context.Context, url id.ContentURIString, file *event.EncryptedFileInfo) ([]byte, error) { return nil, nil }

// Additional missing methods from Interface
func (m *MockMatrixAPI) SetDisplayName(ctx context.Context, name string) error { return nil }
func (m *MockMatrixAPI) SetAvatarURL(ctx context.Context, avatarURL id.ContentURIString) error { return nil }
func (m *MockMatrixAPI) SetExtraProfileMeta(ctx context.Context, data any) error { return nil }
func (m *MockMatrixAPI) CreateRoom(ctx context.Context, req *mautrix.ReqCreateRoom) (id.RoomID, error) { return "", nil }
func (m *MockMatrixAPI) DeleteRoom(ctx context.Context, roomID id.RoomID, puppetsOnly bool) error { return nil }
func (m *MockMatrixAPI) InviteUser(ctx context.Context, roomID id.RoomID, userID id.UserID) error { return nil }
func (m *MockMatrixAPI) EnsureJoined(ctx context.Context, roomID id.RoomID) error { return nil }
func (m *MockMatrixAPI) EnsureInvited(ctx context.Context, roomID id.RoomID, userID id.UserID) error { return nil }
func (m *MockMatrixAPI) TagRoom(ctx context.Context, roomID id.RoomID, tag event.RoomTag, isTagged bool) error { return nil }
func (m *MockMatrixAPI) MuteRoom(ctx context.Context, roomID id.RoomID, until time.Time) error { return nil }


func TestToMatrix_Text(t *testing.T) {
	mc := &MessageConverter{
		ServerName: "example.com",
	}

	ctx := context.Background()
	mockAPI := new(MockAPI)
	source := &bridgev2.UserLogin{
		Client: mockAPI,
	}
	portal := &bridgev2.Portal{
		Portal: &database.Portal{
			PortalKey: networkid.PortalKey{ID: networkid.PortalID("channel1")},
		},
	}
	
	post := &model.Post{
		Message: "Hello *world*!",
	}
	
	converted := mc.ToMatrix(ctx, portal, nil, source, post)
	
	assert.Len(t, converted.Parts, 1)
	assert.Equal(t, event.EventMessage, converted.Parts[0].Type)
	// Output seems to prefer underscores for emphasis and strip outer p tags?
	assert.Equal(t, "Hello _world_!", converted.Parts[0].Content.Body)
	assert.Equal(t, "Hello <em>world</em>!", converted.Parts[0].Content.FormattedBody)
}

func TestToMatrix_File(t *testing.T) {
	mc := &MessageConverter{
		ServerName:  "example.com",
		MaxFileSize: 50 * 1024 * 1024,
	}

	ctx := context.Background()
	mockAPI := new(MockAPI)
	mockMatrix := new(MockMatrixAPI)
	
	source := &bridgev2.UserLogin{
		Client: mockAPI,
	}
	portal := &bridgev2.Portal{
		Portal: &database.Portal{
			PortalKey: networkid.PortalKey{ID: networkid.PortalID("channel1")},
			MXID: id.RoomID("!room:example.com"),
		},
	}
	
	fileID := "file123"
	fileContent := []byte("fake image")
	fileInfo := &model.FileInfo{
		Id:       fileID,
		Name:     "test.png",
		MimeType: "image/png",
		Size:     int64(len(fileContent)),
		Width:    100,
		Height:   100,
	}
	post := &model.Post{
		Message: "",
		FileIds: []string{fileID},
	}
	
	// Mock GetFileWithInfo to return both content and metadata
	mockAPI.On("GetFileWithInfo", mock.Anything, fileID).Return(fileContent, fileInfo, nil)
	mockMatrix.On("UploadMedia", mock.Anything, portal.MXID, fileContent, "test.png", "image/png").Return("mxc://example.com/xyz", nil, nil)

	converted := mc.ToMatrix(ctx, portal, mockMatrix, source, post)
	
	assert.Len(t, converted.Parts, 1)
	assert.Equal(t, event.EventMessage, converted.Parts[0].Type)
	assert.Equal(t, "test.png", converted.Parts[0].Content.Body)
	assert.Equal(t, id.ContentURIString("mxc://example.com/xyz"), converted.Parts[0].Content.URL)
	// Verify image dimensions are set
	assert.Equal(t, 100, converted.Parts[0].Content.Info.Width)
	assert.Equal(t, 100, converted.Parts[0].Content.Info.Height)
}
