# Agents Guide for Mautrix-Mattermost Bridge

This document provides guidance for AI agents working on the Mautrix-Mattermost bridge codebase.

## Project Overview

This is a Matrix-Mattermost bridge that enables bidirectional messaging between Matrix and Mattermost. The project has two implementations:

1. **mautrix-bridgev2** (`mattermost/` directory) - Standalone bridge using mautrix-go bridgev2 framework
2. **Legacy Plugin** (`legacy-plugin/server/` directory) - Mattermost plugin architecture with Application Service integration

## Key Documentation

- **[SPEC.md](SPEC.md)** - Source of truth for Matrix and Mattermost specs (profiles, messages, API endpoints)
- **[ROADMAP.md](ROADMAP.md)** - Feature status and planned work
- **[TESTING.md](TESTING.md)** - Testing procedures and scenarios
- **[README.md](README.md)** - Quick start and environment setup

## Directory Structure

```
mattermost/              # bridgev2-based implementation
├── api.go               # HTTP API handlers
├── client.go            # Mattermost SDK wrapper
├── connector.go         # Bridge connector interface
├── events.go            # Event type definitions
├── login.go             # Authentication logic
├── slashcmd.go          # /matrix slash commands
├── sync.go              # Mirror mode sync logic
├── websocket.go         # WebSocket event handling
└── msgconv/             # Message format conversion
    ├── from_mattermost.go   # MM → Matrix
    ├── to_mattermost.go     # Matrix → MM
    └── msgconv.go           # Converter setup

legacy-plugin/server/    # Mattermost plugin implementation
├── plugin.go            # Plugin entry point
├── sync_to_matrix.go    # MM → Matrix sync
├── matrix_webhook.go    # Matrix → MM webhook handler
├── bridge_utils.go      # Bridge utilities
├── emoji.go             # Emoji conversion
├── markdown_utils.go    # Markdown/HTML conversion
└── post_tracker.go      # Event ID mapping

tests/                   # Integration tests
scripts/                 # Development scripts
docker/                  # Docker configurations
```

## Key Dependencies

- `maunium.net/go/mautrix` - Matrix client library
- `maunium.net/go/mautrix/bridgev2` - Bridge framework
- `github.com/mattermost/mattermost/server/public/model` - Mattermost SDK
- `github.com/JohannesKaufmann/html-to-markdown` - HTML → Markdown conversion

## Common Tasks

### Adding New Message Type Support

1. Update `mattermost/msgconv/from_mattermost.go` for MM → Matrix
2. Update `mattermost/msgconv/to_mattermost.go` for Matrix → MM
3. Add tests in `mattermost/msgconv/msgconv_test.go`
4. Update SPEC.md with new formats

### Adding New Event Type

1. Define event handling in `mattermost/websocket.go`
2. Add Matrix event sending logic
3. Update `mattermost/events.go` if new types needed
4. Document in SPEC.md

### Working with Emoji

- Emoji mappings: `legacy-plugin/server/emoji_mappings_generated.go`
- Conversion logic: `legacy-plugin/server/emoji.go`
- Mattermost uses names (`:thumbsup:`), Matrix uses Unicode (👍)

### Working with Mentions

- User mention regex: `\B@([a-zA-Z0-9\.\-_:]+)\b`
- Matrix format: `<a href="https://matrix.to/#/@user:server">@User</a>`
- Must populate `m.mentions.user_ids` array

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
go test ./tests/...

# Start test environment
./setup.sh && ./provision_mm.sh

# Clean up
./cleanup.sh --full
```

## Important Patterns

### Ghost User IDs

Matrix ghost users for Mattermost users follow this pattern:
```
@_mattermost_{mattermost_user_id}:{server_domain}
```

### Event ID Mapping

Posts/events are tracked via:
- `post.Props["matrix_event_id_{server_domain}"]` - Matrix event ID stored on MM post
- KVStore mappings for bidirectional lookup

### Loop Prevention

- Check `post.Props["from_matrix"]` or `post.RemoteId` to avoid re-bridging
- Ghost user events should be filtered out

## Configuration

Key config fields (see `config.yaml`):
- `server_url` - Mattermost server URL
- `homeserver.address` - Matrix server URL
- `appservice` - Application service registration
- `bridge.permissions` - User permission levels

## Debugging Tips

1. Check logs: `docker compose logs bridge`
2. Verify WebSocket connection in `websocket.go`
3. Use `/matrix status` slash command
4. Check event mapping in KVStore

## When Making Changes

1. Always consult SPEC.md for format specifications
2. Update SPEC.md if you discover new API behaviors
3. Add tests for new functionality
4. Maintain backward compatibility with existing mappings
