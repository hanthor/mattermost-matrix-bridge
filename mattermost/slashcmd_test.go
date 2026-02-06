package mattermost

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlashCommandHandler_Help(t *testing.T) {
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			ServerURL: "http://test.mattermost.com",
			Mode:      ModeMirror,
		},
	}
	handler := NewSlashCommandHandler(connector, "test-token")

	form := url.Values{}
	form.Set("token", "test-token")
	form.Set("user_id", "user123")
	form.Set("user_name", "testuser")
	form.Set("channel_id", "channel123")
	form.Set("text", "help")

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Matrix Bridge Commands")
	assert.Contains(t, rr.Body.String(), "/matrix help")
}

func TestSlashCommandHandler_InvalidToken(t *testing.T) {
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			ServerURL: "http://test.mattermost.com",
		},
	}
	handler := NewSlashCommandHandler(connector, "correct-token")

	form := url.Values{}
	form.Set("token", "wrong-token")
	form.Set("text", "help")

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSlashCommandHandler_Status(t *testing.T) {
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			ServerURL: "http://test.mattermost.com",
			Mode:      ModeMirror,
		},
	}
	handler := NewSlashCommandHandler(connector, "")

	form := url.Values{}
	form.Set("text", "status")

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Matrix Bridge Status")
	assert.Contains(t, rr.Body.String(), "mirror")
}

func TestSlashCommandHandler_JoinMissingArg(t *testing.T) {
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			ServerURL: "http://test.mattermost.com",
		},
	}
	handler := NewSlashCommandHandler(connector, "")

	form := url.Values{}
	form.Set("text", "join")

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Usage:")
}

func TestSlashCommandHandler_UnknownCommand(t *testing.T) {
	connector := &MattermostConnector{
		Config: &NetworkConfig{
			ServerURL: "http://test.mattermost.com",
		},
	}
	handler := NewSlashCommandHandler(connector, "")

	form := url.Values{}
	form.Set("text", "foobar")

	req := httptest.NewRequest(http.MethodPost, "/mattermost/command", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Unknown subcommand")
}
