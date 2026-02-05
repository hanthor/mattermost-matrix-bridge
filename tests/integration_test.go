package tests

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	
	"github.com/hanthor/mautrix-mattermost/mattermost"
	"github.com/mattermost/mattermost/server/public/model"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridge/status"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/bridgeconfig"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// MockMatrixConnector implements bridgev2.MatrixConnector for testing
type MockMatrixConnector struct {
	SentEvents []event.Event
}

func (m *MockMatrixConnector) GetCapabilities() *bridgev2.MatrixCapabilities {
	return &bridgev2.MatrixCapabilities{}
}

func (m *MockMatrixConnector) Init(br *bridgev2.Bridge) {}
func (m *MockMatrixConnector) Start(ctx context.Context) error { return nil }
func (m *MockMatrixConnector) Stop() {}

func (m *MockMatrixConnector) SendMessage(ctx context.Context, roomID id.RoomID, content event.MessageEventContent) (*id.EventID, error) {
	evtID := id.EventID(fmt.Sprintf("$fake:%d", time.Now().UnixNano()))
	m.SentEvents = append(m.SentEvents, event.Event{
		Type:    event.EventMessage,
		Content: event.Content{Parsed: &content},
		ID:      evtID,
	})
	return &evtID, nil
}

func (m *MockMatrixConnector) SendBridgeStatus(ctx context.Context, state *status.BridgeState) error { return nil }
func (m *MockMatrixConnector) SendMessageStatus(ctx context.Context, status *bridgev2.MessageStatus, evt *bridgev2.MessageStatusEventInfo) {}
func (m *MockMatrixConnector) ParseGhostMXID(userID id.UserID) (networkid.UserID, bool) { return "", false }
func (m *MockMatrixConnector) GhostIntent(userID networkid.UserID) bridgev2.MatrixAPI { return nil }
func (m *MockMatrixConnector) NewUserIntent(ctx context.Context, userID id.UserID, accessToken string) (bridgev2.MatrixAPI, string, error) { return nil, "", nil }
func (m *MockMatrixConnector) GenerateDeterministicEventID(roomID id.RoomID, portalKey networkid.PortalKey, messageID networkid.MessageID, partID networkid.PartID) id.EventID {
    return id.EventID(fmt.Sprintf("$%s", messageID))
}
func (m *MockMatrixConnector) GenerateReactionEventID(roomID id.RoomID, targetMessage *database.Message, sender networkid.UserID, emojiID networkid.EmojiID) id.EventID {
    return id.EventID(fmt.Sprintf("$%s", emojiID))
}
func (m *MockMatrixConnector) ServerName() string { return "test" }

// Stubs for other MatrixConnector methods...
func (m *MockMatrixConnector) GetPowerLevels(ctx context.Context, roomID id.RoomID) (*event.PowerLevelsEventContent, error) { return nil, nil }
func (m *MockMatrixConnector) GetMembers(ctx context.Context, roomID id.RoomID) (map[id.UserID]*event.MemberEventContent, error) { return nil, nil }
func (m *MockMatrixConnector) GetMemberInfo(ctx context.Context, roomID id.RoomID, userID id.UserID) (*event.MemberEventContent, error) { return nil, nil }
func (m *MockMatrixConnector) IsGhost(userID id.UserID) bool { return false }
func (m *MockMatrixConnector) GetGhost(userID id.UserID) *bridgev2.Ghost { return nil }
func (m *MockMatrixConnector) BatchSend(ctx context.Context, roomID id.RoomID, req *mautrix.ReqBeeperBatchSend, extra []*bridgev2.MatrixSendExtra) (*mautrix.RespBeeperBatchSend, error) { return nil, nil }
func (m *MockMatrixConnector) GenerateContentURI(ctx context.Context, mediaID networkid.MediaID) (id.ContentURIString, error) { return "", nil }

func (m *MockMatrixConnector) BotIntent() bridgev2.MatrixAPI {
	return nil
}



type TestCommandProcessor struct{}
func (p *TestCommandProcessor) Handle(ctx context.Context, roomID id.RoomID, eventID id.EventID, user *bridgev2.User, message string, replyTo id.EventID) {}

func TestIntegration_MattermostMirroring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start Mattermost Container
	req := testcontainers.ContainerRequest{
		Image:        "mattermost/mattermost-preview:latest",
		ExposedPorts: []string{"8065/tcp"},
		WaitingFor:   wait.ForHTTP("/api/v4/system/ping").WithPort("8065/tcp").WithStartupTimeout(2 * time.Minute),
		Env: map[string]string{
			"MM_SERVICESETTINGS_SITEURL": "http://localhost:8065",
			"MM_SERVICESETTINGS_ENABLELOCALMODE": "true",
			"MM_SERVICESETTINGS_LOCALMODESOCKETLOCATION": "/var/tmp/mattermost_local.socket",
			"MM_SERVICESETTINGS_ENABLEUSERACCESSTOKENS": "true",
		},
	}
	
	t.Log("Starting Mattermost container...")
	mmContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	defer mmContainer.Terminate(ctx)

	// Get Host/Port
	host, err := mmContainer.Host(ctx)
	require.NoError(t, err)
	port, err := mmContainer.MappedPort(ctx, "8065")
	require.NoError(t, err)
	mmURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	t.Logf("Mattermost running at %s", mmURL)

	// Generate Admin Token (Executing inside container)
	// Note: mattermost-preview automatically creates an admin user usually, but better to generate a fresh token via CLI.
	// We need to wait a bit more for the socket/CLI to be ready?
	time.Sleep(5 * time.Second) 
	
	// Create admin user via CLI if needed or just generate token for 'sysadmin' (default in preview)
	// mmctl user create --email admin@example.com --username admin --password password123 --system_admin ??
	// Actually preview image has 'sysadmin' / 'Sys@dmin-sample1' usually? 
	// Let's rely on `mmctl` to generate token for existing user or create one.
	
	// Try creating a test admin
	_, _, err = mmContainer.Exec(ctx, []string{"/mm/mattermost/bin/mmctl", "user", "create", 
		"--email", "testadmin@example.com", "--username", "testadmin", "--password", "TestPass123!", "--system-admin", "--local"})
	
	// Ignore error if user exists?
	
	// Generate token
	code, out, err := mmContainer.Exec(ctx, []string{"/mm/mattermost/bin/mmctl", "token", "generate", "testadmin", "bridgetest", "--local"})
	require.NoError(t, err)
	outBytes, _ := io.ReadAll(out)
	t.Logf("Token Generation Output: %q", string(outBytes))
	require.Equal(t, 0, code, "Failed to generate token: %s", string(outBytes))
	
	// Extract token using regex looking for 26-char alphanumeric followed by ": bridgetest"
	re := regexp.MustCompile(`([a-z0-9]{26}): bridgetest`)
	matches := re.FindStringSubmatch(string(outBytes))
	require.Len(t, matches, 2, "Could not extract admin token from output: %q", string(outBytes))
	token := matches[1]

	t.Logf("Got Admin Token: %s", token)

	// Remove existing DB to ensure clean state
	os.Remove("integration_test.db")

	// Initialize Bridge with file-based DB to avoid memory cache issues
	db, err := dbutil.NewFromConfig("test", dbutil.Config{
		PoolConfig: dbutil.PoolConfig{
			Type: "sqlite3",
			URI:  "file:integration_test.db",
		},
	}, dbutil.ZeroLogger(zerolog.New(os.Stdout)))
	require.NoError(t, err)

	mockMatrix := &MockMatrixConnector{}
	mmConnector := &mattermost.MattermostConnector{}

	// Create a minimal config
	cfg := &bridgeconfig.Config{
		Bridge: bridgeconfig.BridgeConfig{
			Permissions: bridgeconfig.PermissionConfig{
				"*": &bridgeconfig.Permissions{
					Admin: true,
				},
			},
		},
	}
	// Manually inject config into connector since we aren't loading from file
	mmConnector.Config = &mattermost.NetworkConfig{
		ServerURL:  mmURL,
		AdminToken: token,
	}

	// Use stdout logger for debugging
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	br := bridgev2.NewBridge("test", db, log, &cfg.Bridge, mockMatrix, mmConnector, func(*bridgev2.Bridge) bridgev2.CommandProcessor {
		return &TestCommandProcessor{}
	})
	
	// Create a dummy user and login in DB so bridge processes events
	ulCtx := context.Background()
	// Upgrade DB first to create tables
	err = br.DB.Upgrade(ulCtx)
	require.NoError(t, err)

	user := &database.User{
		BridgeID: "test",
		MXID:     id.UserID("@admin:example.com"),
	}
	err = br.DB.User.Insert(ulCtx, user)
	require.NoError(t, err)

	login := &database.UserLogin{
		BridgeID: "test",
		ID:       networkid.UserLoginID("test-admin"),
		UserMXID: user.MXID,
		Metadata: map[string]any{"token": token},
	}
	err = br.DB.UserLogin.Insert(ulCtx, login)
	require.NoError(t, err)

	// Start Bridge
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := br.Start()
		if err != nil {
			t.Logf("Bridge stopped: %v", err)
		}
	}()
	defer br.Stop()

	// Wait for bridge to be ready (WS connected)
	time.Sleep(2 * time.Second)

	// Test: Send message to MM API using token
	// We use the Channel ID from default 'Town Square' or create one?
	// Mattermost preview usually has a default team and channel. 
	// Let's create a channel first via API to be safe.
	
	// Quick API helper
	mmClient := mattermost.NewClient(mmURL, token)
	err = mmClient.Connect(ctx)
	require.NoError(t, err)

	// Create Team (or find existing) - skipping for simplicity, assuming default team or System Admin can post anywhere?
	// Finding default team
	// We can use the client to fetch teams.
	// But `client.go` might not have `GetAllTeams`.
	// For integration test, let's just make a raw HTTP call or use the client if it has it.
	// Our `client.go` has `GetMe` and `GetChannel`.
	
	// Verify connection
	err = mmClient.Connect(ctx)
	require.NoError(t, err)

	// Get or Create Team
	team, _, err := mmClient.GetTeamByName(ctx, "test-team", "")
	if err != nil {
		t.Log("Creating test-team")
		team, _, err = mmClient.CreateTeam(ctx, &model.Team{
			Name:        "test-team",
			DisplayName: "Test Team",
			Type:        model.TeamOpen,
		})
		require.NoError(t, err, "Failed to create team")
	}

	// Get or Create Channel
	channel, _, err := mmClient.GetChannelByName(ctx, "test-channel", team.Id, "")
	if err != nil {
		t.Log("Creating test-channel")
		channel, _, err = mmClient.CreateChannel(ctx, &model.Channel{
			TeamId:      team.Id,
			Name:        "test-channel",
			DisplayName: "Test Channel",
			Type:        model.ChannelTypeOpen,
		})
		require.NoError(t, err, "Failed to create channel")
	}

	t.Logf("Found Channel ID: %s", channel.Id)
	
	// Create post via Client
	t.Log("Sending test post via Client...")
	testMsg := fmt.Sprintf("Hello Bridge %d", time.Now().Unix())
	post, _, err := mmClient.CreatePost(ctx, &model.Post{
		ChannelId: channel.Id,
		Message:   testMsg,
	})
	require.NoError(t, err, "Failed to create post")
	t.Logf("Created Post ID: %s", post.Id)

	// Verify: mockMatrix.SentEvents has the event
	assert.Eventually(t, func() bool {
		for _, evt := range mockMatrix.SentEvents {
			if evt.Content.Parsed != nil {
				content, ok := evt.Content.Parsed.(*event.MessageEventContent)
				if ok && content.Body == testMsg {
					return true
				}
			}
		}
		return false
	}, 10*time.Second, 500*time.Millisecond, "Did not receive bridged message in Matrix")
}

