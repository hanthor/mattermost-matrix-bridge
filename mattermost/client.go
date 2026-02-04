package mattermost

import (
	"context"
	"fmt"

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
	if user.Roles != "system_admin" {
		// We might not strictly need system_admin if we have enough permissions,
		// but for puppeting it's usually required.
		fmt.Printf("Warning: Admin token user %s is not a system admin\n", user.Username)
	}
	return nil
}
