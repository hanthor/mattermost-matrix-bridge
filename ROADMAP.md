# Features & roadmap

* Matrix → Mattermost
    * [x] Message content
        * [x] Plain text
        * [x] Formatted text (Markdown)
        * [x] User pings
        * [x] Media and files
        * [x] Edits
        * [x] Threads (as MM threads)
    * [x] Reactions
    * [ ] Typing status
    * [x] Message redaction
    * [ ] Read receipts
* Mattermost → Matrix
    * [x] Message content
        * [x] Plain text
        * [x] Formatted text (Markdown)
        * [x] User pings
        * [x] Media and files (with metadata)
        * [x] Edits
        * [x] Threads (as Matrix native threads)
        * [ ] Custom Mattermost emoji
    * [x] Reactions
    * [ ] Typing status
    * [x] Message deletion
    * [x] Reading pre-login message history
    * [x] Conversation types
        * [x] Public channels
        * [x] Private channels
        * [x] 1:1 DM
        * [ ] Group DM
    * [x] Initial conversation metadata
        * [x] Name
        * [x] Topic/Purpose
        * [x] Description
        * [x] Channel members
    * [ ] Conversation metadata changes
        * [ ] Name
        * [ ] Topic
        * [ ] Avatar
    * [ ] Mark conversation as read
* Mirror Mode (Server-Wide Sync)
    * [x] Teams → Matrix Spaces
    * [x] Channels → Matrix Rooms
    * [x] User sync (ghosts)
    * [x] Team membership sync
    * [x] Channel membership sync
    * [x] Message history backfill
    * [ ] Real-time membership updates
    * [ ] User profile updates (avatar, display name changes)
    * [ ] Online/Away/DND status sync
* Slash Commands (`/matrix`)
    * [x] `/matrix help`
    * [x] `/matrix status`
    * [x] `/matrix me`
    * [x] `/matrix join <room>` - Join federated Matrix rooms
    * [x] `/matrix dm <user>` - Start DM with Matrix users
    * [x] `/matrix rooms` - List bridged rooms
    * [ ] `/matrix invite <user>` - Invite Matrix user to channel
    * [x] `/matrix account` - Get Matrix account credentials
* Matrix Account Access
    * [x] Ghost user creation via Synapse Admin API
    * [x] Password generation and delivery
    * [x] `/matrix account` credential retrieval
    * [ ] Web portal for Matrix credentials
    * [ ] SSO/OIDC integration (stretch goal)
* Mattermost Plugin UI (Future)
    * [ ] Bridge status panel in System Console
    * [ ] Per-channel bridge settings
    * [ ] Matrix room directory browser
    * [ ] User Matrix account management
    * [ ] Federation status dashboard
* Misc
    * [x] Automatic portal creation
        * [x] On startup (mirror mode)
        * [x] When receiving message
        * [ ] When added to channel
    * [x] Creating DM by inviting ghost to Matrix room
    * [x] Double puppeting support
    * [x] Shared channel portals between Matrix users
    * [ ] Relay bot mode for unauthenticated users
    * [ ] End-to-bridge encryption (disabled for MAS compatibility)
