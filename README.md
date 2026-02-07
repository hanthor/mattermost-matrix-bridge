# matrix-mattermost-bridge

A Matrix-Mattermost bridge built on the [mautrix-go](https://github.com/mautrix/go) framework by Tulir Asokan.

> **⚠️ PRE-RELEASE SOFTWARE**: This bridge is currently in active development. Direct Messages (DMs) have been fully tested and work reliably. Other features are functional but require further testing and refinement.
>
> **🔒 SECURITY NOTICE**: A comprehensive security review has identified critical issues that must be addressed before production deployment. See [SECURITY_REVIEW.md](SECURITY_REVIEW.md) for details and [SECURITY_FIXES.md](SECURITY_FIXES.md) for implementation guidance.
>
> **Note**: This is an independent project and not an official mautrix bridge.

## Why This Bridge?

### Unlock Federation for Free Mattermost

Mattermost's federation features are currently **only available in Enterprise Edition**. This bridge brings **full Matrix federation** to **any Mattermost instance**, including free/open-source deployments.

**What this enables:**
- 🌐 **True Federation** - Connect your Mattermost server to the global Matrix network
- 💬 **Cross-Platform Messaging** - Chat with users on Element, FluffyChat, and other Matrix clients
- 🔗 **Federated DMs** - Direct message users across different homeservers
- 🏢 **No Enterprise License Required** - Federation for everyone

### Built on Modern Bridge Architecture

- **Ghost Puppeting** - Matrix users appear as native Mattermost users (not relay bots)
- **Bidirectional Sync** - Full two-way message flow
- **Native Threading** - Mattermost and Matrix threads map correctly
- **Rich Content** - Text formatting, files, reactions, edits, and deletions

## Current Status

### ✅ Fully Tested & Working
- **Direct Messages (DMs)** - Fully functional with federation support
- **Message Content** - Plain text, markdown, media, files
- **Message Actions** - Edits, deletions, reactions, threads
- **Ghost Profiles** - Avatar and display name synchronization
- **Slash Commands** - `/matrix dm`, `/matrix help`, `/matrix status`, etc.

### 🚧 Functional but Needs Testing
- **Public Channels** - Basic functionality works
- **Private Channels** - Basic functionality works  
- **Channel Metadata** - Name, topic, purpose syncing
- **Mirror Mode** - Server-wide sync (teams → spaces, channels → rooms)

### 📋 Roadmap

**High Priority:**
- [ ] **End-to-Bridge Encryption (E2B)** - Encrypt messages in transit
- [ ] **Large Room Testing** - Join and sync large public Matrix rooms
- [ ] **Full Mirror Mode** - Complete Matrix hijack for seamless experience
- [ ] **Group DMs** - Multi-user DM support
- [ ] **Typing Indicators** - Real-time typing status
- [ ] **Read Receipts** - Message read status sync

**Future Enhancements:**
- [ ] Online/Away/DND status sync
- [ ] Custom emoji support
- [ ] SSO/OIDC integration
- [ ] Web portal for Matrix credentials

See [ROADMAP.md](ROADMAP.md) for the complete feature matrix.

## Quick Start

### Prerequisites
- Mattermost server (any edition)
- Synapse homeserver
- Docker and Docker Compose (for local testing)

### Local Development Setup

```bash
# 1. Clone the repository
git clone https://github.com/hanthor/matrix-mattermost-bridge
cd matrix-mattermost-bridge

# 2. Start services
./scripts/local-setup.sh

# 3. Configure the bridge
cp example-config.yaml config.yaml
# Edit config.yaml with your server details
```

### Production Deployment

See the [deployment guide](docs/deployment.md) for production setup instructions including:
- Kubernetes manifests
- Docker Compose configuration  
- TLS/SSL setup
- Slash command integration
- Monitoring and troubleshooting

## Usage

### Slash Commands

From Mattermost, use these commands to interact with Matrix:

```
/matrix help                    # Show available commands
/matrix dm @user:example.org    # Start a DM with a Matrix user
/matrix join #room:example.org  # Join a Matrix room
/matrix rooms                   # List your bridged rooms
/matrix status                  # Check bridge connection
/matrix account                 # Get your Matrix credentials
```

### Federation Example

1. **In Mattermost**: `/matrix dm @alice:matrix.org`
2. **Matrix user receives invitation** via Element/other client
3. **Both users can chat** across platforms seamlessly

## Architecture

- **Ghost Users** - Matrix IDs map to Mattermost usernames (e.g., `mx.alice_matrix.org`)
- **Personal Access Tokens** - Cached per-user for API authentication
- **Smart Sync** - SHA256-based avatar deduplication
- **UUID Mapping** - Reliable Mattermost ID resolution

## Configuration

Key configuration options in `config.yaml`:

```yaml
mattermost:
  server_url: https://your-mattermost.example.org
  admin_token: your_admin_token

synapse_admin:
  url: https://your-synapse.example.org
  token: your_admin_token

mirror:
  enabled: false  # Set to true for full server sync
  sync_all_teams: true
  sync_all_channels: true
```

See [example-config.yaml](example-config.yaml) for all options.

## Contributing

Contributions are welcome! This bridge is in active development and we need help with:

- **Security**: Implementing fixes from [SECURITY_FIXES.md](SECURITY_FIXES.md)
- Testing large Matrix rooms
- E2B encryption implementation
- Mirror mode refinements
- Documentation improvements

## Security

**Important**: This bridge has undergone a security review. Please review the following documents before deployment:

- 📋 [SECURITY_REVIEW.md](SECURITY_REVIEW.md) - Comprehensive security analysis
- 🔧 [SECURITY_FIXES.md](SECURITY_FIXES.md) - Implementation guide for fixes
- 📝 [SECURITY_SUMMARY.md](SECURITY_SUMMARY.md) - Quick reference
- 🔒 [SECURITY.md](SECURITY.md) - Vulnerability disclosure policy

**Current Security Status**: ⚠️ Not recommended for production (Critical issues identified)

To report security vulnerabilities, please follow the [responsible disclosure policy](SECURITY.md).

## Support

- **Issues**: [GitHub Issues](https://github.com/hanthor/matrix-mattermost-bridge/issues)
- **Security**: See [SECURITY.md](SECURITY.md) for reporting vulnerabilities
- **Matrix Room**: Coming soon
- **Documentation**: See [SPEC.md](SPEC.md) for technical details

## License

AGPL-3.0 - See [LICENSE](LICENSE) for details.

## Acknowledgments

This bridge is built on the excellent [mautrix-go](https://github.com/mautrix/go) framework created by **Tulir Asokan**. The mautrix-go library provides the core Matrix protocol implementation and bridge architecture that makes this project possible.

Special thanks to Tulir and Beeper, and the broader Matrix community for their foundational work on Matrix bridging infrastructure.
