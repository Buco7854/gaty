# GATY - IoT Gate Control SaaS - Plan de Développement

## Stack validée
- **Backend** : Go (Huma + chi router)
- **Base de données** : PostgreSQL + Redis (cache/rate-limiting)
- **Migrations** : golang-migrate (fichiers SQL versionnés)
- **Broker IoT** : Eclipse Mosquitto (MQTT)
- **Reverse Proxy** : Caddy (On-Demand TLS)
- **Frontend** : React + Vite + TypeScript + shadcn/ui + Tailwind CSS
- **Tests** : Unitaires + Intégration API + E2E (flux critiques)
- **Dev local** : Go avec hot-reload (air) + Docker Compose pour l'infra (PG, Redis, Mosquitto, Caddy)

---

## Architecture Core : Users vs Members

Deux entités distinctes coexistent dans GATY.

### Users (Comptes Plateforme)
- Personne qui **crée un compte** sur l'application web
- Table : `users` (id, email, created_at)
- Credential : `credentials` (target_type=USER, type=PASSWORD ou OIDC_IDENTITY)
- Peut créer des workspaces et en être OWNER/ADMIN/MEMBER
- Table de jointure : `workspace_members` → rôles OWNER, ADMIN, MEMBER
- Authentification : JWT émis par l'API (accès à l'interface web)

### Members (Personnes gérées par admin)
- Personne **ajoutée manuellement** par un admin dans un workspace
- **N'a PAS de compte plateforme** par défaut
- Table : `members` (id, workspace_id, display_name, email, username, user_id nullable, created_at)
  - `email` = simple info de contact pour l'admin (non utilisé pour auth)
  - `username` = identifiant de connexion dans le contexte du workspace
  - `user_id` = nullable, renseigné si le member convertit son compte en User
- Credential : `credentials` (target_type=MEMBER, type=PASSWORD ou API_TOKEN)
- Peut être autorisé sur des gates spécifiques via `gate_member_policies`
- **Conversion possible** : un member peut créer un compte User → le lien `user_id` est établi

### Vecteurs d'accès aux Gates
| Vecteur | Mécanisme |
|---------|-----------|
| User OWNER/ADMIN du workspace | Rôle workspace (accès total) |
| User avec policy | `gate_user_policies` (gate:trigger_open etc.) |
| Member avec policy | `gate_member_policies` (gate:trigger_open etc.) |
| Code PIN public | `credentials` (target_type=GATE, type=PIN_CODE) |
| Token API | `credentials` (target_type=USER\|MEMBER, type=API_TOKEN) |

---

## Phase 0 : Setup du Projet & Outillage

- [x] Initialiser le module Go (`go mod init`)
- [x] Mettre en place la structure de dossiers backend (cmd/, internal/, migrations/, configs/)
- [x] Initialiser le projet React avec Vite + TypeScript
- [x] Installer et configurer Tailwind CSS + shadcn/ui
- [x] Créer le `docker-compose.yml` (PostgreSQL, Redis, Mosquitto, Caddy)
- [x] Configurer `air` pour le hot-reload Go
- [x] Créer le `Caddyfile` de base (dev local)
- [x] Mettre en place le `.env` et la gestion des variables d'environnement (Viper ou envconfig)
- [x] Configurer le linter/formatter (golangci-lint, ESLint, Prettier)
- [x] Créer le `Makefile` avec les commandes courantes (migrate, dev, test, build) _(remplacé par Taskfile.yml — go-task)_

---

## Phase 1 : Base de Données & Migrations

### 1.1 - Setup golang-migrate
- [x] Installer golang-migrate
- [x] Configurer le répertoire `migrations/` et les commandes dans le Makefile _(dans Taskfile.yml)_
- [x] Créer la migration initiale : extensions (`uuid-ossp` ou `pgcrypto`)

### 1.2 - Tables Core & Multi-Tenant
- [x] Migration : table `users` (id UUID, email, created_at, deleted_at)
- [x] Migration : table `workspaces` (id, name, owner_id FK, oidc_settings JSONB, created_at, deleted_at)
- [x] Migration : table `workspace_members` (workspace_id, user_id, workspace_role ENUM, PK composite) — jointure User ↔ Workspace
- [x] Migration : table `gates` (id, workspace_id FK, name, integration_type ENUM, integration_config JSONB, status, last_seen_at, created_at, deleted_at)

### 1.3 - Tables Auth & Permissions
- [x] Migration : table `credentials` (id, target_type ENUM, target_id, credential_type ENUM, hashed_value, metadata JSONB, created_at)
- [x] Index unique composite sur `credentials` pour éviter les doublons
- [x] Migration : table `permissions` (code PK, description)
- [x] Migration : seed des permissions de base (gate:read_status, gate:trigger_open, gate:manage, workspace:manage)
- [x] Migration : table `gate_user_policies` (gate_id, user_id, permission_code, PK composite)

### 1.5 - Tables Members (non-user)
- [x] Migration : table `members` (id UUID, workspace_id FK, display_name, email nullable, username, user_id nullable FK → users, created_at, deleted_at)
- [x] Contrainte UNIQUE (workspace_id, username) sur `members`
- [x] Migration : étendre l'ENUM `credential_target_type` pour inclure `MEMBER` _(000008)_
- [x] Migration : table `gate_member_policies` (gate_id FK, member_id FK, permission_code FK, PK composite) _(000009)_
- [x] Index sur `gate_member_policies(member_id)` pour lookup rapide

### 1.4 - Tables Domaines Personnalisés & Audit
- [x] Migration : table `custom_domains` (id, domain_name UNIQUE, target_type ENUM, target_id, base_path, is_verified, created_at)
- [x] Migration : table `audit_logs` (id, workspace_id, gate_id nullable, user_id nullable, action, ip_address, metadata JSONB, created_at)
- [x] Index sur `audit_logs` (workspace_id, created_at) pour les requêtes chronologiques

---

## Phase 2 : Backend Core (Connexion DB, Config, Serveur HTTP)

- [x] Module de configuration (chargement .env, validation des variables requises)
- [x] Connexion PostgreSQL (pool de connexions via pgx ou database/sql)
- [x] Connexion Redis
- [x] Setup Huma API avec chi comme routeur sous-jacent (CORS, Recovery, Logger via chi middleware)
- [x] Configurer la génération automatique OpenAPI 3.1 via Huma
- [x] Middleware d'extraction du Tenant via le header `Host` (Tenant Resolution)
- [x] Structure de réponse API standardisée (erreurs, pagination) _(géré nativement par Huma)_
- [x] Health check endpoint (`GET /api/health`)

---

## Phase 3 : Authentification & Gestion des Utilisateurs

### 3.1 - Auth par mot de passe (Password)
- [x] Endpoint `POST /api/auth/register` (création user + credential type PASSWORD)
- [x] Endpoint `POST /api/auth/login` (validation bcrypt, émission JWT)
- [x] Middleware d'authentification JWT (AuthExtractor global + RequireAuth par opération, Huma natif)
- [x] Endpoint `POST /api/auth/refresh` (renouvellement de token, Redis-backed rotation)
- [x] Endpoint `GET /api/auth/me` (profil utilisateur connecté)

### 3.2 - Gestion des Workspaces (Users)
> Ces endpoints gèrent les **Users plateforme** dans un workspace (table `workspace_members`).
- [x] Endpoint `POST /api/workspaces` (création workspace, l'utilisateur devient OWNER)
- [x] Endpoint `GET /api/workspaces` (liste des workspaces de l'utilisateur connecté)
- [x] Endpoint `GET /api/workspaces/:ws_id` (détails d'un workspace)
- [x] Endpoint `POST /api/workspaces/:ws_id/users` (invitation d'un user existant dans le workspace) _(actuellement /members)_
- [x] Endpoint `PATCH /api/workspaces/:ws_id/users/:user_id` (changement de rôle)
- [x] Endpoint `DELETE /api/workspaces/:ws_id/users/:user_id` (retrait d'un user)

### 3.3 - Gestion des Members (non-user)
> Ces endpoints gèrent les **personnes ajoutées par l'admin** qui n'ont pas de compte plateforme (table `members`).
- [x] Endpoint `POST /api/auth/member/login` (login par username **ou** email + mot de passe, retourne JWT member)
- [x] Endpoint `POST /api/workspaces/:ws_id/members` (création d'un member par l'admin)
- [x] Endpoint `GET /api/workspaces/:ws_id/members` (liste des members du workspace)
- [x] Endpoint `GET /api/workspaces/:ws_id/members/:member_id` (détails d'un member)
- [x] Endpoint `PATCH /api/workspaces/:ws_id/members/:member_id` (mise à jour infos)
- [x] Endpoint `DELETE /api/workspaces/:ws_id/members/:member_id` (soft delete)
- [ ] Endpoint `POST /api/workspaces/:ws_id/members/:member_id/convert` (conversion member → user)
  - Crée un compte `users` avec l'email du member
  - Migre le credential PASSWORD vers le nouveau user
  - Lie `members.user_id` → nouveau user
  - Envoie un email d'invitation (si email renseigné)

---

## Phase 4 : RBAC & Permissions Granulaires

### 4.1 - RBAC Users (implémenté)
- [x] Service RBAC : vérification du rôle workspace (OWNER, ADMIN, MEMBER)
- [x] Middleware d'autorisation workspace (injecte le rôle dans le context) _(WorkspaceMember + WorkspaceAdmin, Huma per-op)_
- [x] Service de vérification des permissions gate (lecture de `gate_user_policies`)
- [x] Endpoint `GET /api/workspaces/:ws_id/gates` avec filtrage contextuel (ADMIN voit tout, MEMBER voit uniquement ses gates autorisées via JOIN)
- [x] Endpoint `POST /api/workspaces/:ws_id/gates/:gate_id/policies` (attribution de permissions à un user sur une gate)
- [x] Endpoint `DELETE /api/workspaces/:ws_id/gates/:gate_id/policies/:user_id` (retrait de permissions)
- [x] Endpoint `GET /api/workspaces/:ws_id/gates/:gate_id/policies` (liste des policies d'une gate)

### 4.2 - RBAC Members (à faire)
- [ ] Endpoint `POST /api/workspaces/:ws_id/gates/:gate_id/member-policies` (attribution permissions à un member sur une gate)
- [ ] Endpoint `DELETE /api/workspaces/:ws_id/gates/:gate_id/member-policies/:member_id` (retrait)
- [ ] Endpoint `GET /api/workspaces/:ws_id/gates/:gate_id/member-policies` (liste)
- [ ] Mise à jour du service RBAC : vérifier `gate_member_policies` lors du `trigger` par un member

---

## Phase 5 : Gestion des Gates & Communication IoT (MQTT)

### 5.1 - CRUD Gates
- [x] Endpoint `POST /api/workspaces/:ws_id/gates` (création d'une gate)
- [x] Endpoint `GET /api/workspaces/:ws_id/gates/:gate_id` (détails d'une gate)
- [x] Endpoint `PATCH /api/workspaces/:ws_id/gates/:gate_id` (mise à jour)
- [x] Endpoint `DELETE /api/workspaces/:ws_id/gates/:gate_id` (soft delete)

### 5.2 - Intégration MQTT
- [x] Client MQTT Go (connexion au broker Mosquitto avec reconnexion automatique)
- [x] Abonnement aux topics de statut : `workspace_{id}/gates/{gate_id}/status`
- [x] Handler de réception des statuts (mise à jour `status` + `last_seen_at` en base)
- [x] Fonction de publication de commande : `workspace_{id}/gates/{gate_id}/command`
- [x] Endpoint `POST /api/workspaces/:ws_id/gates/:gate_id/trigger` (déclenchement d'ouverture, vérification RBAC, publication MQTT, log audit)
- [x] Détection des appareils offline (vérification à la lecture via `EffectiveStatus()` basée sur `last_seen_at`)

### 5.3 - Configuration Mosquitto
- [x] Configuration Mosquitto pour le dev (listener 1883, anonymous access)
- [x] Stratégie d'authentification MQTT : backend s'authentifie via `MQTT_USERNAME`/`MQTT_PASSWORD` env (optionnel, anonyme en dev) ; chaque gate device utilise son `gate_id` comme username + `API_TOKEN` de la table `credentials`
- [x] ACL MQTT : fichier `configs/mosquitto.acl` — `gaty-server` accès total, gate devices limités à leurs propres topics via pattern `%u`

---

## Phase 6 : Guest Access (Code PIN Public)

- [ ] Endpoint `POST /api/public/unlock` (gate_id déduit du middleware Tenant ou du body)
- [ ] Rate Limiting Redis (5 essais / 15 min / IP + rate limit global par gate_id)
- [ ] Réponse à temps constant (obfuscation temporelle avec `time.Sleep` ou `subtle.ConstantTimeCompare`)
- [ ] Recherche du credential (target_type=GATE, credential_type=PIN_CODE)
- [ ] Validation bcrypt du PIN
- [ ] Vérification des règles métier dans `metadata` JSONB (expires_at, allowed_days, allowed_hours)
- [ ] Publication MQTT de la commande d'ouverture en cas de succès
- [ ] Écriture dans `audit_logs` (succès et échecs)
- [ ] CRUD des PIN codes pour les admins (`POST/DELETE /api/workspaces/:ws_id/gates/:gate_id/pins`)

---

## Phase 7 : OIDC (Single Sign-On)

- [ ] Endpoint `GET /api/auth/oidc/:ws_id/authorize` (redirection vers le provider OIDC)
- [ ] Endpoint `GET /api/auth/oidc/:ws_id/callback` (réception du code, échange token)
- [ ] Validation du JWT OIDC (signature, issuer, audience)
- [ ] Auto-Provisioning Just-In-Time : création user si inexistant
- [ ] Lecture des claims (groups, roles) et application des `mapping_rules` du workspace
- [ ] Attribution automatique des `gate_user_policies` selon les règles mappées
- [ ] Stockage du credential OIDC_IDENTITY dans la table `credentials`
- [ ] Configuration UI pour les admins : endpoint `PATCH /api/workspaces/:ws_id/oidc-settings`

---

## Phase 8 : Domaines Personnalisés & Reverse Proxy (Caddy)

### 8.1 - Backend
- [ ] Endpoint interne `GET /api/internal/verify-domain` (utilisé par Caddy pour l'On-Demand TLS)
- [ ] CRUD custom domains : `POST/GET/DELETE /api/workspaces/:ws_id/domains`
- [ ] Vérification de propriété du domaine (DNS TXT record check)
- [ ] Middleware Tenant Resolution complet (résolution Host -> workspace ou gate)

### 8.2 - Caddy Configuration
- [ ] Caddyfile avec directive `on_demand_tls` pointant vers le endpoint de vérification
- [ ] Règles de reverse proxy vers le backend Go et le frontend
- [ ] Configuration pour le dev local (certificats auto-signés ou HTTP)

---

## Phase 9 : Frontend React

### 9.1 - Setup & Architecture
- [ ] Structure de dossiers (features/, components/, hooks/, lib/, api/, types/)
- [ ] Configuration du routeur (React Router ou TanStack Router)
- [ ] Client API (Axios ou fetch wrapper avec intercepteurs JWT)
- [ ] Store d'authentification (Context API ou Zustand)
- [ ] Layout principal (Sidebar, Header, Content area)
- [ ] Thème dynamique : injection des couleurs du tenant via CSS variables

### 9.2 - Pages Auth
- [ ] Page Login (email/password)
- [ ] Page Register
- [ ] Bouton "Se connecter avec SSO" (flux OIDC)
- [ ] Gestion du refresh token

### 9.3 - Dashboard Workspace
- [ ] Page liste des workspaces
- [ ] Page dashboard d'un workspace (liste des gates avec statut temps réel)
- [ ] Indicateur de statut gate (online/offline/unknown basé sur last_seen_at)
- [ ] Page de gestion des membres du workspace
- [ ] Page de gestion des domaines personnalisés

### 9.4 - Gestion des Gates
- [ ] Page détail d'une gate (statut, config, logs récents)
- [ ] Bouton "Ouvrir" avec confirmation (appel trigger)
- [ ] Page de gestion des permissions d'une gate (attribution users/droits)
- [ ] Page de gestion des PIN codes d'une gate

### 9.5 - Vue Guest (Domaine ciblant une Gate)
- [ ] Page Guest : pavé numérique (PIN pad) plein écran
- [ ] Feedback visuel (succès/erreur/loading)
- [ ] Design adaptatif mobile-first (cas d'usage principal : téléphone devant le portail)
- [ ] Aucune navigation visible, branding du workspace uniquement

### 9.6 - Temps Réel (SSE)
- [ ] Endpoint SSE backend via `sse.Register` de Huma (`GET /api/workspaces/:ws_id/gates/events`)
- [ ] Définir les types d'événements SSE (GateStatusChanged, GateCommandAck) comme structs Go mappés dans sse.Register
- [ ] Bridge MQTT → SSE : le backend relaie les messages MQTT reçus vers les connexions SSE actives (fan-out par workspace)
- [ ] Client SSE frontend : hook React `useGateEvents()` basé sur `EventSource` avec reconnexion automatique
- [ ] Mise à jour automatique de l'UI lors d'un changement de statut

---

## Phase 10 : Tests & Qualité

### 10.1 - Tests Unitaires (Go)
- [ ] Tests du service RBAC (vérification des rôles et permissions)
- [ ] Tests de la validation PIN (bcrypt, règles métier metadata, temps constant)
- [ ] Tests du Tenant Resolution middleware
- [ ] Tests du rate limiter Redis (mock)
- [ ] Tests de l'auto-provisioning OIDC (mapping rules)

### 10.2 - Tests d'Intégration API (Go)
- [ ] Setup d'une DB de test (testcontainers-go ou DB dédiée)
- [ ] Tests du flux auth complet (register -> login -> access protected route)
- [ ] Tests du flux CRUD workspace + members
- [ ] Tests du flux CRUD gates + policies
- [ ] Tests du flux Guest PIN unlock (succès, échec, rate limit, expiration)
- [ ] Tests du filtrage contextuel GET /gates (ADMIN vs MEMBER)

### 10.3 - Tests E2E (Frontend)
- [ ] Setup Playwright
- [ ] Test E2E : flux login -> dashboard -> ouverture gate
- [ ] Test E2E : flux Guest PIN (page dédiée -> saisie PIN -> feedback)

---

## Phase 11 : Dockerisation & Déploiement

- [ ] Dockerfile backend Go (multi-stage build)
- [ ] Dockerfile frontend React (build + serve via Caddy ou nginx)
- [ ] Docker Compose complet de production (tous les services)
- [ ] Scripts d'initialisation (seed des permissions, migration auto au démarrage)
- [ ] Documentation du setup local dans le README
- [ ] Variables d'environnement documentées (.env.example)
