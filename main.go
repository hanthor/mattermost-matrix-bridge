package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"maunium.net/go/mautrix/appservice"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/hanthor/mattermost-matrix-bridge/mattermost"
)

//go:embed example-config.yaml
var ExampleConfig string

type MattermostBridge struct {
	mxmain.BridgeMain
}

func main() {
	br := &MattermostBridge{}
	br.BridgeMain = mxmain.BridgeMain{
		Name:        "mautrix-mattermost",
		Description: "A Matrix-Mattermost puppeting bridge.",
		URL:         "https://github.com/hanthor/mattermost-matrix-bridge",
		Version:     "0.1.0",

		Connector: &mattermost.MattermostConnector{},
	}

	// Hook into PostInit to inject middleware
	br.PostInit = func() {
		// We need to wrap the AppService HTTP handler to intercept transactions
		// The BridgeMain.Matrix is a *matrix.Connector, which has the AS and Router
		// But BridgeMain fields are not directly exported in a way we can just swap the router easily
		// deeper in the struct. However, we can access br.Matrix.GetRouter() if we cast it.

		// Actually, br.Matrix is an interface in Bridge struct, but in BridgeMain it is *matrix.Connector
		// Let's look at how we can access the router.
		// br.Matrix is *matrix.Connector (from mxmain/main.go)

		// Wait, BridgeMain doesn't expose Matrix field publicly as *matrix.Connector, it's inside the struct.
		// But in main.go we define MattermostBridge embedding mxmain.BridgeMain.
		// The fields of BridgeMain ARE public.

		// br.Matrix.GetRouter() returns nil if PublicAddress is not configured/default,
		// but we might still be running in HTTP mode (e.g. behind proxy).
		// We can access the router directly via the AppService struct.
		if br.Matrix.AS == nil || br.Matrix.AS.Router == nil {
			br.Log.Warn().Msg("AppService or Router is nil - middleware not injected!")
			return
		}
		router := br.Matrix.AS.Router

		br.Log.Info().Msg("Injecting auto-provisioning middleware into AppService router")

		// Inject middleware
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Only intercept PUT /_matrix/app/v1/transactions/{txnID}
				if r.Method == "PUT" && strings.Contains(r.URL.Path, "/transactions/") {
					br.Log.Debug().Str("path", r.URL.Path).Msg("Middleware intercepted transaction request")
					// We need to peek at the body without consuming it
					// Read body
					bodyBytes, err := io.ReadAll(r.Body)
					if err != nil {
						br.Log.Err(err).Msg("Failed to read request body in middleware")
						next.ServeHTTP(w, r)
						return
					}
					// Restore body for next handler
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					// Parse transaction
					var txn appservice.Transaction
					if err := json.Unmarshal(bodyBytes, &txn); err != nil {
						br.Log.Err(err).Msg("Failed to unmarshal transaction in middleware")
						next.ServeHTTP(w, r)
						return
					}

					br.Log.Debug().Int("event_count", len(txn.Events)).Msg("Middleware parsed transaction")

					// Process events
					for _, evt := range txn.Events {
						if evt.Type == event.StateMember {
							if err := evt.Content.ParseRaw(evt.Type); err != nil {
								br.Log.Warn().Err(err).Msg("Failed to parse event content")
								continue
							}

							membership := evt.Content.AsMember().Membership
							stateKey := evt.GetStateKey()
							br.Log.Debug().Str("type", evt.Type.String()).Str("membership", string(membership)).Str("state_key", stateKey).Msg("Middleware processing member event")

							if membership == event.MembershipInvite {
								// Check if target is a ghost
								targetID := id.UserID(stateKey)
								_, isGhost := br.Matrix.ParseGhostMXID(targetID)

								br.Log.Debug().Str("target_id", stateKey).Bool("is_ghost", isGhost).Msg("Checking if invite target is ghost")

								if isGhost {
									// Sender is the inviter
									senderMXID := evt.Sender
									br.Log.Info().Str("sender", senderMXID.String()).Str("target", stateKey).Msg("Detected invite to ghost user")

									// Check if we have a login for this sender
									// We need the User object first
									ctx := r.Context()
									user, err := br.Bridge.GetUserByMXID(ctx, senderMXID)
									if err != nil {
										br.Log.Err(err).Str("sender", senderMXID.String()).Msg("Failed to get user in middleware")
										continue
									}

									// GetCachedUserLogins returns list
									logins := user.GetCachedUserLogins()
									br.Log.Info().Int("login_count", len(logins)).Msg("Checked existing logins for user")

									if len(logins) == 0 {
										// Auto-provision login!
										br.Log.Info().Str("user_id", senderMXID.String()).Msg("Auto-provisioning login for Matrix user inviting ghost")

										connector := br.Connector.(*mattermost.MattermostConnector)
										_, mmUserID, err := connector.GetClientForUser(ctx, senderMXID.String())
										if err != nil {
											br.Log.Err(err).Msg("Failed to get client/token for auto-provisioning")
											continue
										}

										// Get the ghost to get the token we just created
										ghost, err := br.Bridge.GetGhostByID(ctx, networkid.UserID(senderMXID.String()))
										if err != nil {
											br.Log.Err(err).Msg("Failed to get ghost after creating client")
											continue
										}
										var token string
										if ghost.Metadata != nil {
											if meta, ok := ghost.Metadata.(map[string]any); ok {
												token, _ = meta["mm_token"].(string)
											}
										}

										// We need to create a UserLogin
										loginID := networkid.UserLoginID("auto_" + senderMXID.String())

										// Create login data
										loginMetadata := map[string]any{
											"mm_id":               mmUserID,
											"token":               token,
											"is_auto_provisioned": true,
										}
										loginData := &database.UserLogin{
											ID:         loginID,
											BridgeID:   br.Bridge.ID,
											UserMXID:   senderMXID,
											RemoteName: senderMXID.String(),
											Metadata:   loginMetadata,
										}

										// Create the login
										newLogin, err := user.NewLogin(ctx, loginData, nil)
										if err != nil {
											br.Log.Err(err).Msg("Failed to auto-provision login")
										} else {
											br.Log.Info().Str("login_id", string(newLogin.ID)).Str("mm_id", mmUserID).Msg("Successfully auto-provisioned login")
										}
									}
								}
							}
						}
					}
				}

				next.ServeHTTP(w, r)
			})
		})
	}

	br.Run()
}
