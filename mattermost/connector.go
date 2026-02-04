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
)




type NetworkConfig struct {
	ServerURL  string `yaml:"server_url"`
	AdminToken string `yaml:"admin_token"`
}

type MattermostConnector struct {
	Bridge *bridgev2.Bridge
	Config *NetworkConfig
	Client   *Client
	WSClient *model.WebSocketClient
	
	usersLock sync.RWMutex
	users     map[networkid.UserLoginID]*bridgev2.UserLogin
}




func (m *MattermostConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{}
}

func (m *MattermostConnector) GetConfig() (string, any, configupgrade.Upgrader) {
	return "mattermost", &NetworkConfig{}, nil
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
}






func (m *MattermostConnector) Start(ctx context.Context) error {
	m.Client = NewClient(m.Config.ServerURL, m.Config.AdminToken)
	err := m.Client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Mattermost: %w", err)
	}

	m.StartWebSocket()
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





