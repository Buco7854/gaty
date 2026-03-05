# GATY

IoT gate control SaaS. Go backend, React frontend. Multi-tenant (workspaces).

## Préférences
- Communication en **français**
- Pas de co-authored commits
- Pas de Makefile → go-task (Taskfile.yml)
- **Commits et pushes uniquement si explicitement demandé**

## Stack
- Backend: Go + Huma v2 + chi router (`github.com/Buco7854/gaty`, entry: `cmd/server/main.go`)
- DB: PostgreSQL + Valkey/Redis + golang-migrate (SQL dans `migrations/`)
- IoT: Eclipse Mosquitto (MQTT)
- Proxy: Caddy (On-Demand TLS)
- Frontend: React + Vite + TS + Mantine + Tailwind CSS v4

## Dev
```
task dev-infra   # docker compose up -d
task dev-api     # air (hot-reload Go)
task dev-frontend # vite dev
task migrate-up/down
```
Credentials: `postgres://gaty:gaty@localhost:5432/gaty` · `redis://localhost:6379` · `tcp://localhost:1883`

Avancement: voir **plan.md**

---

## Packages internes (`internal/`)

| Package | Rôle |
|---------|------|
| `model` | Types Go purs, zéro dépendance DB |
| `repository` | Interfaces + types de params + erreurs sentinelles — **zéro pgx** |
| `repository/postgres` | Implémentations PostgreSQL — constructeurs retournent les interfaces |
| `handler` | Handlers Huma, un fichier par domaine |
| `service` | Logique métier (auth, sso, member, gate_ttl) |
| `middleware` | AuthExtractor (global soft), RequireAuth, RequireMembership, WorkspaceMember/Admin, GateManager, TenantResolver |
| `mqtt` | Client MQTT, SubscribeGateStatuses |
| `config` | Env vars |
| `db`, `cache` | Pool pgx, client Redis |

**Règle de câblage** : seul `main.go` et `middleware/tenant.go` importent `repository/postgres`. Tous les autres reçoivent les interfaces.

Erreurs sentinelles : `repository.ErrNotFound`, `ErrAlreadyExists`, `ErrUnauthorized`

---

## Schéma DB (état actuel)

```
users(id UUID PK, email UNIQUE, created_at)

credentials(id, user_id→users CASCADE, type∈{PASSWORD,SSO_IDENTITY,API_TOKEN},
            hashed_value, label, expires_at, metadata JSONB, created_at)
  -- UNIQUE(user_id) WHERE type='PASSWORD'

workspaces(id, name, owner_id→users, sso_settings JSONB, member_auth_config JSONB, created_at)

workspace_memberships(id UUID PK, workspace_id→workspaces, user_id→users SET NULL,
    local_username TEXT, display_name, role∈{OWNER,ADMIN,MEMBER},
    auth_config JSONB, invited_by→users, created_at
    -- CHECK(user_id IS NOT NULL OR local_username IS NOT NULL)
    -- UNIQUE(workspace_id,user_id) WHERE user_id IS NOT NULL
    -- UNIQUE(workspace_id,local_username) WHERE local_username IS NOT NULL)

membership_credentials(id, membership_id→workspace_memberships CASCADE,
    type∈{PASSWORD,SSO_IDENTITY,API_TOKEN}, hashed_value, label, expires_at, metadata JSONB, created_at)

gates(id, workspace_id→workspaces, name, integration_type, integration_config JSONB,
      open_config JSONB, close_config JSONB, status_config JSONB,
      gate_token TEXT, status, last_seen_at, status_metadata JSONB,
      meta_config JSONB, status_rules JSONB, created_at)

gate_access_codes(id, gate_id→gates CASCADE, hashed_pin, label, metadata JSONB,
                  schedule_id→access_schedules SET NULL, created_at)

access_schedules(id, workspace_id→workspaces, name, description, rules JSONB, created_at)

permissions(code PK, description)

membership_policies(membership_id→workspace_memberships, gate_id→gates, permission_code→permissions
    PRIMARY KEY(membership_id, gate_id, permission_code))

membership_gate_schedules(membership_id, gate_id, schedule_id→access_schedules
    PRIMARY KEY(membership_id, gate_id))

custom_domains(id, gate_id, workspace_id, domain UNIQUE, dns_challenge_token,
               verified_at, created_at)
```

Migrations: `000001`→extensions + `uuid_generate_v7()` (PK par défaut partout), `000002`→core tables,
`000003`→gates, `000004`→credentials, `000005`→permissions, `000006`→custom_domains,
`000007`→gate action configs, `000008`→gate tokens, `000009`→status rules, `000010`→gate TTL index

---

## Auth : deux chemins JWT

**Login global** `POST /api/auth/login` → email+password → user plateforme
```json
{"sub":"<user_id>","type":"global","memberships":[{"workspace_id":"...","membership_id":"...","role":"MEMBER"}]}
```

**Login local** `POST /api/auth/login/local` → workspace_slug+local_username+password
```json
{"sub":"<membership_id>","type":"local","workspace_id":"...","role":"MEMBER"}
```

Refresh tokens en Redis. Membres managés = memberships sans `user_id`.

**Fusion** `POST /api/auth/merge` : lie un membership local à un user_id via UPDATE atomique (permissions intactes).

**Config auth par niveau** :
1. Env vars pour users plateforme (`AUTH_PASSWORD_ENABLED`, `AUTH_SSO_ENABLED`, etc.)
2. `workspaces.member_auth_config` → défauts pour le workspace
3. `workspace_memberships.auth_config` → surcharge par membre (`null` = hérite)

---

## Middleware pattern (Huma)

```go
// main.go — ordre important
api.UseMiddleware(middleware.AuthExtractor(authSvc))  // global soft: injecte identité si token présent
requireAuth       := middleware.RequireAuth(api)          // user global uniquement
requireMembership := middleware.RequireMembership(api)    // global OU local
wsMember          := middleware.WorkspaceMember(api, wsRepo, membershipRepo)
wsAdmin           := middleware.WorkspaceAdmin(api, wsRepo, membershipRepo)
wsGateManager     := middleware.GateManager(api, policyRepo)
```

Lecture du contexte dans un handler :
```go
userID := middleware.UserIDFromContext(ctx)         // type=global
m      := middleware.MembershipFromContext(ctx)     // type=local → .ID, .WorkspaceID, .Role
```

Chi middleware (appliqué avant Huma) : `RequestID`, `RealIP`, `Logger`, `JSONRecoverer`, `CORS`, `TenantResolver`

`TenantResolver` : résout le `Host` header contre `custom_domains` → injecte `tenant_type`+`tenant_id` dans ctx.

---

## Gate : fonctionnement

**ActionConfig** : chaque gate a `open_config`, `close_config`, `status_config` — chacun avec `{type: MQTT|HTTP|NONE, config: {...}}`

**StatusRules** : évaluées sur les métadonnées reçues → override du statut. Ordre : premier match gagne.

**Gate TTL worker** : goroutine dans `main.go`, `service.NewGateTTLWorker(gateRepo, redisClient, DefaultGateTTL)`. Marque les gates "unresponsive" si `last_seen_at > TTL`. Publie sur Redis pour SSE.

**GateToken** : secret d'authentification de l'appareil (rotation via `POST .../rotate-token`). Retourné uniquement à la création et rotation.

---

## Access Schedules

`AccessSchedule` = ensemble nommé de `ScheduleRule`. Logique OR entre les règles.

Types de règles : `time_range` (jours+heure), `weekdays_range` (jours consécutifs), `date_range` (dates calendaire), `day_of_month_range` (jours du mois 1-31), `month_range` (mois 1-12). Tous supportent le wrap-around.

Attachement : `gate_access_codes.schedule_id` (PIN) ou `membership_gate_schedules` (membre+gate).

---

## SSE (temps réel)

Route chi brute (pas Huma) : `GET /api/workspaces/{ws_id}/gates/events`
MQTT → UpdateStatus DB → Redis Pub/Sub `gate:status:{ws_id}` → fan-out SSE.

---

## Huma v2 — Rappels rapides

```go
// Middleware router-agnostic
api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) { ... })

// Stopper dans middleware
huma.WriteErr(api, ctx, http.StatusUnauthorized, "msg"); return

// Context values
ctx = huma.WithValue(ctx, "key", val)   // écrire
ctx.Context().Value("key")              // lire dans middleware
ctx.Value("key")                        // lire dans handler

// Header dans middleware
ctx.Header("Authorization")

// Opération avec middlewares
huma.Register(api, huma.Operation{
    OperationID: "id", Method: http.MethodGet, Path: "/path/{id}",
    Tags: []string{"Tag"}, DefaultStatus: 201,
    Middlewares: huma.Middlewares{myMw},
}, handler)

// Erreurs
huma.Error400BadRequest | 401Unauthorized | 403Forbidden | 404NotFound | 409Conflict | 500InternalServerError
```

Input tags : `path:"x"` `query:"x"` `header:"x"` `required:"true"` `minLength:"n"` `format:"email"`
Output : Body présent → 200, nil → 204.

### Champs requis / optionnels — règles Huma

| Situation | Comportement |
|-----------|-------------|
| Champ body sans tag spécial | **Requis** par défaut |
| Query / header / cookie param | **Optionnel** par défaut |
| `json:"x,omitempty"` | Optionnel |
| `json:"x" omitzero:"true"` | Optionnel (zéro-value) |
| `required:"false"` | Optionnel explicite |
| `required:"true"` | Requis explicite (utile pour query/header) |
| `*string`, `*int`, etc. | **Aucun effet** — pointeur ≠ optionnel |

> **Règle d'or** : tout champ optionnel dans un body (input **ou** output) doit avoir `omitempty` (ou `required:"false"`). Un pointeur seul ne suffit pas.

### Nullable

| Situation | Résultat schéma |
|-----------|----------------|
| `*string \`json:"x"\`` | Requis + **nullable** (peut valoir `null`) |
| `*string \`json:"x,omitempty"\`` | Optionnel + **non nullable** (absent ou valeur) |
| `string \`json:"x" nullable:"true"\`` | Requis + nullable |
| `nullable:"true"` sur un objet | **Interdit** — utiliser un champ `_ struct{} \`nullable:"true"\`` |

### Defaults

Le tag `default` documente ET applique la valeur par défaut côté serveur (Huma remplit le champ si absent). À utiliser sur les champs optionnels qui ont une valeur par défaut, pour éviter de dupliquer la logique dans le handler.

```go
// Bien : défaut documenté et appliqué par Huma
Role model.WorkspaceRole `json:"role,omitempty" default:"MEMBER"`

// À éviter : défaut silencieux dans le handler uniquement
if role == "" { role = model.RoleMember }
```

> Utiliser `*bool \`json:"enabled" default:"true"\`` pour les booléens où `false` est une valeur significative (sinon `false` == zéro-value == "non fourni").

### readOnly / writeOnly

`readOnly:"true"` → champ présent uniquement en réponse (jamais envoyé par le client).
`writeOnly:"true"` → champ présent uniquement en requête (jamais retourné).
Non enforced par Huma, documentation uniquement — utile si on réutilise un struct pour input et output.
