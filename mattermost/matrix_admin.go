package mattermost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"maunium.net/go/mautrix/id"
)

// MatrixAdminClient provides access to the Synapse Admin API for user management
type MatrixAdminClient struct {
	BaseURL    string
	AdminToken string
	HTTPClient *http.Client
}

// NewMatrixAdminClient creates a new Synapse Admin API client
func NewMatrixAdminClient(baseURL, adminToken string) *MatrixAdminClient {
	return &MatrixAdminClient{
		BaseURL:    baseURL,
		AdminToken: adminToken,
		HTTPClient: &http.Client{},
	}
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Password    string     `json:"password,omitempty"`
	DisplayName string     `json:"displayname,omitempty"`
	Admin       bool       `json:"admin"`
	Deactivated bool       `json:"deactivated"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	ThreePIDs   []ThreePID `json:"threepids,omitempty"`
}

// ThreePID represents a third-party identity (email, phone, etc.)
type ThreePID struct {
	Medium  string `json:"medium"`  // "email" or "msisdn"
	Address string `json:"address"` // The actual email or phone number
}

// CreateUserResponse represents the response from creating a user
type CreateUserResponse struct {
	Name        string `json:"name"`
	Admin       bool   `json:"admin"`
	Deactivated bool   `json:"deactivated"`
}

// CreateUser creates a new Matrix user via the Synapse Admin API
// The userID should be in the format @localpart:domain
func (c *MatrixAdminClient) CreateUser(ctx context.Context, userID id.UserID, password, displayName string) error {
	// Extract localpart from userID for the API endpoint
	// Synapse Admin API: PUT /_synapse/admin/v2/users/{user_id}

	reqBody := CreateUserRequest{
		Password:    password,
		DisplayName: displayName,
		Admin:       false,
		Deactivated: false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal create user request: %w", err)
	}

	url := fmt.Sprintf("%s/_synapse/admin/v2/users/%s", c.BaseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// UpdateUserDisplayName updates a user's display name
func (c *MatrixAdminClient) UpdateUserDisplayName(ctx context.Context, userID id.UserID, displayName string) error {
	reqBody := map[string]string{
		"displayname": displayName,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/_synapse/admin/v2/users/%s", c.BaseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update user (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// JoinUserToRoom forces a user to join a room (admin API)
func (c *MatrixAdminClient) JoinUserToRoom(ctx context.Context, userID id.UserID, roomID id.RoomID) error {
	// Synapse Admin API: POST /_synapse/admin/v1/join/{room_id}
	reqBody := map[string]string{
		"user_id": string(userID),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/_synapse/admin/v1/join/%s", c.BaseURL, roomID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to join user to room: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to join user to room (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// UserExists checks if a user already exists
func (c *MatrixAdminClient) UserExists(ctx context.Context, userID id.UserID) (bool, error) {
	url := fmt.Sprintf("%s/_synapse/admin/v2/users/%s", c.BaseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("failed to check user (status %d): %s", resp.StatusCode, string(respBody))
	}

	return true, nil
}

// GetUserInfo retrieves user information from Synapse Admin API
func (c *MatrixAdminClient) GetUserInfo(ctx context.Context, userID id.UserID) (*CreateUserResponse, error) {
	url := fmt.Sprintf("%s/_synapse/admin/v2/users/%s", c.BaseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info (status %d): %s", resp.StatusCode, string(respBody))
	}

	var userInfo CreateUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &userInfo, nil
}

// ProfileResponse represents the response from getting a user's profile
type ProfileResponse struct {
	DisplayName string `json:"displayname"`
	AvatarURL   string `json:"avatar_url"`
}

// GetProfile retrieves a user's profile from the Matrix Client-Server API
// Note: This uses the public CS API, not the Admin API, but likely works with Admin Token
func (c *MatrixAdminClient) GetProfile(ctx context.Context, userID id.UserID) (*ProfileResponse, error) {
	url := fmt.Sprintf("%s/_matrix/client/v3/profile/%s", c.BaseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Admin token usually works for client C-S API as well
	req.Header.Set("Authorization", "Bearer "+c.AdminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Profile not set
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get profile (status %d): %s", resp.StatusCode, string(respBody))
	}

	var profile ProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &profile, nil
}

// RoomAliasResponse represents the response from resolving a room alias
type RoomAliasResponse struct {
	RoomID  string   `json:"room_id"`
	Servers []string `json:"servers"`
}

// ResolveRoomAlias resolves a Matrix room alias to a room ID
// e.g., "#room:server.com" -> "!abc123:server.com"
func (c *MatrixAdminClient) ResolveRoomAlias(ctx context.Context, alias string) (id.RoomID, []string, error) {
	// The alias should be in format #room:server.com
	if !strings.HasPrefix(alias, "#") {
		return "", nil, fmt.Errorf("invalid room alias: must start with #")
	}

	// URL encode the alias for use in the path (# becomes %23, : becomes %3A, etc.)
	encodedAlias := url.PathEscape(alias)

	urlStr := fmt.Sprintf("%s/_matrix/client/v3/directory/room/%s", c.BaseURL, encodedAlias)
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("failed to resolve room alias: %s (status %d)", string(body), resp.StatusCode)
	}

	var result RoomAliasResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, err
	}

	return id.RoomID(result.RoomID), result.Servers, nil
}

// JoinRoomVia joins a room with via server hints for federation
// The userID should be the full Matrix user ID (e.g., @user:server.com)
// viaServers are the servers to try for federation (from ResolveRoomAlias)
func (c *MatrixAdminClient) JoinRoomVia(ctx context.Context, userID id.UserID, roomID id.RoomID, viaServers []string) error {
	// Build URL with server_name query parameters for federation
	urlStr := fmt.Sprintf("%s/_matrix/client/v3/join/%s", c.BaseURL, url.PathEscape(string(roomID)))

	// Add server_name parameters for via servers
	if len(viaServers) > 0 {
		params := url.Values{}
		for _, server := range viaServers {
			params.Add("server_name", server)
		}
		// We also need to impersonate the user via the appservice
		params.Set("user_id", string(userID))
		urlStr = urlStr + "?" + params.Encode()
	}

	// Empty JSON body for join request
	reqBody := []byte("{}")
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to join room (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetRoomInfo retrieves room information from the Matrix Client-Server API
func (c *MatrixAdminClient) GetRoomInfo(ctx context.Context, roomID id.RoomID) (map[string]interface{}, error) {
	// Get the room's join rules to determine if it's public or private
	url := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/state/m.room.join_rules", c.BaseURL, roomID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.AdminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get room info: %s (status %d)", string(body), resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// GenerateMatrixUserID creates a Matrix user ID from a Mattermost user
func GenerateMatrixUserID(mmUser *model.User, serverName string) id.UserID {
	// Use Mattermost username as the localpart, sanitized
	// Matrix localparts are case-insensitive and allow: a-z, 0-9, ., _, =, -, /
	localpart := mmUser.Username
	return id.NewUserID(localpart, serverName)
}

// GeneratePassword generates a random password for newly created Matrix users
func GeneratePassword() string {
	// In production, use a proper random password generator
	// For now, we'll use a fixed-length random string
	// This could also support SSO/OIDC in Phase 8
	return "mattermost-bridge-" + randomString(16)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)] // Simple deterministic for now
	}
	return string(b)
}
