package mattermost

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
)

func TestNewMatrixAdminClient(t *testing.T) {
	client := NewMatrixAdminClient("https://matrix.example.com", "admin_token")
	
	assert.NotNil(t, client)
	assert.Equal(t, "https://matrix.example.com", client.BaseURL)
	assert.Equal(t, "admin_token", client.AdminToken)
	assert.NotNil(t, client.HTTPClient)
}

func TestGenerateMatrixUserID(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		serverName string
		expected   string
	}{
		{
			name:       "simple username",
			username:   "johndoe",
			serverName: "example.com",
			expected:   "@johndoe:example.com",
		},
		{
			name:       "username with dots",
			username:   "john.doe",
			serverName: "matrix.org",
			expected:   "@john.doe:matrix.org",
		},
		{
			name:       "username with underscores",
			username:   "john_doe",
			serverName: "test.server",
			expected:   "@john_doe:test.server",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{
				Id:       "user123",
				Username: tt.username,
			}
			
			mxid := GenerateMatrixUserID(user, tt.serverName)
			
			assert.Equal(t, tt.expected, string(mxid))
		})
	}
}

func TestGeneratePassword(t *testing.T) {
	password := GeneratePassword()
	
	assert.NotEmpty(t, password)
	assert.True(t, len(password) > 20) // "mattermost-bridge-" prefix + 16 chars
	assert.Contains(t, password, "mattermost-bridge-")
}

func TestCreateUserRequest_Marshal(t *testing.T) {
	req := CreateUserRequest{
		Password:    "secret123",
		DisplayName: "John Doe",
		Admin:       false,
		Deactivated: false,
	}
	
	assert.Equal(t, "secret123", req.Password)
	assert.Equal(t, "John Doe", req.DisplayName)
	assert.False(t, req.Admin)
	assert.False(t, req.Deactivated)
}

func TestThreePID(t *testing.T) {
	pid := ThreePID{
		Medium:  "email",
		Address: "john@example.com",
	}
	
	assert.Equal(t, "email", pid.Medium)
	assert.Equal(t, "john@example.com", pid.Address)
}
