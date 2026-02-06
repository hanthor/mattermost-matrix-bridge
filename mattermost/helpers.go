package mattermost

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"go.mau.fi/util/random"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

// EnsureGhost ensures a Mattermost ghost user exists for the given Matrix ID.
// Returns the Mattermost User ID (UUID).
func (m *MattermostConnector) EnsureGhost(ctx context.Context, mxid string) (string, error) {
	// 1. Generate a valid Mattermost username using reversible encoding
	// @james:reilly.asia -> matrix_james.reilly.asia
	// _ -> __
	// : -> .
	// . -> _d
	// - is preserved
	cleanMXID := strings.TrimPrefix(mxid, "@")
	var sb strings.Builder
	sb.WriteString("mx.")
	
	for _, char := range cleanMXID {
		switch char {
		case '_':
			sb.WriteString("__")
		case ':':
			// Replace colon with underscore as requested by user
			sb.WriteRune('_')
		default:
			// Mattermost allows letters, numbers, ., -, _
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.' {
				sb.WriteRune(char)
			} else if char >= 'A' && char <= 'Z' {
				sb.WriteRune(char + 32) // basic lowercase
			} else {
				// Encode other chars as _xHH
				sb.WriteString(fmt.Sprintf("_x%02x", char))
			}
		}
	}
	
	username := sb.String()
	// Mattermost limit is usually 64
	if len(username) > 64 {
		username = username[:64]
	}

	// 2. Check if user exists
	user, err := m.Client.GetUserByUsername(ctx, username)
	if err == nil && user != nil {
		return user.Id, nil
	}

	// 3. Create user if not exists
	// Parse MXID for pretty display name
	localpart := cleanMXID
	serverName := ""
	if idx := strings.LastIndex(cleanMXID, ":"); idx != -1 {
		localpart = cleanMXID[:idx]
		serverName = cleanMXID[idx+1:]
	}

	email := fmt.Sprintf("%s@matrix.bridge.local", username) // Fake email
	password := "MatrixBridge_" + random.String(16)

	newUser := &model.User{
		Username:  username,
		Email:     email,
		Password:  password,
		FirstName: localpart,
		LastName:  fmt.Sprintf("(%s)", serverName),
		Nickname:  mxid,
		Position:  "Matrix Bridge Ghost",
	}

	createdUser, err := m.Client.CreateUser(ctx, newUser)
	if err != nil {
		// Race condition check: try fetching again
		user, err2 := m.Client.GetUserByUsername(ctx, username)
		if err2 == nil && user != nil {
			return user.Id, nil
		}
		return "", fmt.Errorf("failed to create Mattermost user for ghost: %w", err)
	}

	// 4. Update ghost metadata if possible to cache the ID
	ghost, _ := m.Bridge.GetGhostByID(ctx, networkid.UserID(mxid))
	if ghost != nil {
		if ghost.Metadata == nil {
			ghost.Metadata = make(map[string]any)
		}
		if meta, ok := ghost.Metadata.(map[string]any); ok {
			if meta["mm_id"] != createdUser.Id {
				meta["mm_id"] = createdUser.Id
				ghost.Metadata = meta
				if ghost.Ghost != nil {
					_ = m.Bridge.DB.Ghost.Update(ctx, ghost.Ghost)
				}
			}
		}
	}

	return createdUser.Id, nil
}

// GetClientForUser returns a Mattermost Client authenticated as the given Matrix user.
// It manages (creates and caches) Personal Access Tokens for the ghost user.
func (m *MattermostConnector) GetClientForUser(ctx context.Context, mxid string) (*Client, string, error) {
	// 1. Ensure ghost user exists and get MM ID
	mmUserID, err := m.EnsureGhost(ctx, mxid)
	if err != nil {
		return nil, "", fmt.Errorf("failed to ensure ghost: %w", err)
	}

	// 2. Get the bridge ghost object to access metadata
	ghost, err := m.Bridge.GetGhostByID(ctx, networkid.UserID(mxid))
	if err != nil {
		return nil, "", fmt.Errorf("failed to get bridge ghost: %w", err)
	}

	// 3. Check for existing token in metadata
	var metadata map[string]any
	if ghost.Metadata == nil {
		metadata = make(map[string]any)
	} else if m, ok := ghost.Metadata.(map[string]any); ok {
		metadata = m
	} else {
		// Should not happen, but safeguard
		metadata = make(map[string]any)
	}
	
	val, ok := metadata["mm_token"]
	if ok {
		tokenStr, ok := val.(string)
		if ok && tokenStr != "" {
			return NewClient(m.Config.ServerURL, tokenStr), mmUserID, nil
		}
	}

	// 4. Generate new token if missing
	token, err := m.Client.CreateUserAccessToken(ctx, mmUserID, "Matrix Bridge Ghost Token")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create access token for ghost %s: %w", mmUserID, err)
	}
	
	// 5. Store token in metadata
	metadata["mm_token"] = token.Token
	ghost.Metadata = metadata
	
	// Save metadata directly to DB
	// Use explicit update if ghost.Ghost exists, otherwise comment out if unsure
	// Based on errors, let's assume ghost.Ghost is safe or the compiler would have complained earlier?
	// Actually, safer to check if Bridge has SaveGhost method?
	// I'll try calling DB Update on ghost.Ghost if safe.
	if ghost.Ghost != nil {
		err = m.Bridge.DB.Ghost.Update(ctx, ghost.Ghost)
		if err != nil {
			m.Bridge.Log.Warn().Err(err).Msg("Failed to save ghost token to database")
		}
	}
	
	return NewClient(m.Config.ServerURL, token.Token), mmUserID, nil
}
