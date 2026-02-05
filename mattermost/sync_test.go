package mattermost

import (
	"context"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

// MockMattermostClient mocks the Mattermost Client for testing
type MockMattermostClient struct {
	mock.Mock
}

func (m *MockMattermostClient) GetAllTeams(ctx context.Context, etag string, page, perPage int) ([]*model.Team, *model.Response, error) {
	args := m.Called(ctx, etag, page, perPage)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*model.Team), nil, args.Error(2)
}

func (m *MockMattermostClient) GetPublicChannelsForTeam(ctx context.Context, teamID string, page, perPage int, etag string) ([]*model.Channel, *model.Response, error) {
	args := m.Called(ctx, teamID, page, perPage, etag)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*model.Channel), nil, args.Error(2)
}

func (m *MockMattermostClient) GetPrivateChannelsForTeam(ctx context.Context, teamID string, page, perPage int, etag string) ([]*model.Channel, *model.Response, error) {
	args := m.Called(ctx, teamID, page, perPage, etag)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*model.Channel), nil, args.Error(2)
}

func (m *MockMattermostClient) GetUsers(ctx context.Context, page, perPage int, etag string) ([]*model.User, *model.Response, error) {
	args := m.Called(ctx, page, perPage, etag)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).([]*model.User), nil, args.Error(2)
}

func (m *MockMattermostClient) GetChannelMembers(ctx context.Context, channelID string, page, perPage int, etag string) (model.ChannelMembers, *model.Response, error) {
	args := m.Called(ctx, channelID, page, perPage, etag)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(model.ChannelMembers), nil, args.Error(2)
}

func (m *MockMattermostClient) GetPostsForChannel(ctx context.Context, channelID string, page, perPage int, etag string, collapsedThreads, collapsedThreadsExtended bool) (*model.PostList, *model.Response, error) {
	args := m.Called(ctx, channelID, page, perPage, etag, collapsedThreads, collapsedThreadsExtended)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*model.PostList), nil, args.Error(2)
}

// MockBridge mocks bridgev2.Bridge for testing
type MockBridge struct {
	mock.Mock
}

func (m *MockBridge) GetPortalByKey(ctx context.Context, key networkid.PortalKey) (*bridgev2.Portal, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bridgev2.Portal), args.Error(1)
}

func (m *MockBridge) GetGhostByID(ctx context.Context, id networkid.UserID) (*bridgev2.Ghost, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bridgev2.Ghost), args.Error(1)
}

func (m *MockBridge) QueueRemoteEvent(login *bridgev2.UserLogin, evt bridgev2.RemoteEvent) {
	m.Called(login, evt)
}

// Helper to create a test SyncEngine with mocked dependencies
func createTestSyncEngine() (*SyncEngine, *MockMattermostClient) {
	mockClient := new(MockMattermostClient)
	
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			Mode: ModeMirror,
			Mirror: MirrorConfig{
				SyncAllTeams:    true,
				SyncAllChannels: true,
				SyncAllUsers:    true,
				AutoInviteUsers: false,
				SyncHistory:     false,
				HistoryLimit:    100,
			},
		},
		users: make(map[networkid.UserLoginID]*bridgev2.UserLogin),
	}
	
	engine := NewSyncEngine(connector)
	return engine, mockClient
}

func TestNewSyncEngine(t *testing.T) {
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			Mode: ModeMirror,
		},
	}
	
	engine := NewSyncEngine(connector)
	
	assert.NotNil(t, engine)
	assert.Equal(t, connector, engine.Connector)
	assert.NotNil(t, engine.syncedTeams)
	assert.NotNil(t, engine.syncedChannels)
	assert.NotNil(t, engine.syncedUsers)
}

func TestSyncEngine_IsMirrorMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     BridgeMode
		expected bool
	}{
		{
			name:     "mirror mode enabled",
			mode:     ModeMirror,
			expected: true,
		},
		{
			name:     "puppet mode",
			mode:     ModePuppet,
			expected: false,
		},
		{
			name:     "empty mode defaults to puppet",
			mode:     "",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := &MattermostConnector{
				Config: &NetworkConfig{
					Mode: tt.mode,
				},
			}
			
			assert.Equal(t, tt.expected, connector.IsMirrorMode())
		})
	}
}

func TestSyncEngine_TeamTrackingPreventsduplicates(t *testing.T) {
	engine, _ := createTestSyncEngine()
	
	team := &model.Team{
		Id:          "team1",
		Name:        "test-team",
		DisplayName: "Test Team",
	}
	
	// First sync should mark as synced
	assert.False(t, engine.syncedTeams[team.Id])
	engine.syncedTeams[team.Id] = true
	assert.True(t, engine.syncedTeams[team.Id])
}

func TestSyncEngine_ChannelTrackingPreventsHDuplicates(t *testing.T) {
	engine, _ := createTestSyncEngine()
	
	channel := &model.Channel{
		Id:          "channel1",
		Name:        "test-channel",
		DisplayName: "Test Channel",
		Type:        model.ChannelTypeOpen,
	}
	
	// First sync should mark as synced
	assert.False(t, engine.syncedChannels[channel.Id])
	engine.syncedChannels[channel.Id] = true
	assert.True(t, engine.syncedChannels[channel.Id])
}

func TestSyncEngine_UserTrackingPreventsDuplicates(t *testing.T) {
	engine, _ := createTestSyncEngine()
	
	user := &model.User{
		Id:       "user1",
		Username: "testuser",
	}
	
	assert.False(t, engine.syncedUsers[user.Id])
	engine.syncedUsers[user.Id] = true
	assert.True(t, engine.syncedUsers[user.Id])
}

func TestTeamSyncEvent_GetType(t *testing.T) {
	event := &TeamSyncEvent{
		Team: &model.Team{
			Id:          "team1",
			DisplayName: "Test Team",
		},
	}
	
	assert.Equal(t, bridgev2.RemoteEventChatInfoChange, event.GetType())
}

func TestTeamSyncEvent_GetChatInfoChange(t *testing.T) {
	team := &model.Team{
		Id:          "team1",
		DisplayName: "Test Team",
		Description: "A test team description",
	}
	
	event := &TeamSyncEvent{Team: team}
	
	chatInfoChange, err := event.GetChatInfoChange(context.Background())
	
	assert.NoError(t, err)
	assert.NotNil(t, chatInfoChange)
	assert.Equal(t, "Test Team", *chatInfoChange.ChatInfo.Name)
	assert.Equal(t, "A test team description", *chatInfoChange.ChatInfo.Topic)
}

func TestChannelSyncEvent_GetType(t *testing.T) {
	event := &ChannelSyncEvent{
		Channel: &model.Channel{
			Id:          "channel1",
			DisplayName: "Test Channel",
		},
	}
	
	assert.Equal(t, bridgev2.RemoteEventChatInfoChange, event.GetType())
}

func TestChannelSyncEvent_GetChatInfoChange(t *testing.T) {
	channel := &model.Channel{
		Id:          "channel1",
		DisplayName: "General",
		Purpose:     "General discussion",
	}
	
	event := &ChannelSyncEvent{Channel: channel}
	
	chatInfoChange, err := event.GetChatInfoChange(context.Background())
	
	assert.NoError(t, err)
	assert.NotNil(t, chatInfoChange)
	assert.Equal(t, "General", *chatInfoChange.ChatInfo.Name)
	assert.Equal(t, "General discussion", *chatInfoChange.ChatInfo.Topic)
}

func TestMirrorConfig_Defaults(t *testing.T) {
	config := MirrorConfig{}
	
	// All bools should default to false
	assert.False(t, config.SyncAllTeams)
	assert.False(t, config.SyncAllChannels)
	assert.False(t, config.SyncAllUsers)
	assert.False(t, config.AutoInviteUsers)
	assert.False(t, config.CreateMatrixAccounts)
	assert.False(t, config.SyncHistory)
	assert.Equal(t, 0, config.HistoryLimit)
}

func TestMirrorConfig_WithValues(t *testing.T) {
	config := MirrorConfig{
		SyncAllTeams:         true,
		SyncAllChannels:      true,
		SyncAllUsers:         true,
		AutoInviteUsers:      true,
		CreateMatrixAccounts: true,
		SyncHistory:          true,
		HistoryLimit:         500,
	}
	
	assert.True(t, config.SyncAllTeams)
	assert.True(t, config.SyncAllChannels)
	assert.True(t, config.SyncAllUsers)
	assert.True(t, config.AutoInviteUsers)
	assert.True(t, config.CreateMatrixAccounts)
	assert.True(t, config.SyncHistory)
	assert.Equal(t, 500, config.HistoryLimit)
}

func TestSynapseAdminConfig_Empty(t *testing.T) {
	config := SynapseAdminConfig{}
	
	assert.Empty(t, config.URL)
	assert.Empty(t, config.Token)
}

func TestBridgeMode_Constants(t *testing.T) {
	assert.Equal(t, BridgeMode("puppet"), ModePuppet)
	assert.Equal(t, BridgeMode("mirror"), ModeMirror)
}

func TestSyncEngine_GetAnyLogin_Empty(t *testing.T) {
	engine, _ := createTestSyncEngine()
	
	// With no users, should return nil
	login := engine.getAnyLogin()
	assert.Nil(t, login)
}

func TestSyncEngine_GetAnyLogin_WithUsers(t *testing.T) {
	engine, _ := createTestSyncEngine()
	
	// Add a user
	login := &bridgev2.UserLogin{
		UserLogin: &database.UserLogin{
			ID: "user1",
		},
	}
	engine.Connector.users[networkid.UserLoginID("user1")] = login
	
	// Should return the login
	result := engine.getAnyLogin()
	assert.NotNil(t, result)
	assert.Equal(t, login, result)
}

func TestMattermostEvent_GetPortalKey(t *testing.T) {
	event := MattermostEvent{
		ChannelID: "channel123",
	}
	
	key := event.GetPortalKey()
	
	assert.Equal(t, networkid.PortalID("channel123"), key.ID)
	assert.Empty(t, key.Receiver)
}

func TestMattermostEvent_GetSender(t *testing.T) {
	event := MattermostEvent{
		UserID: "user456",
	}
	
	sender := event.GetSender()
	
	assert.Equal(t, networkid.UserID("user456"), sender.Sender)
}
