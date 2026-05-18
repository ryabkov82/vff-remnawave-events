# vff-remnawave-events

Production webhook consumer for VPN for Friends Remnawave events.

## Purpose

The service receives Remnawave webhooks, verifies the HMAC signature, filters `torrent_blocker.report` events, deduplicates reports, resolves the affected user to a Telegram chat ID, and sends a soft notification about temporary VPN blocking caused by torrent/P2P traffic.

It can send notifications through different Telegram bots depending on the SHM service category. This is needed because VPN for Friends has multiple Telegram bots and users may be attached to different SHM service categories.

## Processing flow

```text
Remnawave torrent_blocker.report
→ POST /remnawave
→ verify X-Remnawave-Signature and X-Remnawave-Timestamp
→ filter only blocked torrent_blocker.report events
→ SQLite deduplication
→ resolve recipient chat_id
→ parse Remnawave username us_<user_service_id>
→ lookup SHM user service category
→ cache user_service_id → category in memory
→ select Telegram bot token by category
→ send Telegram notification
```

Example mapping:

```text
user.username = us_301
→ user_service_id = 301
→ SHM category = vpn-mz-test
→ Telegram bot token for vpn-mz-test
```

## Scope

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
- Optional SHM category lookup from Remnawave username `us_<user_service_id>`.
- In-memory cache for SHM `user_service_id → category` lookups.
- Telegram sender routing by SHM service category.
- Fallback to the default `TELEGRAM_BOT_TOKEN` when category routing is not configured.

## Notification text

The user receives a soft warning that VPN access was temporarily limited because torrent/P2P traffic was detected. The message also explains that torrents may be used only when VPN is configured as a proxy for selected applications, not as a full-tunnel VPN, so the torrent client does not use the VPN connection.

## Configuration

Copy `deploy/env.production.example` or `.env.example` to `.env` and fill secrets.

Minimal configuration with one default Telegram bot:

```env
LISTEN_ADDR=:8080
WEBHOOK_SECRET_HEADER=change-me
WEBHOOK_MAX_CLOCK_SKEW_SECONDS=300
SQLITE_PATH=/data/events.db
DEDUP_TTL_HOURS=168
TELEGRAM_BOT_TOKEN=change-me
TELEGRAM_PARSE_MODE=HTML
MESSENGER_DRY_RUN=false
RESOLVER_DESCRIPTION_LOGIN_ENABLED=true
```

Category-based Telegram sender routing:

```env
SHM_ADMIN_BASE_URL=https://admin.vpn-for-friends.com
SHM_ADMIN_LOGIN=change-me
SHM_ADMIN_PASSWORD=change-me
SHM_REQUEST_TIMEOUT_SECONDS=10
SHM_SERVICE_CATEGORY_CACHE_TTL_HOURS=24
TELEGRAM_BOT_TOKENS_BY_CATEGORY_JSON={"vpn-mz-test":"change-me-antiblock-bot-token","vpn-mz-fc":"change-me-standard-bot-token"}
```

`TELEGRAM_BOT_TOKEN` may still be set as a fallback. If no category mapping is configured, the service uses only `TELEGRAM_BOT_TOKEN`.

## SHM authentication

The service authenticates to SHM the same way as the VPN for Friends bot:

```text
POST /shm/user/auth.cgi
body: {"login":"...","password":"..."}
→ response.session_id
→ cookie session_id is used for subsequent admin API requests
```

For service category lookup the service calls:

```text
GET /shm/v1/admin/user/service?filter={"user_service_id":301}
```

The category is read from `data[0].services.category`, with fallback to `data[0].category`.

## Cache behavior

SHM service category lookups are cached in memory by `user_service_id`.

Default TTL:

```env
SHM_SERVICE_CATEGORY_CACHE_TTL_HOURS=24
```

A cache miss produces a log like:

```text
SHM service category cached user_service_id=301 category=vpn-mz-test ttl=24h0m0s
```

A cache hit produces a log like:

```text
SHM service category cache hit user_service_id=301 category=vpn-mz-test
```

The cache is in-memory only and is cleared on container restart.

## Dry-run mode

Use dry-run for safe production testing without sending Telegram messages:

```env
MESSENGER_DRY_RUN=true
```

In dry-run mode the service logs the target `chat_id` and message text instead of calling Telegram Bot API.

## Local run

```bash
go mod tidy
go run ./cmd/vff-remnawave-events
```

## Build

```bash
make build
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

`WEBHOOK_SECRET_HEADER` must contain only letters and numbers for Remnawave 2.7.x. A safe generation command is:

```bash
openssl rand -hex 32
```

Do not use base64 for this value because base64 may contain `/`, `+`, or `=`.

When deployed into the same Docker network as Remnawave, the webhook endpoint does not need to be exposed publicly.

## Useful logs

```bash
docker logs --tail=120 vff-remnawave-events
```

Expected successful route with SHM category lookup:

```text
SHM service category cached user_service_id=301 category=vpn-mz-test ttl=24h0m0s
messenger route category=vpn-mz-test user_service_id=301 username=us_301
telegram notification sent chat_id=783272415 user_id=7280 username=us_301 node=nl-ams-3
```
