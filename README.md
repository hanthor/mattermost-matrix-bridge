# Mattermost-Matrix Bridge E2E Testing Environment

## Quick Start

### 1. Clean Start (Fresh Slate)
```bash
./cleanup.sh --full  # Full wipe including databases
# OR
./cleanup.sh         # Soft cleanup (preserves data)
```

### 2. Setup Environment
```bash
./setup.sh           # Starts all services with smart port detection
```

### 3. Provision Mattermost
```bash
./provision_mm.sh    # Creates admin user, team, channel, and configures bridge
```

## Scripts Overview

| Script | Purpose |
|--------|---------|
| `cleanup.sh` | Reset environment (use `--full` for complete wipe) |
| `setup.sh` | Start all Docker services with port conflict detection |
| `provision_mm.sh` | Configure Mattermost and bridge connection |

## Access URLs

After running `setup.sh` and `provision_mm.sh`:

- **Mattermost**: http://localhost:8065
  - User: `sysadmin` / `Sys@dmin123`
  - Team: "Test Team" → "test-channel"

- **Element** (Matrix Client): http://localhost:8080
  - Create account or login

- **Synapse** (Matrix Server): http://localhost:8008

> **Note:** Ports may differ if defaults are in use. Check script output for actual URLs.

## Testing the Bridge

1. Login to Mattermost at http://localhost:8065
2. Navigate to "Test Team" → "test-channel"
3. Send a message
4. Open Element at http://localhost:8080 and create/login to Matrix account
5. Look for the bridged room (`#mattermost_test-channel:localhost`)
6. Verify message appears in Matrix

## Troubleshooting

### View Logs
```bash
# All services
docker compose logs

# Specific service
docker compose logs bridge
docker compose logs synapse
docker compose logs mattermost
```

### Check Status
```bash
docker compose ps
```

### Full Reset
```bash
./cleanup.sh --full
./setup.sh
./provision_mm.sh
```

### Port Conflicts
The `setup.sh` script automatically detects port conflicts and uses alternative ports.
Check the script output or `.env.urls` file for actual URLs.

## File Structure

- `docker-compose.yaml` - Service definitions
- `config.yaml` - Bridge configuration (generated)
- `registration.yaml` - Appservice registration (generated)
- `synapse-data/` - Synapse homeserver data
- `.env.urls` - Current service URLs (generated)

## Documentation

See `TESTING.md` for detailed testing procedures and manual test scenarios.
