# GATIE — IoT Gate Control SaaS

GATIE is a multi-tenant SaaS platform for centralized control of physical gates (barriers, doors, gates) via a web interface. IoT devices maintain persistent outbound MQTT connections to a cloud broker — no port-forwarding needed.

## Stack

| Layer | Technology |
|-------|-----------|
| Backend API | Go + Huma v2 + chi router |
| Database | PostgreSQL |
| Cache / Rate Limiting | Valkey (Redis-compatible) |
| IoT Broker | Eclipse Mosquitto (MQTT) |
| Reverse Proxy / TLS | Caddy (On-Demand TLS) |
| Frontend | React 19 + Vite + TypeScript |
| UI | Mantine v7 + Tailwind CSS v4 |
| State | TanStack Query v5 + Zustand |
| i18n | react-i18next (EN / FR) |
| Migrations | golang-migrate (SQL files) |
| Task runner | go-task (Taskfile.yml) |

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| Go ≥ 1.25 | Backend runtime |
| Node.js ≥ 20 + npm | Frontend toolchain |
| Docker Desktop | Infrastructure services |
| [go-task](https://taskfile.dev/installation/) | Task runner (`task` CLI) |
| [air](https://github.com/air-verse/air) | Go hot-reload (`go install github.com/air-verse/air@latest`) |
| [golang-migrate CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) | DB migrations |

---

## Quick Start (Development)

### 1 — Clone & configure environment

```bash
git clone https://github.com/Buco7854/gatie.git
cd gatie
cp .env.example .env   # edit if needed
```

Default `.env` values work out of the box for local dev:

```env
DATABASE_URL=postgres://gatie:gatie@localhost:5432/gatie?sslmode=disable
REDIS_URL=redis://localhost:6379
MQTT_BROKER=tcp://localhost:1883
JWT_SECRET=dev-secret-change-in-prod
BASE_URL=http://localhost:8080
FRONTEND_URL=http://localhost:5173
```

### 2 — Start infrastructure (Docker)

```bash
task dev-infra
```

This starts PostgreSQL, Valkey, Mosquitto, and Caddy via Docker Compose.

### 3 — Run database migrations

```bash
task migrate-up
```

### 4 — Start the backend (hot-reload)

```bash
task dev-api
```

The Go API will be available at `http://localhost:8080`.

### 5 — Install frontend dependencies (first time only)

```bash
cd frontend && npm install && cd ..
```

### 6 — Start the frontend dev server

```bash
task dev-frontend
```

The React app will be available at `http://localhost:5173` with HMR and automatic proxy to the API.

---

## Local Demo (5 minutes)

The quickest way to see GATIE working end-to-end with real gate simulations.

### Prerequisites check

```bash
docker info          # Docker must be running
task --version       # go-task installed
air -v               # air installed  (go install github.com/air-verse/air@latest)
migrate -version     # golang-migrate CLI installed
```

### Step 1 — Start everything

Open **4 terminals** in the project root:

```bash
# Terminal 1 — infrastructure
task dev-infra && task migrate-up

# Terminal 2 — backend API
task dev-api

# Terminal 3 — frontend (first time: cd frontend && npm install)
task dev-frontend
```

### Step 2 — Seed demo data

```bash
# Terminal 4
task demo-seed
```

This creates account `demo@gatie.local` / `Demo1234!`, workspace **Demo**, and 4 gates.
The command prints each gate's token and simulation instructions.

### Step 3 — Open the UI

**http://localhost:5173** → log in with `demo@gatie.local` / `Demo1234!`

You should see the **Demo** workspace with 4 gates, all in `unknown` state.

### Step 4 — Simulate a gate

**Option A — HTTP push** (Portail Principal, gate 1):

```bash
# copy token1 from the seed output, then:
task demo-sim -- --token=<token1>
```

Click **Open** or **Close** in the UI — the gate simulator receives the command and
pushes the new status back. You will see it update in real time via SSE.

**Option B — MQTT native** (Interphone, gate 3):

```bash
go run ./cmd/gatesim --mode=mqtt --token=<token3>
```

Same behaviour: click Open/Close in the UI, gatesim responds over MQTT.

**Option C — MQTT custom payload** (Barrière, gate 4):

Publish directly to the broker (e.g. with [MQTT Explorer](https://mqtt-explorer.com/) on `localhost:1883`):

```
topic  : workspace_<wsID>/gates/<gateID4>/status
payload: {"token":"<token4>","state":"open","voltage":12.3,"temp":22.5}
```

The `state` field is mapped to the gate status; `voltage` and `temp` appear as metadata.

### What to expect

| Gate | Driver | How to test |
|------|--------|-------------|
| Portail Principal | `HTTP_INBOUND` | `task demo-sim -- --token=<t>` |
| Garage | `NONE` | manual status (no simulator) |
| Interphone | `MQTT_GATIE` | `gatesim --mode=mqtt --token=<t>` |
| Barrière | `MQTT_CUSTOM` | publish JSON via MQTT client |

Battery rule on **Portail Principal**: push `{"status":"closed","battery":5}` (via the
HTTP simulator) → status overrides to `low_battery` automatically.

---

## Task Reference

Run `task --list` to see all available tasks.

| Task | Description |
|------|-------------|
| `task dev-infra` | Start Docker services (PG, Valkey, Mosquitto, Caddy) |
| `task dev-infra-down` | Stop Docker services |
| `task dev-api` | Start Go backend with hot-reload (air) |
| `task dev-frontend` | Start React dev server (Vite) |
| `task dev` | Start API + frontend concurrently |
| `task build` | Build Go binary to `bin/server` |
| `task build-frontend` | Build React app for production |
| `task build-all` | Build both backend and frontend |
| `task migrate-up` | Apply all pending migrations |
| `task migrate-down` | Roll back last migration |
| `task migrate-create -- <name>` | Create a new migration pair |
| `task demo-seed` | Provision demo account + workspace + 4 gates |
| `task demo-sim -- --token=<t>` | Run HTTP gate simulator (Portail Principal) |
| `task test` | Run Go unit tests (no infra required) |
| `task test-integration` | Run E2E tests against live server |
| `task lint` | Run golangci-lint + ESLint |

---

## Project Structure

```
gatie/
├── cmd/server/          # Go entry point (main.go)
├── internal/
│   ├── config/          # Environment config (Viper)
│   ├── handler/         # Huma HTTP handlers
│   ├── middleware/       # Auth, RBAC, tenant resolution
│   ├── model/           # Domain models (structs)
│   ├── repository/      # PostgreSQL queries
│   ├── service/         # Business logic
│   ├── mqtt/            # MQTT client
│   └── integration/     # Gate integration drivers (MQTT, HTTP, …)
├── migrations/          # golang-migrate SQL files
├── configs/             # Mosquitto config, Caddyfile
├── frontend/
│   └── src/
│       ├── api/         # Typed API service functions
│       ├── components/  # Shared UI components
│       ├── hooks/       # Custom React hooks
│       ├── i18n/        # Translations (EN, FR)
│       ├── layouts/     # AppLayout, AuthLayout
│       ├── lib/         # Axios client, utilities
│       ├── pages/       # Route-level page components
│       ├── store/       # Zustand stores
│       └── types/       # TypeScript type definitions
├── docker-compose.yml
├── Taskfile.yml
└── README.md
```

---

## Gate Integration System

Each gate can be configured independently for three actions:

| Action | Description | Supported drivers |
|--------|-------------|-------------------|
| **Status** | How to retrieve the gate's current state | `MQTT` (subscription), `NONE` |
| **Open** | How to send an "open" command | `MQTT`, `HTTP`, `NONE` |
| **Close** | How to send a "close" command | `MQTT`, `HTTP`, `NONE` |

The drivers are configured via `open_config`, `close_config`, and `status_config` JSONB fields on the gate.

**MQTT driver config example:**
```json
{ "type": "MQTT" }
```

**HTTP driver config example:**
```json
{
  "type": "HTTP",
  "config": {
    "url": "http://192.168.1.10/open",
    "method": "POST",
    "headers": { "Authorization": "Bearer token" },
    "body": "{\"action\": \"open\"}"
  }
}
```

---

## Authentication

Two JWT flows coexist:

- **Global login** (`POST /api/auth/login`) — email + password → `type=global` JWT with all workspace memberships
- **Local login** (`POST /api/auth/login/local`) — workspace slug + local username + password → `type=local` JWT scoped to one workspace

Guest access (no account) is handled via PIN codes on the public `/unlock` page.

---

## OpenAPI

The full OpenAPI 3.1 specification is auto-generated by Huma and available at:

```
http://localhost:8080/api/openapi.json
```

---

## Docker

```bash
# Build the production image (multi-stage: Go binary + React build)
docker build -t gatie .

# Run with the existing docker-compose (add the app service)
docker compose up -d
```

The Dockerfile produces a minimal Alpine image (~30MB) with:
- Compiled Go binary (no runtime needed)
- Pre-built React frontend (`frontend/dist/`)
- Migration files (`migrations/`)

---

## Security

- Passwords hashed with **bcrypt** (configurable policy: length, uppercase, lowercase, digit)
- **JWT access tokens** (15 min TTL) + refresh tokens (SHA-256 hashed in Redis)
- **SSRF protection** on HTTP gate drivers (private IP blocking with configurable allowlist)
- **Rate limiting** on auth endpoints (Redis-backed, fail-closed)
- **PIN brute-force protection** (per-IP and per-gate rate limits)
- **CORS** with explicit origins only (wildcard rejected)
- **HttpOnly + SameSite=Lax cookies** for session tokens
- **API tokens** prefixed (`gatie_`) and SHA-256 hashed at rest

---

## Development Tips

- **Backend hot-reload**: `air` watches all `.go` files and rebuilds automatically.
- **Database reset**: `task migrate-down && task migrate-up` (or `migrate -path migrations -database "$DATABASE_URL" drop -f && task migrate-up`)
- **MQTT testing**: Use [MQTT Explorer](https://mqtt-explorer.com/) to inspect topics on `localhost:1883`.
- **API tokens**: Format `gatie_<64 hex chars>`, stored as SHA-256 in the DB.

---

## License

[MIT](LICENSE)
