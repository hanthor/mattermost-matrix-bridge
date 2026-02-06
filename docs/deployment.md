# Deployment Guide

This guide covers deploying the matrix-mattermost-bridge in production environments.

## Prerequisites

- **Mattermost Server** (any edition, v7.0+)
- **Matrix Homeserver** (Synapse recommended)
- **Docker** or **Kubernetes**
- **System Admin Access** to both Mattermost and Matrix

## Deployment Options

### Option 1: Docker Compose (Recommended for Small Deployments)

See [`docker-compose.yaml`](../docker-compose.yaml) in the repository root for a complete local setup.

For production:
1. Update the compose file with your domain names
2. Configure TLS/SSL certificates
3. Set secure passwords in environment variables
4. Use volume mounts for persistent data

### Option 2: Kubernetes (Recommended for Production)

Full Kubernetes manifests are provided in the [`examples/`](examples/) directory.

#### Quick Start

1. **Deploy Mattermost** (if not already running):
   ```bash
   kubectl apply -f docs/examples/kubernetes-mattermost.yaml
   ```

2. **Create Bridge Configuration**:
   ```bash
   # Edit the example to add your tokens and URLs
   kubectl apply -f docs/examples/kubernetes-bridge.yaml
   ```

3. **Register the Appservice** with Synapse:
   - Extract the `registration.yaml` from the bridge secret
   - Add it to your Synapse configuration
   - Restart Synapse

## Configuration Steps

### 1. Generate Tokens

Generate secure tokens for the bridge:

```bash
# Generate appservice and homeserver tokens
openssl rand -hex 32  # AS token
openssl rand -hex 32  # HS token
```

### 2. Create Mattermost Admin Token

1. Log in to Mattermost as a System Admin
2. Go to **Account Settings** → **Security** → **Personal Access Tokens**
3. Create a token with **System Admin** privileges
4. Save the token securely

### 3. Get Synapse Admin Token

If using Synapse Admin API for user management:

```bash
# Generate via Synapse admin API or use existing admin token
curl -X POST "https://matrix.example.com/_synapse/admin/v1/users/@admin:example.com/login" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN"
```

### 4. Configure the Bridge

Edit `config.yaml` with your settings:

```yaml
homeserver:
  address: https://matrix.example.com
  domain: example.com

network:
  server_url: https://mattermost.example.com
  admin_token: YOUR_MATTERMOST_ADMIN_TOKEN
  mode: mirror  # or "puppet" for per-user bridging

synapse_admin:
  url: https://matrix.example.com
  token: YOUR_SYNAPSE_ADMIN_TOKEN
```

See [`example-config.yaml`](../example-config.yaml) for all configuration options.

### 5. Register with Synapse

Add the bridge's `registration.yaml` to your Synapse `homeserver.yaml`:

```yaml
app_service_config_files:
  - /path/to/mattermost-registration.yaml
```

Then restart Synapse.

## Bridge Modes

### Puppet Mode (Default)

- Users authenticate individually
- Each user controls their own bridging
- Best for small teams or personal use

### Mirror Mode (Server-Wide Sync)

```yaml
network:
  mode: mirror
  mirror:
    sync_all_teams: true
    sync_all_channels: true
    sync_all_users: true
    sync_history: true
```

- Entire Mattermost server syncs to Matrix
- Teams become Matrix Spaces
- Channels become Matrix Rooms
- Best for large deployments

## Slash Command Setup

To enable `/matrix` commands in Mattermost:

1. Go to **System Console** → **Integrations** → **Slash Commands**
2. Create a new command:
   - **Command Trigger**: `matrix`
   - **Request URL**: `http://bridge:8081/slash`
   - **Request Method**: POST
   - **Response Username**: Mattermost Bot
   - **Autocomplete**: Enabled

3. Add the slash command token to your bridge config:
   ```yaml
   network:
     slash_command:
       enabled: true
       token: YOUR_SLASH_COMMAND_TOKEN
   ```

## TLS/SSL Configuration

### With Reverse Proxy (Recommended)

Use nginx, Traefik, or another reverse proxy:

```nginx
# Synapse
server {
    listen 443 ssl;
    server_name matrix.example.com;
    
    location / {
        proxy_pass http://synapse:8008;
    }
}

# Mattermost
server {
    listen 443 ssl;
    server_name mattermost.example.com;
    
    location / {
        proxy_pass http://mattermost:8065;
    }
}
```

### Kubernetes Ingress

See the commented Ingress sections in the example YAML files.

## Monitoring and Troubleshooting

### Check Bridge Status

```bash
# Kubernetes
kubectl logs -n mautrix deployment/mattermost-matrix-bridge

# Docker
docker logs mattermost-matrix-bridge
```

### Common Issues

**Bridge not connecting to Mattermost:**
- Verify admin token is valid
- Check network connectivity
- Ensure Mattermost URL is correct

**Synapse not recognizing appservice:**
- Verify registration.yaml is correctly formatted
- Check Synapse logs for appservice errors
- Ensure tokens match between config and registration

**Messages not bridging:**
- Check bridge logs for errors
- Verify user permissions in bridge config
- Ensure Matrix user is in the bridged room

## Performance Tuning

For large deployments (>1000 users):

```yaml
database:
  type: postgres  # Use PostgreSQL instead of SQLite
  uri: postgres://user:pass@host/db

bridge:
  backfill:
    enable: false  # Disable for initial sync

network:
  mirror:
    history_limit: 100  # Reduce history backfill
```

## Security Considerations

- **Never commit secrets to git** - Use environment variables or secret managers
- **Use TLS/SSL** for all connections
- **Limit bridge permissions** to only required users
- **Regularly update** both the bridge and dependencies
- **Monitor logs** for suspicious activity

## Backup and Recovery

### Database Backup

```bash
# SQLite
cp /data/bridge.db /backup/bridge.db

# PostgreSQL
pg_dump -h localhost -U bridge bridge_db > bridge_backup.sql
```

### Configuration Backup

Back up your:
- `config.yaml`
- `registration.yaml`
- Any custom scripts or configurations

## Updating the Bridge

1. Pull the latest image:
   ```bash
   docker pull ghcr.io/hanthor/matrix-mattermost-bridge:latest
   ```

2. Restart the bridge:
   ```bash
   # Kubernetes
   kubectl rollout restart -n mautrix deployment/mattermost-matrix-bridge
   
   # Docker Compose
   docker-compose up -d --force-recreate bridge
   ```

3. Check logs for successful startup

## Support

- **Issues**: [GitHub Issues](https://github.com/hanthor/matrix-mattermost-bridge/issues)
- **Documentation**: See [SPEC.md](../SPEC.md) for technical details
- **Configuration**: See [example-config.yaml](../example-config.yaml)
