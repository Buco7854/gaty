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
- Frontend: React + Vite + TS + Mantine + Tailwind CSS v4
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

---

## Architecture : Users & Memberships

> **Refactor en cours (Phase R)** — L'ancienne architecture Users vs Members a été remplacée.
> Voir plan.md Phase R pour le détail du refactor.

### Principe central

Tout accès à un workspace passe par `workspace_memberships`. Cette table est la pièce maîtresse :
- Un **User** (compte plateforme) peut avoir une ou plusieurs memberships, liées via `user_id`
- Un **membre managé** (ajouté par un admin) est une membership sans `user_id` — il s'authentifie localement avec `local_username` + `local_password_hash`
- La **fusion** (merge) = un membre managé lie son `user_id` via un simple UPDATE atomique — ses permissions restent intactes car elles sont attachées au `membership_id`, pas à l'identité

### Schema DB cible

```
-- Compte plateforme
users (id, email UNIQUE NOT NULL, created_at)
  └─► credentials (
          id UUID PK,
          user_id FK → users.id CASCADE,
          type CHECK ('PASSWORD','SSO_IDENTITY','API_TOKEN'),
          hashed_value TEXT,
          label TEXT,           -- libellé pour les API tokens
          expires_at TIMESTAMPTZ,
          metadata JSONB,       -- provider/sub pour SSO, scopes pour tokens
          created_at
          -- UNIQUE (user_id) WHERE type = 'PASSWORD'  → un seul mot de passe
          -- API_TOKEN : plusieurs lignes autorisées par user
      )

-- Workspace
workspaces (
    id, slug UNIQUE NOT NULL, name, owner_id → users.id,
    sso_settings JSONB,        -- { issuer, client_id, client_secret, ... }
    member_auth_config JSONB,   -- config par défaut pour les membres de ce workspace :
                                -- { "password": true, "sso": true, "api_token": true,
                                --   "api_token_max": 5, "sso_auto_provision": true }
    created_at
)

-- Table pivot : toute personne ayant accès à un workspace
workspace_memberships (
    id UUID PK,
    workspace_id FK → workspaces.id CASCADE,
    user_id FK → users.id SET NULL,     -- null = membre managé sans compte plateforme
    local_username TEXT,                 -- identifiant local au workspace
    display_name TEXT,
    role CHECK ('OWNER','ADMIN','MEMBER'),
    auth_config JSONB,                   -- surcharge par-membre des defaults du workspace :
                                         -- { "password": null,   ← hérite du workspace
                                         --   "sso": false,      ← désactivé pour ce membre
                                         --   "api_token": true } ← activé même si workspace=false
    invited_by UUID → users.id,
    created_at,
    CHECK (user_id IS NOT NULL OR local_username IS NOT NULL),
    UNIQUE (workspace_id, user_id) WHERE user_id IS NOT NULL,
    UNIQUE (workspace_id, local_username) WHERE local_username IS NOT NULL
)
  └─► membership_credentials (
          id UUID PK,
          membership_id FK → workspace_memberships.id CASCADE,
          type CHECK ('PASSWORD','SSO_IDENTITY','API_TOKEN'),
          hashed_value TEXT,
          label TEXT,
          expires_at TIMESTAMPTZ,
          metadata JSONB,       -- même sémantique que credentials
          created_at
          -- UNIQUE (membership_id) WHERE type = 'PASSWORD'
          -- API_TOKEN : plusieurs lignes autorisées par membership
      )

gates (id, workspace_id, name, integration_type, integration_config, status, last_seen_at, created_at)

gate_pins (id, gate_id → gates.id CASCADE, hashed_pin, label, metadata JSONB, created_at)

permissions (code PK, description)

membership_policies (
    membership_id FK → workspace_memberships.id CASCADE,
    gate_id FK → gates.id CASCADE,
    permission_code FK → permissions.code CASCADE,
    PRIMARY KEY (membership_id, gate_id, permission_code)
)
```

### Vecteurs d'accès aux Gates
| Vecteur | Mécanisme |
|---------|-----------|
| OWNER/ADMIN du workspace | `workspace_memberships.role` |
| MEMBER avec policy | `membership_policies` lié au `membership_id` |
| PIN code public | `gate_pins` (table dédiée, bcrypt) |
| Token API | `credentials` (type=API_TOKEN, lié à `users.id`) |

### Authentification : deux chemins JWT

**Login global** (`POST /api/auth/login`) — email + password → user plateforme
```json
{ "sub": "<user_id>", "type": "global",
  "memberships": [{"workspace_id": "...", "membership_id": "...", "role": "MEMBER"}],
  "exp": ... }
```

**Login local** (`POST /api/auth/login/local`) — workspace_slug + local_username + password (ou SSO workspace)
```json
{ "sub": "<membership_id>", "type": "local",
  "workspace_id": "...", "role": "MEMBER", "exp": ... }
```

Refresh tokens Redis pour les deux types.

### Configuration des méthodes d'auth

**Users plateforme** — contrôlé par variables d'environnement :
```
AUTH_PASSWORD_ENABLED=true
AUTH_SSO_ENABLED=true
AUTH_API_TOKEN_ENABLED=true
AUTH_API_TOKEN_MAX=10
```

**Membres managés** — deux niveaux de config :
1. `workspaces.member_auth_config` — valeurs par défaut pour tout le workspace
2. `workspace_memberships.auth_config` — surcharge par membre (`null` = hérite du workspace)

Résolution de la config effective : `membre ?? workspace` pour chaque méthode.

### Méthodes d'auth disponibles (users et membres)

| Méthode | Unicité | Notes |
|---------|---------|-------|
| `PASSWORD` | 1 par identité | bcrypt |
| `SSO_IDENTITY` | 1 par provider (app level) | sub + issuer dans metadata |
| `API_TOKEN` | N par identité | label + expires_at, plusieurs tokens simultanés |

### Self-linking SSO
Un user ou member peut lier un compte SSO à son compte existant :
- `POST /api/auth/sso/link` (user) ou `POST /api/auth/local/sso/link` (member)
- Redirige vers le provider SSO, callback crée un `SSO_IDENTITY` credential lié au compte

### Auto-provisioning SSO (membres)
Configurable dans `workspaces.member_auth_config.sso_auto_provision` :
- `true` → si SSO login et aucune membership trouvée pour ce sub, créer une membership MEMBER automatiquement
- `false` → refuser le login si le sub n'est pas déjà lié à une membership

### Flux de fusion (merge)

Un utilisateur connecté en global veut récupérer une membership locale :
1. `POST /api/auth/merge` — fournit workspace_slug + local_username + local_password
2. Backend vérifie le mot de passe local
3. UPDATE atomique :
```sql
UPDATE workspace_memberships
SET user_id = $global_user_id, local_username = NULL, local_password_hash = NULL
WHERE id = $membership_id AND user_id IS NULL
```
4. Au prochain refresh du JWT global, le nouveau workspace apparaît dans les claims
5. Les `membership_policies` sont intactes — aucune migration de données

---

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
// Global (soft) - extrait le token si présent (global ou local)
api.UseMiddleware(middleware.AuthExtractor(authSvc))

// Par opération (hard) - rejette si pas authentifié
requireAuth := middleware.RequireAuth(api)          // user plateforme uniquement
requireMembership := middleware.RequireMembership(api) // global OU local
huma.Register(api, huma.Operation{
    Middlewares: huma.Middlewares{requireAuth},
}, handler)
```

Deux identités dans le contexte après AuthExtractor :
- `type=global` → `UserIDFromContext(ctx)` retourne l'user_id
- `type=local`  → `MembershipFromContext(ctx)` retourne membership_id + workspace_id + role
