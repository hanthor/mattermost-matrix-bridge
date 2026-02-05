package mattermost

import (
	"context"
	"fmt"
	"sync"

	"go.mau.fi/util/configupgrade"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/hanthor/mautrix-mattermost/mattermost/msgconv"
	_ "embed"
	"time"
)

//go:embed example-config.yaml
var ExampleConfig string

// BridgeMode defines the operating mode of the bridge
type BridgeMode string

const (
	// ModePuppet is the traditional single-user bridging mode (like other Beeper bridges)
	ModePuppet BridgeMode = "puppet"
	// ModeMirror is full server mirroring with admin API access
	ModeMirror BridgeMode = "mirror"
)

// MirrorConfig contains settings for mirror mode
type MirrorConfig struct {
	SyncAllTeams         bool `yaml:"sync_all_teams"`
	SyncAllChannels      bool `yaml:"sync_all_channels"`
	SyncAllUsers         bool `yaml:"sync_all_users"`
	AutoInviteUsers      bool `yaml:"auto_invite_users"`
	CreateMatrixAccounts bool `yaml:"create_matrix_accounts"`
	SyncHistory          bool `yaml:"sync_history"`
	HistoryLimit         int  `yaml:"history_limit"`
}

// SynapseAdminConfig contains Synapse admin API settings
type SynapseAdminConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type NetworkConfig struct {
	ServerURL    string             `yaml:"server_url"`
	AdminToken   string             `yaml:"admin_token"`
	Mode         BridgeMode         `yaml:"mode"`
	Mirror       MirrorConfig       `yaml:"mirror"`
	SynapseAdmin SynapseAdminConfig `yaml:"synapse_admin"`
}

type MattermostConnector struct {
	Bridge *bridgev2.Bridge
	Config *NetworkConfig
	Client   *Client
	WSClient *model.WebSocketClient
	MsgConv  *msgconv.MessageConverter
	
	usersLock sync.RWMutex
	users     map[networkid.UserLoginID]*bridgev2.UserLogin
}




func (m *MattermostConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{}
}

func (m *MattermostConnector) GetConfig() (string, any, configupgrade.Upgrader) {
	return ExampleConfig, &m.Config, configupgrade.SimpleUpgrader(m.UpgradeConfig)
}

func (m *MattermostConnector) UpgradeConfig(helper configupgrade.Helper) {
	helper.Copy(configupgrade.Str, "server_url")
	helper.Copy(configupgrade.Str, "admin_token")
	helper.Copy(configupgrade.Str, "mode")
	
	// Mirror mode settings
	helper.Copy(configupgrade.Bool, "mirror", "sync_all_teams")
	helper.Copy(configupgrade.Bool, "mirror", "sync_all_channels")
	helper.Copy(configupgrade.Bool, "mirror", "sync_all_users")
	helper.Copy(configupgrade.Bool, "mirror", "auto_invite_users")
	helper.Copy(configupgrade.Bool, "mirror", "create_matrix_accounts")
	helper.Copy(configupgrade.Bool, "mirror", "sync_history")
	helper.Copy(configupgrade.Int, "mirror", "history_limit")
	
	// Synapse admin settings
	helper.Copy(configupgrade.Str, "synapse_admin", "url")
	helper.Copy(configupgrade.Str, "synapse_admin", "token")
}

// IsMirrorMode returns true if the bridge is running in mirror mode
func (m *MattermostConnector) IsMirrorMode() bool {
	return m.Config != nil && m.Config.Mode == ModeMirror
}




func (m *MattermostConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{}
}


func (m *MattermostConnector) GetName() bridgev2.BridgeName {
	return bridgev2.BridgeName{
		DisplayName:      "Mattermost",
		NetworkID:        "mattermost",
		BeeperBridgeType: "mattermost",
		NetworkURL:       "https://mattermost.com",
	}
}



func (m *MattermostConnector) Init(br *bridgev2.Bridge) {
	m.Bridge = br
	m.users = make(map[networkid.UserLoginID]*bridgev2.UserLogin)
	m.MsgConv = msgconv.New(br)
}






func (m *MattermostConnector) Start(ctx context.Context) error {
	// Log bridge mode
	mode := m.Config.Mode
	if mode == "" {
		mode = ModePuppet // Default to puppet mode
	}
	fmt.Printf("INFO: Starting Mattermost bridge in %s mode\n", mode)
	
	m.Client = NewClient(m.Config.ServerURL, m.Config.AdminToken)
	err := m.Client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Mattermost: %w", err)
	}

	m.StartWebSocket()
	
	// Mirror mode: start server sync engine
	if m.IsMirrorMode() {
		fmt.Printf("INFO: Mirror mode enabled - will sync all teams/channels/users\n")
		go m.startMirrorSync(ctx)
	}
	
	// Auto-login sysadmin if no users are logged in
	go func() {
		time.Sleep(2 * time.Second)
		m.usersLock.RLock()
		userCount := len(m.users)
		m.usersLock.RUnlock()
		
		if userCount == 0 && m.Config.AdminToken != "" {
			fmt.Printf("DEBUG: Auto-provisioning sysadmin login\n")
			me, _, err := m.Client.GetMe(ctx, "")
			if err == nil {
				// Get or create the user via the bridge's API
				user, err := m.Bridge.GetUserByMXID(ctx, "@admin:localhost")
				if err != nil {
					fmt.Printf("DEBUG: Failed to get user: %v\n", err)
					return
				}
				
				// Create login via bridge's user management
				loginID := networkid.UserLoginID(me.Id)
				login, err := user.NewLogin(ctx, &database.UserLogin{
					ID:         loginID,
					BridgeID:   m.Bridge.ID,
					UserMXID:   user.MXID,
					RemoteName: me.Username,
					Metadata: map[string]any{
						"token": m.Config.AdminToken,
					},
				}, nil)
				if err != nil {
					fmt.Printf("DEBUG: Failed to create login: %v\n", err)
					return
				}
				
				err = m.LoadUserLogin(ctx, login)
				if err != nil {
					fmt.Printf("DEBUG: Failed to load auto-login: %v\n", err)
				} else {
					fmt.Printf("DEBUG: Successfully auto-provisioned sysadmin login\n")
				}
			} else {
				fmt.Printf("DEBUG: Failed to get sysadmin info for auto-login: %v\n", err)
			}
		}
	}()
	
	return nil
}




func (m *MattermostConnector) Stop() {
	// Stop background processes
}

func (m *MattermostConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	m.usersLock.Lock()
	m.users[login.ID] = login
	m.usersLock.Unlock()

	api, err := m.NewNetworkAPI(login)
	if err != nil {
		return err
	}
	login.Client = api
	return nil
}



func (m *MattermostConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{
		{
			ID: "personal-access-token",
			Name: "Personal Access Token",
			Description: "Login using a Mattermost Personal Access Token",
		},
	}
}

func (m *MattermostConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	if flowID == "personal-access-token" {
		return &PATLogin{
			user:      user,
			connector: m,
		}, nil

	}
	return nil, fmt.Errorf("unknown login flow ID: %s", flowID)
}


func (m *MattermostConnector) GetBridgeInfoVersion() (info, capabilities int) {
	return 0, 0
}

func (m *MattermostConnector) NewNetworkAPI(login *bridgev2.UserLogin) (bridgev2.NetworkAPI, error) {
	fmt.Printf("DEBUG: NewNetworkAPI called for login %s\n", login.ID)
	api := &MattermostAPI{
		Login:     login,
		Connector: m,
	}

	m.usersLock.Lock()
	m.users[login.ID] = login
	m.usersLock.Unlock()

	if login != nil {

		meta, ok := login.Metadata.(map[string]any)
		if ok {
			if token, ok := meta["token"].(string); ok && token != "" {
				api.Client = NewClient(m.Config.ServerURL, token)
			}
		}
	}

	if api.Client == nil {
		api.Client = m.Client
	}
	return api, nil
}

func (m *MattermostConnector) GetUsers() []*bridgev2.UserLogin {
	m.usersLock.RLock()
	defer m.usersLock.RUnlock()
	var users []*bridgev2.UserLogin
	for _, u := range m.users {
		users = append(users, u)
	}
	return users
}





