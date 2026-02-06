package mattermost

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

type Client struct {
	model.Client4
	AdminToken string
}

func NewClient(url, adminToken string) *Client {
	c := model.NewAPIv4Client(url)
	c.SetToken(adminToken)
	return &Client{
		Client4:    *c,
		AdminToken: adminToken,
	}
}

func (c *Client) Connect(ctx context.Context) error {
	// Verify connection and admin token
	user, _, err := c.GetMe(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to connect to Mattermost: %w", err)
	}
	isSystemAdmin := false
	for _, role := range strings.Fields(user.Roles) {
		if role == "system_admin" {
			isSystemAdmin = true
			break
		}
	}
	if !isSystemAdmin {
		fmt.Printf("Warning: Admin token user %s is not a system admin (roles: %s)\n", user.Username, user.Roles)
	}
	return nil
}

func (c *Client) GetClient() *model.Client4 {
	return &c.Client4
}

func (c *Client) GetFile(ctx context.Context, fileID string) ([]byte, error) {
	data, _, err := c.Client4.GetFile(ctx, fileID)
	return data, err
}

func (c *Client) UploadFile(ctx context.Context, data []byte, channelID, filename string) (*model.FileInfo, error) {
	resp, _, err := c.Client4.UploadFile(ctx, data, channelID, filename)
	if err != nil {
		return nil, err
	}
	if len(resp.FileInfos) > 0 {
		return resp.FileInfos[0], nil
	}
	return nil, fmt.Errorf("no file info returned")
}
func (c *Client) GetTeam(ctx context.Context, teamID string) (*model.Team, error) {
	team, _, err := c.Client4.GetTeam(ctx, teamID, "")
	return team, err
}

func (c *Client) CreateDirectChannel(ctx context.Context, otherUserID string) (*model.Channel, error) {
	// The first argument is the "other" user ID. The client's own ID is inferred by the server or we might need to pass it?
	// Client4.CreateDirectChannel takes (userId1, userId2).
	// We need the current user's ID. 
	// The wrapper `Client` doesn't strictly store its own ID, but `Connect` fetches "Me".
	// Maybe we should store "Me" in `Client`.
	// Or we just pass both IDs.
	// But `CreateDirectChannel` in `api.go` will be called on a logged-in user's API.
	return nil, fmt.Errorf("use CreateDirectChannelWithBoth instead")
}

func (c *Client) CreateDirectChannelWithBoth(ctx context.Context, userID1, userID2 string) (*model.Channel, error) {
	ch, _, err := c.Client4.CreateDirectChannel(ctx, userID1, userID2)
	return ch, err
}

func (c *Client) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, _, err := c.Client4.GetUserByEmail(ctx, email, "")
	return user, err
}

func (c *Client) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	user, _, err := c.Client4.GetUserByUsername(ctx, username, "")
	return user, err
}

// GetFileInfo retrieves metadata about a file from Mattermost
func (c *Client) GetFileInfo(ctx context.Context, fileID string) (*model.FileInfo, error) {
	info, _, err := c.Client4.GetFileInfo(ctx, fileID)
	return info, err
}

// GetFileWithInfo retrieves both file content and metadata
func (c *Client) GetFileWithInfo(ctx context.Context, fileID string) ([]byte, *model.FileInfo, error) {
	// Get file info first
	info, _, err := c.Client4.GetFileInfo(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Get file content
	data, _, err := c.Client4.GetFile(ctx, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}
	
	return data, info, nil
}

// GetFileThumbnail retrieves a thumbnail for an image file
func (c *Client) GetFileThumbnail(ctx context.Context, fileID string) ([]byte, error) {
	data, _, err := c.Client4.GetFileThumbnail(ctx, fileID)
	return data, err
}

// GetFilePreview retrieves a preview for a file
func (c *Client) GetFilePreview(ctx context.Context, fileID string) ([]byte, error) {
	data, _, err := c.Client4.GetFilePreview(ctx, fileID)
	return data, err
}

// GetTeamIcon retrieves the team icon/avatar
func (c *Client) GetTeamIcon(ctx context.Context, teamID string) ([]byte, error) {
	data, _, err := c.Client4.GetTeamIcon(ctx, teamID, "")
	return data, err
}

// GetTeamsForUser retrieves all teams a user is a member of
func (c *Client) GetTeamsForUser(ctx context.Context, userID string) ([]*model.Team, error) {
	teams, _, err := c.Client4.GetTeamsForUser(ctx, userID, "")
	return teams, err
}

// GetTeamMembers retrieves all members of a team
func (c *Client) GetTeamMembers(ctx context.Context, teamID string, page, perPage int) ([]*model.TeamMember, error) {
	members, _, err := c.Client4.GetTeamMembers(ctx, teamID, page, perPage, "")
	return members, err
}

func (c *Client) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	u, _, err := c.Client4.CreateUser(ctx, user)
	return u, err
}
