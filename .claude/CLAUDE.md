# GATY - Project Context

## Project
- Repo: github.com/Buco7854/gaty
- Local: C:\Users\Micro\Documents\Projects\gaty
- Windows dev environment (Git Bash + PowerShell), no Makefile

## Stack
- Backend: Go + Huma v2 + chi router
- DB: PostgreSQL + Valkey (Redis-compatible) + golang-migrate (SQL files)
- IoT: Eclipse Mosquitto (MQTT)
- Proxy: Caddy (On-Demand TLS)
- Frontend: React + Vite + TS + shadcn/ui + Tailwind CSS v4
- Task runner: go-task (Taskfile.yml)

## Go module
- Module: github.com/Buco7854/gaty
- Entry: cmd/server/main.go
- Internal packages: config, handler, middleware, model, repository, service, mqtt

## Dev workflow
- `task dev-infra` → docker compose up -d (start Docker Desktop first)
- `task dev-api` → air (Go hot-reload)
- `task dev-frontend` → vite dev server
- `task migrate-up/down` → golang-migrate

## Dev credentials
- Postgres: postgres://gaty:gaty@localhost:5432/gaty?sslmode=disable
- Valkey: redis://localhost:6379
- MQTT: tcp://localhost:1883

## Avancement
L'avancement se suit dans **plan.md** à la racine du repo.

## User preferences
- No co-authored commits
- No Makefile, uses go-task (Taskfile.yml)
- Communication en français
- Commits et pushes uniquement si explicitement demandé

---

# Huma v2 - Notes clés

Source: https://huma.rocks/features/

## Deux types de middleware

### 1. Router-specific (chi)
Appliqué AVANT la création de l'API Huma. S'applique à toutes les routes.
```go
router := chi.NewMux()
router.Use(myChiMiddleware)
api := humachi.New(router, config)
```

### 2. Router-agnostic (Huma natif)
Signature : `func(ctx huma.Context, next func(huma.Context))`

**Global** (toutes les routes enregistrées APRÈS) :
```go
api.UseMiddleware(MyMiddleware) // AVANT les huma.Register !
huma.Get(api, "/path", handler)
```

**Par opération** :
```go
huma.Register(api, huma.Operation{
    Middlewares: huma.Middlewares{MyMiddleware},
}, handler)
```

### Context values dans Huma middleware
```go
// Écrire
ctx = huma.WithValue(ctx, "key", value)
next(ctx)

// Lire dans un autre middleware Huma
ctx.Context().Value("key")

// Lire dans un handler (ctx context.Context)
ctx.Value("key")
```

### Lire un header dans Huma middleware
```go
ctx.Header("Authorization")  // sur huma.Context
```

### Stopper + erreur dans Huma middleware
```go
huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
return // ne pas appeler next
```

## Opérations
```go
huma.Register(api, huma.Operation{
    OperationID: "unique-id",
    Method:      http.MethodGet,
    Path:        "/path/{id}",
    Summary:     "...",
    Tags:        []string{"Tag"},
    DefaultStatus: 201, // optionnel
    Middlewares: huma.Middlewares{myMw},
}, handler)
```

Raccourcis : `huma.Get`, `huma.Post`, `huma.Put`, `huma.Patch`, `huma.Delete`

## Input - tags disponibles
| Tag | Description |
|-----|-------------|
| `path:"name"` | Path parameter |
| `query:"name"` | Query string |
| `header:"name"` | Header |
| `cookie:"name"` | Cookie |
| `required:"true"` | Obligatoire (headers/query/cookies) |
| `minLength:"8"` | Validation |
| `format:"email"` | Format JSON Schema |

## Output
```go
type MyOutput struct {
    ContentType string `header:"Content-Type"`
    Body struct {
        ID   uuid.UUID `json:"id"`
        Name string    `json:"name"`
    }
}
```
- 200 si `Body` présent, 204 si nil
- Override avec `DefaultStatus: 201` dans l'opération

## Erreurs standard Huma
```go
huma.Error400BadRequest("message")
huma.Error401Unauthorized("message")
huma.Error403Forbidden("message")
huma.Error404NotFound("message")
huma.Error409Conflict("message")
huma.Error500InternalServerError("message")
```

## Pattern auth (projet GATY)
```go
// Global (soft) - extrait le token si présent
api.UseMiddleware(middleware.AuthExtractor(authSvc))

// Par opération (hard) - rejette si pas authentifié
requireAuth := middleware.RequireAuth(api)
huma.Register(api, huma.Operation{
    Middlewares: huma.Middlewares{requireAuth},
}, handler)
```
