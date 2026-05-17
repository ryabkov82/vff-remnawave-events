# Production deployment

This service is expected to run on the Remnawave panel host in the same Docker network as the panel.

## Files

Recommended host layout:

```text
/opt/vff-remnawave-events/
  docker-compose.yml
  .env
  data/
```

`data/` must be writable by the container user `10001`.

```bash
sudo mkdir -p /opt/vff-remnawave-events/data
sudo chown -R 10001:10001 /opt/vff-remnawave-events/data
sudo chmod 750 /opt/vff-remnawave-events/data
```

## Compose

Copy `deploy/docker-compose.prod.yml` to:

```text
/opt/vff-remnawave-events/docker-compose.yml
```

The service is attached to the external Docker network:

```text
remnawave-network
```

The HTTP port is not published to the host. Remnawave should call the service internally:

```text
http://vff-remnawave-events:8080/remnawave
```

## Environment

Create `/opt/vff-remnawave-events/.env` from `deploy/env.production.example`.

Required secrets:

```env
WEBHOOK_SECRET_HEADER=<same value as in Remnawave panel>
TELEGRAM_BOT_TOKEN=<VPN for Friends Telegram bot token>
```

For production, keep dry-run disabled:

```env
MESSENGER_DRY_RUN=false
```

For smoke tests without Telegram API access, use:

```env
MESSENGER_DRY_RUN=true
```

## Start

```bash
cd /opt/vff-remnawave-events
docker compose up -d
```

## Health check

From the panel host:

```bash
docker exec vff-remnawave-events wget -qO- http://127.0.0.1:8080/healthz
```

Expected output:

```text
ok
```

From Remnawave container/network side, the service should be resolvable as:

```text
http://vff-remnawave-events:8080/healthz
```

## Remnawave panel env

Panel must be configured with:

```env
WEBHOOK_ENABLED=true
WEBHOOK_URL=http://vff-remnawave-events:8080/remnawave
WEBHOOK_SECRET_HEADER=<same secret as service>
```

After changing panel env, recreate/restart the Remnawave panel container according to the existing deployment process.

## Logs

```bash
docker logs -f vff-remnawave-events
```

Useful messages:

```text
ignored event scope=service event=service.panel_started
duplicate torrent blocker event key=... user=... node=...
telegram chat id not found user_id=... username=...
telegram send failed chat_id=... user_id=... error=...
telegram notification sent chat_id=... user_id=... username=... node=...
```
