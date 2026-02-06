package mattermost

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/id"
)

func (m *MattermostAPI) UpdateGhost(ctx context.Context, ghost *bridgev2.Ghost) error {
	// Get the Mattermost User ID for the ghost
	mmUserID := m.getMMID(ctx, ghost.ID)
	
	m.Connector.Bridge.Log.Info().Str("mm_user_id", mmUserID).Str("mxid", string(ghost.ID)).Str("ghost_name", ghost.Name).Str("avatar_mxc", string(ghost.AvatarMXC)).Msg("UpdateGhost called")

	// If ghost profile is empty, try to fetch it from Matrix
	if (ghost.Name == "" || ghost.AvatarMXC == "") && m.Connector.Config.SynapseAdmin.URL != "" {
		// Create ad-hoc admin client to fetch profile
		adminClient := NewMatrixAdminClient(m.Connector.Config.SynapseAdmin.URL, m.Connector.Config.SynapseAdmin.Token)
		profile, err := adminClient.GetProfile(ctx, id.UserID(ghost.ID))
		
		if err == nil && profile != nil {
			m.Connector.Bridge.Log.Info().Str("displayname", profile.DisplayName).Str("avatar_url", profile.AvatarURL).Msg("Fetched profile from Matrix")
			if profile.DisplayName != "" {
				ghost.Name = profile.DisplayName
			}
			if profile.AvatarURL != "" {
				ghost.AvatarMXC = id.ContentURIString(profile.AvatarURL)
			}
			// Update ghost in DB
			if ghost.Ghost != nil {
				m.Connector.Bridge.DB.Ghost.Update(ctx, ghost.Ghost)
			}
		} else {
			m.Connector.Bridge.Log.Warn().Err(err).Msg("Failed to fetch profile from Matrix")
		}
	}
	
	// Get authenticated client for the ghost (for updating profile)
	// We need a client that can update the user. System admin token (m.Client) is best.
	
	// Check if we need to update avatar
	if ghost.AvatarMXC == "" {
		m.Connector.Bridge.Log.Info().Str("mxid", string(ghost.ID)).Msg("Ghost has no AvatarMXC, skipping avatar update")
	} else if ghost.AvatarHash == [32]byte{} {
		m.Connector.Bridge.Log.Info().Str("mxid", string(ghost.ID)).Msg("Ghost has no AvatarHash, but has MXC... proceeding?")
		// Proceeding might be risky if we don't have hash? 
		// Actually typical logic relies on hash to check changes.
		// But let's log it.
	}

	if ghost.AvatarMXC != "" {
		// Download avatar from Matrix
		data, err := m.Connector.Bridge.Bot.DownloadMedia(ctx, ghost.AvatarMXC, nil)
		if err != nil {
			return fmt.Errorf("failed to download avatar: %w", err)
		}

		// Calculate hash and check if update is needed
		hash := sha256.Sum256(data)
		if hash == ghost.AvatarHash {
			m.Connector.Bridge.Log.Debug().Str("mxid", string(ghost.ID)).Msg("Avatar hash matches, skipping update")
		} else {
			// Upload to Mattermost
			_, err = m.Client.SetProfileImage(ctx, mmUserID, data)
			if err != nil {
				return fmt.Errorf("failed to set profile image: %w", err)
			}
			
			// Update hash and persist
			ghost.AvatarHash = hash
			if ghost.Ghost != nil {
				err = m.Connector.Bridge.DB.Ghost.Update(ctx, ghost.Ghost)
				if err != nil {
					m.Connector.Bridge.Log.Warn().Err(err).Msg("Failed to persist ghost avatar hash")
				}
			}
			m.Connector.Bridge.Log.Info().Str("user_id", mmUserID).Str("mxid", string(ghost.ID)).Msg("Updated ghost avatar")
		}
	}
	
	// Update Display Name if changed
	if ghost.Name != "" {
		// Fetch current user to compare
		user, _, err := m.Client.GetUser(ctx, mmUserID, "")
		if err == nil {
			// Update if different
			if user.FirstName != ghost.Name {
				patch := &model.UserPatch{
					FirstName: &ghost.Name,
					LastName:  ptr.Ptr(""),
					Nickname:  &ghost.Name,
				}
				_, _, err := m.Client.PatchUser(ctx, mmUserID, patch)
				if err != nil {
					m.Connector.Bridge.Log.Warn().Err(err).Msg("Failed to update ghost display name")
				} else {
					m.Connector.Bridge.Log.Info().Str("user_id", mmUserID).Str("name", ghost.Name).Msg("Updated ghost display name")
				}
			}
		}
	}

	return nil
}
