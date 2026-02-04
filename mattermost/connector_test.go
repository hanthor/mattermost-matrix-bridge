package mattermost

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/id"
)


func TestMattermostConnector_GetName(t *testing.T) {
	connector := &MattermostConnector{}
	name := connector.GetName()

	assert.Equal(t, "Mattermost", name.DisplayName)
	assert.Equal(t, "mattermost", name.NetworkID)
	assert.Equal(t, "https://mattermost.com", name.NetworkURL)
}

func TestMattermostConnector_GetLoginFlows(t *testing.T) {
	connector := &MattermostConnector{}
	flows := connector.GetLoginFlows()

	assert.Len(t, flows, 1)
	assert.Equal(t, "personal-access-token", flows[0].ID)
	assert.Equal(t, "Personal Access Token", flows[0].Name)
}

func TestMattermostConnector_CreateLogin(t *testing.T) {
	connector := &MattermostConnector{}
	user := &bridgev2.User{
		User: &database.User{
			MXID: id.UserID("test-user"),
		},
	}


	// Test valid flow
	process, err := connector.CreateLogin(context.Background(), user, "personal-access-token")
	assert.NoError(t, err)
	assert.IsType(t, &PATLogin{}, process)

	// Test invalid flow
	_, err = connector.CreateLogin(context.Background(), user, "invalid-flow")
	assert.Error(t, err)
}
