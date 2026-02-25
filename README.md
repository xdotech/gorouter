# GoRouter

**Auto-route to free & cheap AI models with smart fallback.**

A lightweight, single-binary AI routing gateway written in Go. GoRouter sits between your coding tools and AI providers — handling format translation, quota tracking, automatic fallback, and OAuth token refresh so you never hit a wall mid-session.

```
Your CLI tool (Claude Code / Cursor / Cline / Codex...)
    │  POST /v1/chat/completions
    ▼
┌───────────────────────── GoRouter ──────────────────────────┐
│  • Format translation: OpenAI ↔ Claude ↔ Gemini             │
│  • Smart routing: resolve model → select account             │
│  • Account fallback: 429/503 → next account (backoff)        │
│  • Combo fallback: quota out → next model in chain           │
│  • OAuth token auto-refresh                                  │
│  • Usage tracking & cost estimation                          │
└──────────────────────────────────────────────────────────────┘
    │
    ├─→ Claude Code (OAuth, subscription)
    ├─→ Gemini CLI (OAuth, 180K free/month)
    ├─→ GitHub Copilot (OAuth)
    ├─→ iFlow (OAuth, unlimited free)
    ├─→ Qwen (device code, unlimited free)
    ├─→ GLM / MiniMax / Kimi (API key, cheap)
    └─→ Any OpenAI-compatible endpoint
```

---

## Quick Start

```bash
git clone <repo> && cd gorouter
make build
./gorouter
```

Point your CLI tool at `http://localhost:14747/v1` — done.

---

## Installation

**From source:**

```bash
cp .env.example .env   # edit secrets
make build
./gorouter
```

**Docker:**

```bash
docker compose up
```

```bash
# Or manually
docker build -t gorouter .
docker run -d \
  --name gorouter \
  -p 14747:14747 \
  -v gorouter-data:/app/data \
  -e JWT_SECRET=your-secret \
  gorouter
```

---

## Configuration

All settings via environment variables or `.env` file:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `14747` | Listen port |
| `HOSTNAME` | `0.0.0.0` | Bind address |
| `JWT_SECRET` | _(weak default)_ | Dashboard session secret — **change in production** |
| `INITIAL_PASSWORD` | `123456` | First login password |
| `DATA_DIR` | `~/.gorouter` | Config + usage storage directory |
| `API_KEY_SECRET` | _(default)_ | HMAC secret for API key generation |
| `REQUIRE_API_KEY` | `false` | Enforce Bearer key on `/v1/*` routes |
| `ENABLE_REQUEST_LOGS` | `false` | Log full request/response bodies |
| `BASE_URL` | `http://localhost:14747` | Public base URL (used in OAuth callbacks) |
| `DASHBOARD_URL` | _(empty)_ | Proxy dashboard to this URL (dev mode) |
| `HTTP_PROXY` / `HTTPS_PROXY` | _(empty)_ | Outbound proxy for upstream calls |

---

## Providers

### OAuth Providers

Connect via dashboard → `/api/oauth/{provider}/authorize`

| Key | Provider | Cost |
|-----|----------|------|
| `cc` | Claude Code | Pro/Max subscription |
| `gc` | Gemini CLI | Free (180K tokens/month) |
| `gh` | GitHub Copilot | $10–19/mo |
| `if` | iFlow | Free (unlimited) |
| `qw` | Qwen | Free (unlimited) |

### API Key Providers

Add key via dashboard → Providers

| Key | Provider | Cost |
|-----|----------|------|
| `glm` | Zhipu GLM-4.7 | ~$0.6/1M tokens |
| `minimax` | MiniMax M2.1 | ~$0.2/1M tokens |
| `kimi` | Moonshot Kimi | $9/mo flat |
| `openai` | OpenAI | Pay per use |
| `anthropic` | Anthropic direct | Pay per use |
| `openrouter` | OpenRouter | Pay per use |

Model format: `{provider}/{model}` — e.g. `cc/claude-opus-4-6`, `if/kimi-k2-thinking`

---

## Combos — Auto Fallback Chains

Create a named combo that automatically falls through to the next model when quota runs out:

```bash
curl -X POST http://localhost:14747/api/combos \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-stack",
    "models": [
      "cc/claude-opus-4-6",
      "glm/glm-4.7",
      "if/kimi-k2-thinking"
    ]
  }'
```

Use `my-stack` as the model name in any client — GoRouter handles the rest.

**Example strategies:**

```
# Zero cost
gc/gemini-2.5-pro → if/kimi-k2-thinking → qw/qwen3-coder-plus

# Maximize subscription
cc/claude-opus-4-6 → glm/glm-4.7 → if/kimi-k2-thinking

# Always on (5 layers)
cc/claude-opus-4-6 → gh/claude-4.5-sonnet → glm/glm-4.7 → minimax/MiniMax-M2.1 → if/kimi-k2-thinking
```

---

## Fallback Behavior

**Account fallback:** When one account hits a rate limit, GoRouter automatically tries the next account for the same provider.

**Backoff levels:** `1min → 5min → 30min → 2h → 8h`

**Fallback strategy** (configurable):
- `fill-first` _(default)_ — exhaust highest-priority account before moving on
- `round-robin` — distribute across accounts with a sticky window

---

## Format Translation

GoRouter automatically translates between formats — no client changes needed:

| Client sends | Provider receives |
|-------------|------------------|
| OpenAI `messages[]` | Claude `messages[]` + `system` |
| OpenAI `tools[]` | Gemini `functionDeclarations[]` |
| Claude SSE stream | OpenAI `data: {"choices":[...]}` |
| Gemini SSE stream | OpenAI `data: {"choices":[...]}` |

---

## API Reference

### AI Routing (OpenAI-compatible)

```
POST /v1/chat/completions    OpenAI format
POST /v1/messages            Claude Messages API format
POST /v1/responses           OpenAI Responses API format
GET  /v1/models              List all providers + combos
GET  /v1beta/models          Gemini-style model list
```

**Example:**

```bash
curl http://localhost:14747/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "my-stack",
    "messages": [{"role": "user", "content": "Write a Go HTTP server"}],
    "stream": true
  }'
```

### Management API

```
POST /api/auth/login
POST /api/auth/logout
GET/PUT  /api/settings

GET/POST        /api/providers
GET/PUT/DELETE  /api/providers/{id}
POST            /api/providers/{id}/test

GET/POST        /api/combos
GET/PUT/DELETE  /api/combos/{id}

GET/POST   /api/keys
DELETE     /api/keys/{id}

GET/POST   /api/models/alias
DELETE     /api/models/alias/{alias}

GET /api/usage/providers      # stats by provider
GET /api/usage/request-logs   # paginated history
GET /api/usage/stream         # SSE live feed

GET/PUT /api/pricing

GET  /api/oauth/{provider}/authorize
GET  /api/oauth/{provider}/callback
POST /api/oauth/{provider}/device-code
POST /api/oauth/{provider}/poll
```

---

## CLI Tool Integration

**Claude Code** — `~/.claude/settings.json`:
```json
{
  "anthropic_api_base": "http://localhost:14747/v1",
  "anthropic_api_key": "your-gorouter-api-key"
}
```

**Cursor / Cline / Continue / RooCode:**
```
Provider:  OpenAI Compatible
Base URL:  http://localhost:14747/v1
API Key:   your-gorouter-api-key
Model:     cc/claude-opus-4-6  (or any combo name)
```

**Codex CLI:**
```bash
export OPENAI_BASE_URL="http://localhost:14747/v1"
export OPENAI_API_KEY="your-gorouter-api-key"
```

---

## Storage

Data stored locally at `$DATA_DIR` (default `~/.gorouter`):

```
~/.gorouter/
├── db.json      # providers, combos, API keys, settings, aliases
├── usage.json   # usage history (last 1000 entries)
└── log.txt      # one-line request log
```

---

## Development

```bash
make build        # compile binary
make run          # build + run on :14747
make test         # run tests
make test-cover   # coverage report
make docker       # build Docker image
make clean        # remove build artifacts
```

**Dashboard (dev mode)** — proxy to the JS frontend:
```bash
# Start JS frontend on :3000 separately
DASHBOARD_URL=http://localhost:3000 ./gorouter
```

---

## Project Structure

```
cmd/gorouter/          Entrypoint
internal/
  config/              Environment variable loading
  server/              chi HTTP server, CORS, dashboard handler
  db/                  JSON file store (db.json, mutex-protected)
  auth/                JWT sessions, bcrypt passwords, HMAC API keys
  translator/          Format translation (OpenAI ↔ Claude ↔ Gemini)
    request/           Request body translators
    response/          SSE stream translators
  executor/            Per-provider HTTP clients + token refresh
  router/              Routing core: model resolve, fallback, backoff
  api/                 Management REST API handlers (/api/*)
  oauth/               OAuth PKCE flows (cc / gc / gh / if / qw)
  usage/               Usage DB, cost estimation, logger, SSE broadcast
```

---

## License

MIT
