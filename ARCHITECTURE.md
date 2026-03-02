# GATY - IoT Gate Control SaaS

## Vue d'ensemble

GATY est un SaaS Multi-Tenant B2B/B2C permettant le contrôle centralisé de portails physiques (portails, barrières, portes) via une interface web. Le système est conçu pour fonctionner à travers les NAT sans nécessiter d'ouverture de ports côté client, grâce à une communication MQTT bidirectionnelle.

## Stack Technique

| Composant | Technologie |
|-----------|-------------|
| Backend API | Go (Huma + chi router) |
| Base de données | PostgreSQL (relationnel + JSONB) |
| Cache / Rate Limiting | Redis |
| Broker IoT | Eclipse Mosquitto (MQTT) |
| Reverse Proxy / TLS | Caddy (On-Demand TLS) |
| Frontend | React + Vite + TypeScript |
| UI Components | shadcn/ui + Tailwind CSS |
| Migrations DB | golang-migrate (fichiers SQL) |
| Hot-reload (dev) | air |

## Architecture Globale

```
┌─────────────┐     HTTPS      ┌───────────┐     HTTP      ┌──────────────┐
│   Browser   │ ──────────────> │   Caddy   │ ────────────> │  Go Backend  │
│  (React SPA)│ <────────────── │  (TLS)    │ <──────────── │  (Huma API)  │
└─────────────┘                 └───────────┘               └──────┬───────┘
                                                                   │
                                              ┌────────────────────┼────────────────┐
                                              │                    │                │
                                         ┌────▼─────┐     ┌───────▼──────┐  ┌──────▼──────┐
                                         │ PostgreSQL│     │    Redis     │  │  Mosquitto  │
                                         │          │     │ (cache/rate) │  │   (MQTT)    │
                                         └──────────┘     └──────────────┘  └──────┬──────┘
                                                                                   │
                                                                            ┌──────▼──────┐
                                                                            │ IoT Devices │
                                                                            │ (Portails)  │
                                                                            └─────────────┘
```

## Concepts Clés

### Multi-Tenancy

Le système est organisé en **Workspaces**. Chaque workspace est un espace isolé contenant ses propres gates, membres et configurations. Un utilisateur peut appartenir à plusieurs workspaces avec des rôles différents.

**Rôles workspace** : `OWNER` > `ADMIN` > `MEMBER`

### RBAC (Role-Based Access Control)

Le contrôle d'accès est ultra-granulaire via deux niveaux :

1. **Niveau Workspace** : le rôle (`OWNER`, `ADMIN`, `MEMBER`) détermine les capacités de gestion.
2. **Niveau Gate** : la table `gate_user_policies` associe des permissions spécifiques (ex: `gate:read_status`, `gate:trigger_open`) à un utilisateur pour une gate donnée.

Un `MEMBER` ne voit sur son dashboard que les gates pour lesquelles il possède au minimum `gate:read_status`.

### Authentification Polymorphique

La table `credentials` centralise toutes les méthodes d'accès via un système polymorphique :

| Type | Usage |
|------|-------|
| `PASSWORD` | Login classique utilisateur |
| `PIN_CODE` | Accès guest sur une gate (sans compte) |
| `API_TOKEN` | Intégrations programmatiques |
| `OIDC_IDENTITY` | SSO entreprise (Keycloak, Entra ID) |

Chaque credential porte un champ `metadata` (JSONB) permettant des règles métier : expiration, jours autorisés, plages horaires.

### Communication IoT (MQTT)

Le backend ne se connecte jamais directement aux réseaux privés des clients. Les appareils IoT maintiennent une connexion sortante persistante vers le broker MQTT cloud.

**Topics MQTT** :
- Commandes (backend -> device) : `workspace_{id}/gates/{gate_id}/command`
- Statuts (device -> backend) : `workspace_{id}/gates/{gate_id}/status`

Le champ `last_seen_at` sur chaque gate permet de détecter les appareils déconnectés.

### Guest Access (Code PIN)

Permet d'ouvrir un portail sans compte utilisateur, via un code PIN.

**Flux** :
1. L'utilisateur accède à l'URL dédiée au portail (custom domain ou short-link).
2. Un pavé numérique s'affiche (pas de navigation, branding du workspace uniquement).
3. Le PIN est envoyé à `POST /api/public/unlock`.
4. Le backend vérifie : rate limit (IP + gate), hash bcrypt, règles métier (horaires, expiration).
5. Si valide : commande MQTT envoyée, événement logué dans `audit_logs`.

**Sécurité** :
- Rate limiting double : par IP (5 essais / 15 min) ET par gate_id (protection contre brute-force distribué).
- Réponse à temps constant (obfuscation temporelle).

### Domaines Personnalisés (White-Labeling)

Le système supporte l'attachement de domaines personnalisés à deux niveaux :

| Cible | Comportement |
|-------|-------------|
| `WORKSPACE` | Affiche le dashboard classique (login, liste des gates) |
| `GATE` | Affiche uniquement l'interface guest PIN pour ce portail |

**Flux TLS** :
1. Caddy reçoit une requête sur un domaine inconnu.
2. Caddy interroge `GET /api/internal/verify-domain` pour valider le domaine.
3. Si validé, un certificat Let's Encrypt est généré automatiquement (On-Demand TLS).
4. Le middleware Go résout le tenant via le header `Host` et injecte le contexte.

### OIDC (Single Sign-On)

Les entreprises peuvent connecter leur propre fournisseur d'identité. Le système supporte l'auto-provisioning Just-In-Time :

1. L'utilisateur se connecte via SSO sur l'URL du workspace.
2. Le backend valide le JWT OIDC et lit les claims (groups, roles).
3. Si l'utilisateur n'existe pas, il est créé automatiquement.
4. Les `mapping_rules` du workspace (stockées dans `oidc_settings` JSONB) attribuent automatiquement les permissions sur les gates.

Exemple de mapping : `{"group:jardiniers": "role:MEMBER", "permissions": {"gate:1": ["gate:trigger_open"], "schedule": "08:00-18:00"}}`

### Temps Réel (SSE via Huma)

Le dashboard affiche les statuts des gates en temps réel via **Server-Sent Events (SSE)**. Le flux est unidirectionnel (serveur → client), ce qui est suffisant car toutes les actions client passent par des requêtes HTTP classiques.

**Architecture du flux SSE** :
1. Le client MQTT Go reçoit un message sur `workspace_{id}/gates/{gate_id}/status`.
2. Le backend met à jour la base de données (`status`, `last_seen_at`).
3. Le backend relaie l'événement vers toutes les connexions SSE actives pour ce workspace (fan-out).
4. Le frontend React reçoit l'événement via `EventSource` et met à jour l'UI.

**Endpoint** : `GET /api/workspaces/:ws_id/gates/events` (SSE stream, JWT auth)

**Implémentation** : utilise le package `sse` de Huma qui intègre les types d'événements dans la spec OpenAPI :

```go
sse.Register(api, huma.Operation{
    OperationID: "gate-events",
    Method:      http.MethodGet,
    Path:        "/api/workspaces/{ws_id}/gates/events",
}, map[string]any{
    "gateStatusChanged": GateStatusChangedEvent{},
    "gateCommandAck":    GateCommandAckEvent{},
}, handler)
```

**Types d'événements** :

| Event | Payload | Déclencheur |
|-------|---------|-------------|
| `gateStatusChanged` | `{gate_id, status, last_seen_at}` | Message MQTT sur le topic status |
| `gateCommandAck` | `{gate_id, command, success}` | Réponse du device après une commande |

**Côté frontend** : un hook React `useGateEvents(workspaceId)` encapsule `EventSource` avec reconnexion automatique native du protocole SSE.

## Modèle de Données

```
users
  ├── id (UUID, PK)
  ├── email (UNIQUE)
  ├── created_at
  └── deleted_at

workspaces
  ├── id (UUID, PK)
  ├── name
  ├── owner_id (FK -> users)
  ├── oidc_settings (JSONB)
  ├── created_at
  └── deleted_at

workspace_members
  ├── workspace_id (FK, PK)
  ├── user_id (FK, PK)
  └── workspace_role (ENUM: OWNER, ADMIN, MEMBER)

gates
  ├── id (UUID, PK)
  ├── workspace_id (FK -> workspaces)
  ├── name
  ├── integration_type (ENUM: MQTT, POLLING, WEBHOOK)
  ├── integration_config (JSONB)
  ├── status
  ├── last_seen_at
  ├── created_at
  └── deleted_at

credentials
  ├── id (UUID, PK)
  ├── target_type (ENUM: USER, GATE)
  ├── target_id (UUID)
  ├── credential_type (ENUM: PASSWORD, PIN_CODE, API_TOKEN, OIDC_IDENTITY)
  ├── hashed_value
  ├── metadata (JSONB: expires_at, allowed_days, allowed_hours)
  └── created_at

permissions
  └── code (PK, ex: gate:read_status, gate:trigger_open)

gate_user_policies
  ├── gate_id (FK, PK)
  ├── user_id (FK, PK)
  └── permission_code (FK, PK)

custom_domains
  ├── id (UUID, PK)
  ├── domain_name (UNIQUE)
  ├── target_type (ENUM: WORKSPACE, GATE)
  ├── target_id (UUID)
  ├── base_path (default: /)
  ├── is_verified (BOOLEAN)
  └── created_at

audit_logs
  ├── id (UUID, PK)
  ├── workspace_id (FK)
  ├── gate_id (FK, nullable)
  ├── user_id (FK, nullable)
  ├── action
  ├── ip_address
  ├── metadata (JSONB)
  └── created_at
```

## Environnement de Développement

**Approche hybride** : le backend Go tourne en local avec hot-reload (`air`), les services d'infrastructure tournent dans Docker Compose.

```yaml
# Services Docker (dev)
- PostgreSQL (port 5432)
- Redis (port 6379)
- Mosquitto (port 1883)
- Caddy (ports 80/443)
```

Le frontend React tourne via `vite dev` en local (port 5173) avec proxy vers le backend.

## Endpoints API (Principaux)

### Publics
- `POST /api/auth/register` - Inscription
- `POST /api/auth/login` - Connexion
- `POST /api/auth/refresh` - Renouvellement JWT
- `GET /api/auth/oidc/:ws_id/authorize` - Redirection SSO
- `GET /api/auth/oidc/:ws_id/callback` - Callback SSO
- `POST /api/public/unlock` - Guest PIN unlock

### Protégés (JWT requis)
- `GET /api/auth/me` - Profil utilisateur
- `CRUD /api/workspaces` - Gestion des workspaces
- `CRUD /api/workspaces/:ws_id/members` - Gestion des membres
- `CRUD /api/workspaces/:ws_id/gates` - Gestion des gates (filtrage RBAC)
- `POST /api/workspaces/:ws_id/gates/:gate_id/trigger` - Ouverture gate
- `CRUD /api/workspaces/:ws_id/gates/:gate_id/policies` - Gestion des permissions
- `CRUD /api/workspaces/:ws_id/gates/:gate_id/pins` - Gestion des PIN codes
- `CRUD /api/workspaces/:ws_id/domains` - Gestion des domaines
- `PATCH /api/workspaces/:ws_id/oidc-settings` - Configuration SSO

### Temps Réel (SSE)
- `GET /api/workspaces/:ws_id/gates/events` - Stream SSE des statuts gates (JWT requis)

### Internes
- `GET /api/internal/verify-domain` - Validation domaine pour Caddy
- `GET /api/health` - Health check

### OpenAPI
- `GET /api/openapi.json` - Spec OpenAPI 3.1 auto-générée par Huma
