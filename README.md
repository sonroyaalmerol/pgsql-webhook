# pgsql-webhook

A lightweight Go service that listens to PostgreSQL NOTIFY events and forwards them as HTTP webhooks.

Perfect for bridging PostgreSQL triggers to webhooks, Node-RED, or any HTTP endpoint.

## Features

- 🚀 Lightweight single binary (~10MB Docker image)
- 🔄 Auto-reconnects on database disconnect
- 🔁 Built-in retry logic for webhook delivery
- 📝 Clear logging of all events
- 🐳 Docker-ready with multi-arch support
- ⚡ Real-time event forwarding (no polling)

## Quick Start

### Docker

```bash
docker run -d \
  -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=disable" \
  -e WEBHOOK_URL="http://your-endpoint/webhook" \
  -e CHANNEL="your_channel" \
  ghcr.io/sonroyaalmerol/pgsql-webhook:latest
```

### Docker Compose

```yaml
version: '3.8'

services:
  pgsql-webhook:
    image: ghcr.io/sonroyaalmerol/pgsql-webhook:latest
    environment:
      - DATABASE_URL=postgres://user:pass@postgres:5432/db?sslmode=disable
      - WEBHOOK_URL=http://node-red:1880/webhook
      - CHANNEL=my_notifications
    restart: unless-stopped
```

## Configuration

All configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://authentik:password@localhost:5432/authentik?sslmode=disable` |
| `WEBHOOK_URL` | HTTP endpoint to send webhooks to | `http://localhost:1880/authentik-webhook` |
| `CHANNEL` | PostgreSQL NOTIFY channel to listen on | `authentik_changes` |

## PostgreSQL Setup

Create a trigger function that sends notifications:

```sql
CREATE OR REPLACE FUNCTION notify_changes()
RETURNS trigger AS $$
DECLARE
  payload json;
BEGIN
  IF (TG_OP = 'DELETE') THEN
    payload = json_build_object(
      'operation', TG_OP,
      'timestamp', NOW(),
      'table', TG_TABLE_NAME,
      'data', row_to_json(OLD)
    );
  ELSE
    payload = json_build_object(
      'operation', TG_OP,
      'timestamp', NOW(),
      'table', TG_TABLE_NAME,
      'data', row_to_json(NEW),
      'old_data', CASE WHEN TG_OP = 'UPDATE' THEN row_to_json(OLD) ELSE NULL END
    );
  END IF;

  PERFORM pg_notify('your_channel', payload::text);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
```

Attach the trigger to your table:

```sql
CREATE TRIGGER my_table_trigger
AFTER INSERT OR UPDATE OR DELETE ON my_table
FOR EACH ROW EXECUTE FUNCTION notify_changes();
```

## Webhook Payload

The service sends POST requests with JSON payloads:

```json
{
  "operation": "INSERT",
  "timestamp": "2025-10-07T14:00:00Z",
  "table": "my_table",
  "data": {
    "id": 123,
    "name": "example"
  },
  "old_data": null
}
```

For `UPDATE` operations, `old_data` contains the previous row state.

## Use Cases

### Authentik Group Changes

Monitor group membership and attribute changes:

```sql
-- Membership changes
CREATE TRIGGER group_membership_trigger
AFTER INSERT OR UPDATE OR DELETE ON authentik_core_user_ak_groups
FOR EACH ROW EXECUTE FUNCTION notify_changes();

-- Group metadata changes
CREATE TRIGGER group_metadata_trigger
AFTER INSERT OR UPDATE OR DELETE ON authentik_core_group
FOR EACH ROW EXECUTE FUNCTION notify_changes();
```

### Node-RED Integration

Simply add an HTTP In node listening on the webhook path:
