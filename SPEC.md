# Mattermost-Matrix Bridge Specification

This document serves as the source of truth for Matrix and Mattermost specifications relevant to bridging. Update this document whenever new API behaviors or formats are discovered.

---

## Table of Contents

1. [Matrix Specifications](#1-matrix-specifications)
   - [API Endpoints](#11-matrix-client-server-api-endpoints)
   - [Event Types](#12-matrix-event-types)
   - [Message Formats](#13-matrix-message-formats)
   - [Profile Formats](#14-matrix-profile-formats)
   - [Room Formats](#15-matrix-room-formats)
2. [Mattermost Specifications](#2-mattermost-specifications)
   - [API Endpoints](#21-mattermost-api-endpoints)
   - [Data Types](#22-mattermost-data-types)
   - [WebSocket Events](#23-mattermost-websocket-events)
3. [Message Conversion](#3-message-conversion)
   - [Text/Markdown](#31-textmarkdown-conversion)
   - [File/Media](#32-filemedia-conversion)
   - [Reactions](#33-reactions)
   - [Threads](#34-threads)
   - [Mentions](#35-mentions)
   - [Emoji](#36-emoji)
4. [Bridge Mappings](#4-bridge-mappings)

---

## 1. Matrix Specifications

### 1.1 Matrix Client-Server API Endpoints

Reference: https://spec.matrix.org/latest/client-server-api/

#### Authentication & User Management

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/_matrix/client/v3/account/whoami` | GET | Verify authentication |
| `/_matrix/client/v3/register` | POST | Register user (Application Service) |
| `/_matrix/client/v3/profile/{userId}` | GET | Get full user profile |
| `/_matrix/client/v3/profile/{userId}/displayname` | GET/PUT | Get/set display name |
| `/_matrix/client/v3/profile/{userId}/avatar_url` | GET/PUT | Get/set avatar URL |

#### Room Operations

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/_matrix/client/v3/createRoom` | POST | Create new room |
| `/_matrix/client/v3/join/{roomIdOrAlias}` | POST | Join room |
| `/_matrix/client/v3/rooms/{roomId}/invite` | POST | Invite user to room |
| `/_matrix/client/v3/rooms/{roomId}/leave` | POST | Leave room |
| `/_matrix/client/v3/rooms/{roomId}/state/{eventType}/{stateKey}` | GET/PUT | Get/set room state |
| `/_matrix/client/v3/directory/room/{roomAlias}` | GET/PUT/DELETE | Room alias operations |

#### Messaging

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/_matrix/client/v3/rooms/{roomId}/send/{eventType}/{txnId}` | PUT | Send message event |
| `/_matrix/client/v3/rooms/{roomId}/redact/{eventId}/{txnId}` | PUT | Redact (delete) event |
| `/_matrix/client/v3/rooms/{roomId}/event/{eventId}` | GET | Get single event |
| `/_matrix/client/v1/rooms/{roomId}/relations/{eventId}` | GET | Get related events (edits, reactions) |
| `/_matrix/client/v3/rooms/{roomId}/messages` | GET | Paginate room messages |

#### Media

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/_matrix/media/v3/upload` | POST | Upload media file |
| `/_matrix/media/v3/download/{serverName}/{mediaId}` | GET | Download media |
| `/_matrix/media/v3/thumbnail/{serverName}/{mediaId}` | GET | Get media thumbnail |
| `/_matrix/client/v1/media/download/{serverName}/{mediaId}` | GET | Authenticated download (newer) |

#### Synapse Admin API (Non-Standard)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/_synapse/admin/v2/users/{userId}` | GET/PUT | Get/create/update user |
| `/_synapse/admin/v1/join/{roomId}` | POST | Force user to join room |
| `/_synapse/admin/v1/rooms/{roomId}` | GET/DELETE | Room admin operations |

### 1.2 Matrix Event Types

```
m.room.message          # Text, file, image, video, audio messages
m.room.member           # Join, leave, invite, ban membership changes
m.reaction              # Emoji reactions
m.room.redaction        # Message deletion
m.room.name             # Room name state event
m.room.topic            # Room topic state event
m.room.avatar           # Room avatar state event
m.room.join_rules       # Room join rules (public/invite)
m.room.history_visibility  # Who can see history
m.room.guest_access     # Guest access rules
m.room.canonical_alias  # Primary room alias
m.room.create           # Room creation event
m.space.child           # Space-room relationship
m.space.parent          # Room-space relationship
```

Custom bridge event type:
```
com.mattermost.bridge.channel   # Mattermost channel metadata
```

### 1.3 Matrix Message Formats

#### Text Message (m.room.message)

```json
{
  "msgtype": "m.text",
  "body": "Plain text fallback",
  "format": "org.matrix.custom.html",
  "formatted_body": "<p>HTML <strong>formatted</strong> content</p>",
  "m.mentions": {
    "user_ids": ["@user:example.com"],
    "room": false
  }
}
```

#### Message Types (msgtype)

| msgtype | Description |
|---------|-------------|
| `m.text` | Plain text or HTML formatted message |
| `m.notice` | Bot/system notification |
| `m.emote` | /me action message |
| `m.image` | Image file |
| `m.file` | Generic file |
| `m.video` | Video file |
| `m.audio` | Audio file |
| `m.location` | Geographic location |

#### File Message

```json
{
  "msgtype": "m.image",
  "body": "filename.jpg",
  "url": "mxc://example.com/media_id",
  "info": {
    "mimetype": "image/jpeg",
    "size": 12345,
    "w": 800,
    "h": 600,
    "thumbnail_url": "mxc://example.com/thumb_id",
    "thumbnail_info": {
      "mimetype": "image/jpeg",
      "size": 1234,
      "w": 100,
      "h": 75
    }
  }
}
```

#### Edit (m.room.message with m.new_content)

```json
{
  "msgtype": "m.text",
  "body": " * Updated message text",
  "m.new_content": {
    "msgtype": "m.text",
    "body": "Updated message text",
    "format": "org.matrix.custom.html",
    "formatted_body": "<p>Updated message text</p>"
  },
  "m.relates_to": {
    "rel_type": "m.replace",
    "event_id": "$original_event_id"
  }
}
```

#### Reaction (m.reaction)

```json
{
  "m.relates_to": {
    "rel_type": "m.annotation",
    "event_id": "$target_message_event_id",
    "key": "👍"
  }
}
```

#### Thread Reply

```json
{
  "msgtype": "m.text",
  "body": "Reply message",
  "m.relates_to": {
    "rel_type": "m.thread",
    "event_id": "$thread_root_event_id",
    "is_falling_back": true,
    "m.in_reply_to": {
      "event_id": "$replied_to_event_id"
    }
  }
}
```

#### Simple Reply (non-threaded)

```json
{
  "msgtype": "m.text",
  "body": "> Original message\n\nReply message",
  "format": "org.matrix.custom.html",
  "formatted_body": "<mx-reply><blockquote>Original</blockquote></mx-reply>Reply",
  "m.relates_to": {
    "m.in_reply_to": {
      "event_id": "$parent_event_id"
    }
  }
}
```

### 1.4 Matrix Profile Formats

#### User Profile

```json
{
  "displayname": "John Doe",
  "avatar_url": "mxc://example.com/avatar_media_id"
}
```

#### Ghost User ID Pattern

```
@_mattermost_{mattermost_user_id}:{homeserver_domain}
```

Example: `@_mattermost_abc123def456:matrix.example.com`

#### Application Service Registration Request

```json
{
  "type": "m.login.application_service",
  "username": "_mattermost_abc123def456"
}
```

### 1.5 Matrix Room Formats

#### Room Creation Request

```json
{
  "name": "Room Display Name",
  "topic": "Room topic or purpose",
  "preset": "public_chat",
  "visibility": "public",
  "room_alias_name": "_mattermost_channel-name",
  "room_version": "10",
  "is_direct": false,
  "initial_state": [
    {
      "type": "m.room.guest_access",
      "state_key": "",
      "content": { "guest_access": "can_join" }
    },
    {
      "type": "m.room.history_visibility",
      "state_key": "",
      "content": { "history_visibility": "world_readable" }
    },
    {
      "type": "m.room.join_rules",
      "state_key": "",
      "content": { "join_rule": "public" }
    },
    {
      "type": "com.mattermost.bridge.channel",
      "state_key": "",
      "content": {
        "mattermost_channel_id": "channel_id",
        "mattermost_team_id": "team_id",
        "created_at": 1234567890
      }
    }
  ],
  "creation_content": {
    "m.federate": true
  }
}
```

#### Room Presets

| Preset | Join Rule | History Visibility | Guest Access |
|--------|-----------|-------------------|--------------|
| `public_chat` | public | shared | can_join |
| `private_chat` | invite | shared | forbidden |
| `trusted_private_chat` | invite | shared (all) | forbidden |

#### Room Alias Pattern

```
#_mattermost_{team}_{channel}:{homeserver_domain}
```

---

## 2. Mattermost Specifications

### 2.1 Mattermost API Endpoints

Reference: https://api.mattermost.com/

#### Authentication

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/users/login` | POST | Login and get session token |
| `/api/v4/users/logout` | POST | Logout and invalidate session |

#### Users

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/users` | GET/POST | List users / Create user |
| `/api/v4/users/{user_id}` | GET/PUT/DELETE | Get/Update/Delete user |
| `/api/v4/users/{user_id}/image` | GET/POST | Get/Set profile image |
| `/api/v4/users/me` | GET | Get current user |
| `/api/v4/users/username/{username}` | GET | Get user by username |
| `/api/v4/users/email/{email}` | GET | Get user by email |

#### Teams

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/teams` | GET/POST | List/Create teams |
| `/api/v4/teams/{team_id}` | GET/PUT/DELETE | Team operations |
| `/api/v4/teams/{team_id}/members` | GET/POST | Team members |
| `/api/v4/teams/{team_id}/image` | GET | Team icon |
| `/api/v4/users/{user_id}/teams` | GET | User's teams |

#### Channels

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/channels` | POST | Create channel |
| `/api/v4/channels/{channel_id}` | GET/PUT/DELETE | Channel operations |
| `/api/v4/channels/{channel_id}/members` | GET/POST | Channel members |
| `/api/v4/channels/direct` | POST | Create direct message channel |
| `/api/v4/channels/group` | POST | Create group message channel |
| `/api/v4/teams/{team_id}/channels` | GET | Team's public channels |
| `/api/v4/users/{user_id}/channels` | GET | User's channels |

#### Posts (Messages)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/posts` | POST | Create post |
| `/api/v4/posts/{post_id}` | GET/PUT/DELETE | Post operations |
| `/api/v4/channels/{channel_id}/posts` | GET | Get channel posts |
| `/api/v4/posts/{post_id}/thread` | GET | Get thread posts |

#### Files

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/files` | POST | Upload file |
| `/api/v4/files/{file_id}` | GET | Download file |
| `/api/v4/files/{file_id}/info` | GET | Get file metadata |
| `/api/v4/files/{file_id}/thumbnail` | GET | Get file thumbnail |
| `/api/v4/files/{file_id}/preview` | GET | Get file preview |

#### Reactions

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v4/reactions` | POST | Add reaction |
| `/api/v4/users/{user_id}/posts/{post_id}/reactions/{emoji_name}` | DELETE | Remove reaction |
| `/api/v4/posts/{post_id}/reactions` | GET | Get post reactions |

### 2.2 Mattermost Data Types

#### User

```json
{
  "id": "abc123def456",
  "username": "johndoe",
  "email": "john@example.com",
  "first_name": "John",
  "last_name": "Doe",
  "nickname": "Johnny",
  "position": "Developer",
  "roles": "system_user",
  "last_picture_update": 1234567890000,
  "locale": "en",
  "timezone": {
    "automaticTimezone": "America/New_York",
    "manualTimezone": "",
    "useAutomaticTimezone": "true"
  },
  "create_at": 1234567890000,
  "update_at": 1234567890000,
  "delete_at": 0,
  "remote_id": null
}
```

**Key fields for bridging:**
- `id` - Unique identifier (used for ghost user ID)
- `username` - Display name / mention handle
- `first_name` + `last_name` - Full display name
- `last_picture_update` - Avatar change detection
- `remote_id` - Non-null indicates federated/bridged user

#### Channel

```json
{
  "id": "channel123",
  "team_id": "team456",
  "type": "O",
  "display_name": "General",
  "name": "general",
  "header": "Channel header text",
  "purpose": "Channel purpose/description",
  "create_at": 1234567890000,
  "update_at": 1234567890000,
  "delete_at": 0,
  "last_post_at": 1234567890000,
  "total_msg_count": 100
}
```

**Channel Types:**

| Type | Code | Description |
|------|------|-------------|
| Public | `O` | Open channel, anyone can join |
| Private | `P` | Private channel, invite only |
| Direct | `D` | 1:1 direct message |
| Group | `G` | Group direct message (3+ users) |

#### Post

```json
{
  "id": "post789",
  "channel_id": "channel123",
  "user_id": "user456",
  "root_id": "",
  "message": "Hello **world**!",
  "type": "",
  "create_at": 1234567890000,
  "update_at": 1234567890000,
  "edit_at": 0,
  "delete_at": 0,
  "file_ids": ["file1", "file2"],
  "props": {
    "from_matrix": true,
    "matrix_event_id_example.com": "$event123"
  },
  "hashtags": "",
  "pending_post_id": "",
  "remote_id": null,
  "metadata": {
    "files": [...],
    "reactions": [...],
    "embeds": [...]
  }
}
```

**Key fields:**
- `root_id` - Empty for root posts, parent ID for thread replies
- `message` - Markdown formatted text
- `file_ids` - Array of attached file IDs
- `props` - Custom properties (used for bridge metadata)
- `remote_id` - Non-null indicates bridged message
- `edit_at` - Non-zero when message has been edited

#### FileInfo

```json
{
  "id": "file123",
  "user_id": "user456",
  "post_id": "post789",
  "channel_id": "channel123",
  "name": "document.pdf",
  "extension": "pdf",
  "mime_type": "application/pdf",
  "size": 12345,
  "width": 0,
  "height": 0,
  "has_preview_image": false,
  "create_at": 1234567890000,
  "delete_at": 0,
  "remote_id": null
}
```

**For images:**
- `width` and `height` populated with dimensions
- `has_preview_image` may be true

#### Reaction

```json
{
  "user_id": "user456",
  "post_id": "post789",
  "emoji_name": "thumbsup",
  "create_at": 1234567890000,
  "remote_id": null,
  "channel_id": "channel123"
}
```

**Note:** `emoji_name` is the shortcode without colons (e.g., `thumbsup` not `:thumbsup:`)

### 2.3 Mattermost WebSocket Events

Connect to: `wss://{server}/api/v4/websocket`

#### Event Structure

```json
{
  "event": "posted",
  "data": {
    "channel_display_name": "General",
    "channel_name": "general",
    "channel_type": "O",
    "post": "{\"id\":\"post123\",...}",
    "sender_name": "johndoe",
    "team_id": "team456"
  },
  "broadcast": {
    "channel_id": "channel123",
    "team_id": "team456",
    "user_id": ""
  },
  "seq": 123
}
```

**Note:** The `post` field is a JSON-encoded string that must be parsed.

#### Event Types for Bridging

| Event | Description |
|-------|-------------|
| `posted` | New message posted |
| `post_edited` | Message edited |
| `post_deleted` | Message deleted |
| `reaction_added` | Reaction added to post |
| `reaction_removed` | Reaction removed from post |
| `channel_created` | New channel created |
| `channel_updated` | Channel metadata changed |
| `channel_deleted` | Channel deleted |
| `user_added` | User added to channel |
| `user_removed` | User removed from channel |
| `user_updated` | User profile updated |
| `typing` | User is typing |
| `status_change` | User status changed (online/away/dnd) |

---

## 3. Message Conversion

### 3.1 Text/Markdown Conversion

#### Mattermost → Matrix

Mattermost uses GitHub-flavored Markdown. Convert to Matrix HTML:

| Mattermost | Matrix HTML |
|------------|-------------|
| `**bold**` | `<strong>bold</strong>` |
| `*italic*` or `_italic_` | `<em>italic</em>` |
| `~~strikethrough~~` | `<del>strikethrough</del>` |
| `` `code` `` | `<code>code</code>` |
| ` ```lang\ncode\n``` ` | `<pre><code class="language-lang">code</code></pre>` |
| `# Heading` | `<h1>Heading</h1>` |
| `[text](url)` | `<a href="url">text</a>` |
| `![alt](url)` | Handled as inline image or ignored |
| `> quote` | `<blockquote>quote</blockquote>` |
| `- list` | `<ul><li>list</li></ul>` |
| `1. list` | `<ol><li>list</li></ol>` |
| `| table |` | `<table>...</table>` |

**Implementation:** Use `maunium.net/go/mautrix/format.RenderMarkdown()`

#### Matrix → Mattermost

Convert HTML back to Markdown:

**Implementation:** Use `github.com/JohannesKaufmann/html-to-markdown`

```go
converter := md.NewConverter("", true, nil)
markdown, err := converter.ConvertString(htmlContent)
```

### 3.2 File/Media Conversion

#### Mattermost → Matrix

1. Download file: `GET /api/v4/files/{file_id}`
2. Get metadata: `GET /api/v4/files/{file_id}/info`
3. Upload to Matrix: `POST /_matrix/media/v3/upload`
4. Create Matrix message with `mxc://` URL

**MIME Type to msgtype mapping:**

| MIME Type | Matrix msgtype |
|-----------|---------------|
| `image/*` | `m.image` |
| `video/*` | `m.video` |
| `audio/*` | `m.audio` |
| `*/*` (other) | `m.file` |

#### Matrix → Mattermost

1. Download from Matrix: `GET /_matrix/media/v3/download/{server}/{media_id}`
2. Upload to Mattermost: `POST /api/v4/files`
3. Attach file ID to post: `post.FileIds = [file_id]`

### 3.3 Reactions

#### Mattermost → Matrix

1. Mattermost uses emoji shortcodes: `thumbsup`, `smile`, `+1`
2. Convert to Unicode emoji: `👍`, `😄`, `👍`
3. Send Matrix reaction event

**Common mappings:**

| Mattermost | Unicode |
|------------|---------|
| `thumbsup`, `+1` | 👍 |
| `thumbsdown`, `-1` | 👎 |
| `smile` | 😄 |
| `heart` | ❤️ |
| `white_check_mark` | ✅ |

#### Matrix → Mattermost

1. Extract emoji from reaction event `key`
2. Convert Unicode to Mattermost shortcode if possible
3. Save reaction: `POST /api/v4/reactions`

### 3.4 Threads

#### Mattermost Threading

- Root posts have empty `root_id`
- Reply posts have `root_id` set to the root post ID
- All replies in a thread share the same `root_id`

#### Matrix Threading

- Uses `m.relates_to` with `rel_type: "m.thread"`
- `event_id` points to thread root
- Optional `m.in_reply_to` for specific reply target

#### Conversion

**Mattermost → Matrix:**
```go
if post.RootId != "" {
    content["m.relates_to"] = {
        "rel_type": "m.thread",
        "event_id": lookupMatrixEventID(post.RootId)
    }
}
```

**Matrix → Mattermost:**
```go
if relatesTo.RelType == "m.thread" {
    post.RootId = lookupMattermostPostID(relatesTo.EventID)
}
```

### 3.5 Mentions

#### Mattermost Mention Formats

| Format | Description |
|--------|-------------|
| `@username` | Mention specific user |
| `@channel` | Notify all channel members |
| `@all` | Notify all channel members |
| `@here` | Notify online channel members |

Regex for user mentions: `\B@([a-zA-Z0-9\.\-_:]+)\b`

#### Matrix Mention Format

HTML pill format:
```html
<a href="https://matrix.to/#/@user:server.com">@DisplayName</a>
```

Must include in `m.mentions`:
```json
{
  "m.mentions": {
    "user_ids": ["@user:server.com"],
    "room": false
  }
}
```

#### Conversion

**Mattermost → Matrix:**
1. Find `@username` patterns
2. Look up Matrix ghost user ID for username
3. Replace with Matrix pill HTML
4. Populate `m.mentions.user_ids`

**Matrix → Mattermost:**
1. Parse `m.mentions.user_ids` array
2. Look up Mattermost username for each Matrix user
3. Replace pill HTML with `@username`

### 3.6 Emoji

#### Emoji Name Mapping

Mattermost uses shortcodes. See `legacy-plugin/server/emoji_mappings_generated.go` for full mapping.

**Conversion functions:**

```go
// Mattermost → Matrix (name to Unicode)
func convertEmojiForMatrix(emojiName string) string {
    // "thumbsup" → "👍"
}

// Matrix → Mattermost (Unicode to name)  
func convertMatrixEmojiToMattermost(emoji string) string {
    // "👍" → "thumbsup"
}
```

**Hex to Unicode conversion:**
```go
func hexToUnicode(hexStr string) string {
    // "1f44d" → "👍"
    // "1f1fa-1f1f8" → "🇺🇸" (multi-codepoint)
}
```

---

## 4. Bridge Mappings

### 4.1 ID Mappings

#### Ghost User ID

```
Matrix: @_mattermost_{mm_user_id}:{server}
Example: @_mattermost_abc123:matrix.example.com
```

#### Room Alias

```
Matrix: #_mattermost_{team}_{channel}:{server}
Example: #_mattermost_myteam_general:matrix.example.com
```

### 4.2 Post Properties (Loop Prevention)

Store on Mattermost post:
```go
post.Props["from_matrix"] = true
post.Props["matrix_event_id_"+serverDomain] = eventID
```

Check before bridging:
```go
if post.Props["from_matrix"] == true || post.RemoteId != nil {
    // Skip - already bridged
}
```

### 4.3 Ghost Puppeting (Matrix → Mattermost)

When sending messages from Matrix to Mattermost, the bridge implements **ghost puppeting** where messages appear directly from ghost users rather than being relayed:

```go
// Get or create ghost for Matrix sender
ghost, err := bridge.GetGhostByID(ctx, networkid.UserID(matrixUserID))

// Get the Mattermost user ID for this ghost
mmUserID := getMMID(ctx, ghost.ID)

// Set the post's UserId to enable ghost puppeting
post.UserId = mmUserID

// Mark as from Matrix to prevent loops
post.Props["from_matrix"] = true
```

**Ghost User Creation:**
- Ghost users are created on Mattermost with username pattern: `matrix_{localpart}_{server}`
- Ghost users have `Position` set to "Matrix Bridge Ghost"
- Reversible encoding: `_` → `__`, `:` → `.` (or `_c` legacy), `.` → `_d`, special chars → `_xHH`

**Benefits:**
- Messages appear to come directly from the Matrix user's ghost
- No "UserA: message" relay prefix
- Proper attribution and conversation flow
- Reactions and edits work correctly

### 4.4 KVStore Keys (Legacy Plugin)

```
channel_mapping_{channelID}         → room_id
room_mapping_{roomID}               → channel_id
ghost_user_{mmUserID}               → matrixGhostUserID
matrix_user_{matrixUserID}          → mattermostUserID
ghost_room_{userID}_{roomID}        → "joined"
matrix_event_{eventID}              → mattermostPostID
mattermost_post_{postID}            → matrixEventID
matrix_reaction_{reactionEventID}   → {post_id, user_id, emoji_name}
```

### 4.5 Bridge State Event

Store in Matrix room state for channel metadata:

```json
{
  "type": "com.mattermost.bridge.channel",
  "state_key": "",
  "content": {
    "mattermost_channel_id": "channel123",
    "mattermost_team_id": "team456",
    "mattermost_channel_type": "O",
    "created_at": 1234567890
  }
}
```

---

## 5. Version History

| Date | Changes |
|------|---------|
| 2026-02-06 | Initial specification document created |

---

## 6. References

- Matrix Client-Server API: https://spec.matrix.org/latest/client-server-api/
- Matrix Application Service API: https://spec.matrix.org/latest/application-service-api/
- Mattermost API Reference: https://api.mattermost.com/
- Mattermost Data Model: https://docs.mattermost.com/
- mautrix-go Documentation: https://pkg.go.dev/maunium.net/go/mautrix
