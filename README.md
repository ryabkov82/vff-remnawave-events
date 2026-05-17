# vff-remnawave-events

Production webhook consumer for VPN for Friends Remnawave events.

## Purpose

The service receives Remnawave webhooks, verifies the HMAC signature, filters `torrent_blocker.report` events, deduplicates reports, resolves the affected user to a Telegram chat ID, and sends a soft notification about temporary VPN blocking caused by torrent/P2P traffic.

## MVP scope

- `POST /remnawave` endpoint for Remnawave webhooks.
- `GET /healthz` endpoint for container health checks.
- HMAC-SHA256 verification using `X-Remnawave-Signature` and `WEBHOOK_SECRET_HEADER`.
- Timestamp freshness check using `X-Remnawave-Timestamp`.
- Event filter:
  - `scope == "torrent_blocker"`
  - `event == "torrent_blocker.report"`
  - `data.report.actionReport.blocked == true`
- SQLite-based deduplication.
- Telegram recipient resolution from:
  - `data.user.telegramId`
  - numeric `login=@123456789` value inside `data.user.description`
- Telegram notification through Bot API `sendMessage`.

## Configuration

Copy `.env.example` to `.env` and fill secrets.

```env
LISTEN_ADDR=:8080
WEBHOOK_SECRET_HEADER=change-me
WEBHOOK_MAX_CLOCK_SKEW_SECONDS=300
SQLITE_PATH=/data/events.db
DEDUP_TTL_HOURS=168
TELEGRAM_BOT_TOKEN=change-me
TELEGRAM_PARSE_MODE=HTML
RESOLVER_DESCRIPTION_LOGIN_ENABLED=true
```

## Local run

```bash
go mod tidy
go run ./cmd/vff-remnawave-events
```

## Docker

```bash
docker build -t vff-remnawave-events:local .
docker run --rm --env-file .env -p 8080:8080 -v "$PWD/data:/data" vff-remnawave-events:local
```

## Remnawave panel env

```env
WEBHOOK_ENABLED=true
WEBHOOK_URL=http://vff-remnawave-events:8080/remnawave
WEBHOOK_SECRET_HEADER=<same-secret-as-service>
```

When deployed into the same Docker network as Remnawave, the webhook endpoint does not need to be exposed publicly.
