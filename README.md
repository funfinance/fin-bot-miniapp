# Finance Bot Mini App

A Telegram bot with a Mini App UI for tracking personal expenses and income. Records are stored per-user in a local SQLite database.

## Features

- Record expenses and income via a Telegram Mini App form
- Recurring expenses — weekly or monthly schedules, multi-day support (e.g. 1st and 15th), auto-triggered daily by a background scheduler
- Multiple ledgers (account books) per user
- Custom categories with emoji
- Multi-currency support with automatic conversion to a base currency
- Monthly / yearly statistics
- Export records to Excel (file sent directly in Telegram chat)
- Browse history by year → month → record list, with inline edit and delete
- Undo last record
- Per-user data isolation — multiple users can share one bot instance

## Architecture

- **Bot**: Telegram bot (telebot v3) — handles commands, sends Mini App inline buttons, delivers notifications and files
- **API server**: HTTP server serving both static Mini App files and a REST API
- **Auth**: Every API request is authenticated via Telegram `initData` HMAC-SHA256 signature — no login required
- **Database**: SQLite via GORM, all queries scoped to `user_id`
- **Frontend**: Plain HTML/JS Mini App pages embedded into the binary via `go:embed`
- **Scheduler**: Background goroutine runs daily at midnight, triggers due recurring expenses and records them automatically

```
fin-bot-miniapp/
├── cmd/bot/              # Entry point
├── internal/
│   ├── api/              # HTTP server, REST handlers, auth middleware
│   ├── bot/              # Telegram bot handler, Notifier implementation
│   ├── service/          # Business logic (expense, category, ledger, rate, recurring scheduler)
│   ├── repository/       # Data access layer
│   ├── model/            # Database models
│   ├── config/           # Configuration
│   ├── database/         # SQLite connection
│   └── logger/           # Logger
├── webfs/
│   └── web/
│       ├── app/          # Mini App HTML pages
│       └── static/       # Shared JS
└── configs/
    └── config.yaml
```

## Requirements

- Go 1.22+
- CGO enabled (required by SQLite driver)
- A public HTTPS URL for the Mini App (Cloudflare Tunnel, or your own domain with TLS)

## Setup

### 1. Create a Telegram bot

Talk to [@BotFather](https://t.me/BotFather), create a bot, and copy the token.

### 2. Configure

Edit `configs/config.yaml`:

```yaml
bot:
  token: "YOUR_BOT_TOKEN"         # Required

server:
  addr: ":8080"
  mini_app_url: "https://your-public-url.com"  # Required, must be HTTPS
  init_data_ttl: 24h                           # How long initData stays valid

database:
  path: "./data/finbot.db"

logger:
  level: "info"                   # debug | info | warn | error
  file: "./logs/bot.log"

rate:
  base_currency: "JPY"            # Base currency for conversions and statistics. However, when you modify the base, you must configure the API key. 
  update_interval: 24h
  api_key: "YOUR_API_KEY"         # Optional, from exchangerate-api.com
  supported_currencies:           # Currencies shown in the Mini App dropdown; base_currency is always prepended automatically
    - USD
    - EUR
    - CNY
```

### 3. Run locally

```bash
make setup      # create data/ and logs/ directories (run once)
make install    # download dependencies
make run        # build and start
```

Expose the local server with a tunnel:

```bash
cloudflared tunnel --url http://localhost:8080 --protocol http2
```

Copy the printed HTTPS URL into `mini_app_url` in your config and restart.

### 4. Run with Docker

```bash
docker compose up -d
```

The container mounts `./configs/config.yaml`, `./data/`, and `./logs/` from the host.

## Bot Commands

| Command | Description |
|---|---|
| `/add` | Open Mini App to record an expense or income |
| `/stats` | View statistics for a date range |
| `/export` | Export records to Excel (file sent in chat) |
| `/undo` | Delete the most recent record |
| `/history` | Browse and edit past records by year/month |
| `/addcat <code> <emoji> <name>` | Add a custom category |
| `/addledger <code> <emoji> <name>` | Add a custom ledger |
| `/help` | Show command list |

Examples:
```
/addcat gym 🏋️ Gym
/addledger savings 💰 Savings
```

## Development

```bash
make test           # run tests
make test-coverage  # run tests with HTML coverage report
make fmt            # format code
make lint           # lint (requires golangci-lint)
make clean          # remove build artifacts
```

## Tech Stack

- **Language**: Go 1.22
- **Bot Framework**: [telebot v3](https://github.com/tucnak/telebot)
- **Database**: SQLite via [GORM](https://gorm.io/)
- **Excel**: [excelize](https://github.com/360EntSecGroup-Skylar/excelize)
- **Config**: YAML
- **Exchange Rates**: [exchangerate-api.com](https://www.exchangerate-api.com/) (optional)

## Design Notes

All data is stored on your own server and communication goes through Telegram's encrypted channels. The SQLite database is directly accessible — you can connect [Metabase](https://www.metabase.com/) or any SQL tool to `data/finbot.db` for custom analysis.

## Integrations

### Metabase (Visual Analytics)

The bot stores all records in a local SQLite file (`data/finbot.db`). You can connect [Metabase](https://www.metabase.com/) directly to this file for richer visualizations — charts, dashboards, and custom queries — without any changes to the bot itself.

This keeps the design decoupled: Telegram handles convenient data entry, Metabase handles analysis.

**Setup:**

1. [Download and run Metabase](https://www.metabase.com/start/oss/) (free, self-hosted)
2. Add a new database → select **SQLite** → point it to your `data/finbot.db` file
3. Start building questions and dashboards on the `expenses` table

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
