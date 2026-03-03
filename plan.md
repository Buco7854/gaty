# GATY - IoT Gate Control SaaS - Plan de Développement

## Stack validée
- **Backend** : Go (Huma + chi router)
- **Base de données** : PostgreSQL + Redis (cache/rate-limiting)
- **Migrations** : golang-migrate (fichiers SQL versionnés)
- **Broker IoT** : Eclipse Mosquitto (MQTT)
- **Reverse Proxy** : Caddy (On-Demand TLS)
- **Frontend** : React + Vite + TypeScript + Mantine + Tailwind CSS
- **Tests** : Unitaires + Intégration API + E2E (flux critiques)
- **Dev local** : Go avec hot-reload (air) + Docker Compose pour l'infra (PG, Redis, Mosquitto, Caddy)

---

## Architecture Core : workspace_memberships (cible post-refactor Phase R)

Tout accès workspace passe par la table pivot `workspace_memberships`.
- **User plateforme** : membership avec `user_id` renseigné, s'authentifie par email/password global
- **Membre managé** : membership avec `user_id` null, s'authentifie par `workspace_slug + local_username + password`
- **Fusion** (merge) : un membre managé lie son `user_id` → UPDATE atomique, permissions intactes
- **Permissions** : `membership_policies` lié au `membership_id` (pas à l'identité) → aucun problème à la fusion

Voir CLAUDE.md pour le schéma complet.

---

## Phase 0 : Setup du Projet & Outillage

- [x] Initialiser le module Go (`go mod init`)
- [x] Mettre en place la structure de dossiers backend (cmd/, internal/, migrations/, configs/)
- [x] Initialiser le projet React avec Vite + TypeScript
- [ ] Installer et configurer Mantine + Tailwind CSS
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
- [x] Migration : table `workspaces` (id, name, owner_id FK, sso_settings JSONB, created_at, deleted_at)
- [x] Migration : table `workspace_members` (workspace_id, user_id, workspace_role ENUM, PK composite) — jointure User ↔ Workspace
- [x] Migration : table `gates` (id, workspace_id FK, name, integration_type ENUM, integration_config JSONB, status, last_seen_at, created_at, deleted_at)

### 1.3 - Tables Auth & Permissions
- [x] Migration : table `credentials` (id, target_type ENUM, target_id, credential_type ENUM, hashed_value, metadata JSONB, created_at)
- [x] Index unique composite sur `credentials` pour éviter les doublons
- [x] Migration : table `permissions` (code PK, description)
- [x] Migration : seed des permissions de base (gate:read_status, gate:trigger_open, gate:manage, workspace:manage)
- [x] Migration : table `gate_user_policies` (gate_id, user_id, permission_code, PK composite)

### 1.5 - Tables Members (non-user)
- [x] Migration : table `members` (id UUID, workspace_id FK, display_name, email nullable, username, **workspace_role DEFAULT 'MEMBER'**, user_id nullable FK → users, created_at, deleted_at)
- [x] Contrainte UNIQUE (workspace_id, username) sur `members`
- [x] Migration : ENUM `credential_target_type` inclut MEMBER depuis la création _(dans 000004)_
- [x] Migration : table `gate_policies` unifiée (gate_id FK, target_type TEXT CHECK IN('USER','MEMBER'), target_id UUID, permission_code FK, PK composite) _(dans 000005)_
- [x] Index sur `gate_policies(target_type, target_id)` pour lookup rapide

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
- [x] Endpoint `POST /api/workspaces/:ws_id/members/:member_id/convert` (conversion member → user)
  - Crée un compte `users` avec l'email du member
  - Migre le credential PASSWORD vers le nouveau user
  - Ajoute le user dans `workspace_members` avec le rôle du member
  - Lie `members.user_id` → nouveau user

---

## Phase 4 : RBAC & Permissions Granulaires

### 4.1 - RBAC Unifié (Users + Members)
- [x] Middleware `WorkspaceMember`/`WorkspaceAdmin` : supporte Users (via `workspace_members`) **et** Members (via `members.workspace_role`)
- [x] Table `gate_policies` unifiée : `target_type` (USER|MEMBER) + `target_id` → remplace `gate_user_policies` + `gate_member_policies`
- [x] Service RBAC : vérification du rôle workspace (OWNER, ADMIN, MEMBER) pour users et members
- [x] `GET /api/workspaces/:ws_id/gates` : filtrage contextuel (ADMIN voit tout, MEMBER voit gates avec policy via JOIN sur `gate_policies`)
- [x] `POST /api/workspaces/:ws_id/gates/:gate_id/policies` (attribution permissions — `target_type` + `target_id` dans le body)
- [x] `DELETE /api/workspaces/:ws_id/gates/:gate_id/policies/{target_type}/{target_id}` (retrait)
- [x] `GET /api/workspaces/:ws_id/gates/:gate_id/policies` (liste des policies d'une gate)

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

---

## Phase R : Refactor Architecture User/Member → workspace_memberships

> **Priorité maximale** — Doit être complété avant de poursuivre les phases 6+.
> L'ancienne architecture (tables `members`, `workspace_members`, `gate_policies` polymorphiques) est remplacée.

### R1 — Migrations DB (réécriture complète, base vierge)
- [x] Supprimer toutes les migrations existantes (000001 à 000005)
- [x] `000001_extensions` : pgcrypto
- [x] `000002_core_tables` : `users`, `workspaces`, `workspace_memberships`
  - `workspaces` : ajouter `slug UNIQUE NOT NULL`, `member_auth_config JSONB DEFAULT '{}'`
  - `workspace_memberships` : id, workspace_id FK, user_id FK nullable (SET NULL), local_username, display_name, role CHECK('OWNER','ADMIN','MEMBER'), auth_config JSONB DEFAULT '{}', invited_by FK, created_at
  - CHECK (user_id IS NOT NULL OR local_username IS NOT NULL)
  - UNIQUE (workspace_id, user_id) WHERE user_id IS NOT NULL
  - UNIQUE (workspace_id, local_username) WHERE local_username IS NOT NULL
- [x] `000003_gates` : table `gates` (inchangée)
- [x] `000004_credentials` :
  - table `credentials` (id PK, user_id FK → users CASCADE, type CHECK('PASSWORD','SSO_IDENTITY','API_TOKEN'), hashed_value, label, expires_at, metadata JSONB) — UNIQUE (user_id) WHERE type='PASSWORD', pas de contrainte unique sur API_TOKEN
  - table `membership_credentials` (id PK, membership_id FK → workspace_memberships CASCADE, mêmes colonnes) — UNIQUE (membership_id) WHERE type='PASSWORD'
  - table `gate_pins` (id, gate_id FK, hashed_pin, label, metadata JSONB, created_at)
- [x] `000005_permissions` : table `permissions` (seed), table `membership_policies` (membership_id FK, gate_id FK, permission_code FK, PK composite)

### R2 — Models
- [x] Supprimer `model/member.go`
- [x] Créer `model/membership.go` : struct `WorkspaceMembership` (id, workspace_id, user_id nullable, local_username, display_name, role, auth_config, created_at)
- [x] Mettre à jour `model/workspace.go` : ajouter `Slug`, `MemberAuthConfig`, supprimer `WorkspaceMember`
- [x] Mettre à jour `model/policy.go` : remplacer `GatePolicy` par `MembershipPolicy`
- [x] Mettre à jour `model/credential.go` : ajouter `Label`, `ExpiresAt` ; supprimer target_type ENUM (simplifié, user_id direct)
- [x] Créer `model/membership_credential.go` : struct `MembershipCredential` (même structure que Credential mais membership_id)
- [x] Créer `model/gate_pin.go` : struct `GatePin`

### R3 — Repositories
- [x] Supprimer `repository/member.go`
- [x] Créer `repository/membership.go` : `WorkspaceMembershipRepository`
  - `Create(workspaceID, userID?, localUsername?, displayName, role)` → Membership
  - `GetByID(membershipID, workspaceID)` → Membership
  - `GetByLocalUsername(workspaceID, localUsername)` → Membership
  - `GetByUserID(workspaceID, userID)` → Membership
  - `List(workspaceID)` → []Membership
  - `UpdateRole(membershipID, workspaceID, role)` → Membership
  - `UpdateAuthConfig(membershipID, authConfig)` → Membership
  - `LinkUser(membershipID, userID)` (merge : set user_id)
  - `Delete(membershipID, workspaceID)` (hard delete)
- [x] Mettre à jour `repository/workspace.go` : supprimer méthodes workspace_members, ajouter `GetBySlug`, `UpdateMemberAuthConfig`
- [x] Mettre à jour `repository/policy.go` : utiliser `membership_id`
- [x] Mettre à jour `repository/gate.go` : `ListForWorkspace` utilise `membership_id` (JOIN membership_policies)
- [x] Créer `repository/gate_pin.go` : CRUD gate_pins
- [x] Mettre à jour `repository/credential.go` : supprimer logique MEMBER/GATE, ajouter `ListByUser` (pour API tokens), `DeleteByID`
- [x] Créer `repository/membership_credential.go` : `MembershipCredentialRepository`
  - `Create(membershipID, type, hashedValue, label, expiresAt, metadata)` → MembershipCredential
  - `GetByMembership(membershipID, type)` → MembershipCredential (PASSWORD/SSO)
  - `ListByMembership(membershipID)` → []MembershipCredential (tous, pour API tokens)
  - `DeleteByID(credID, membershipID)`

### R4 — Services
- [x] Mettre à jour `service/auth.go` :
  - Login global PASSWORD : email + password → JWT `type=global`
  - Login local PASSWORD : workspace_slug + local_username + password → JWT `type=local`
  - `Merge(globalUserID, workspaceSlug, localUsername, localPassword)` → UPDATE atomique
  - Refresh tokens Redis pour les deux types (global = raw UUID, local = JSON)
- [x] Créer `service/membership.go` (fichier `service/member.go` réécrit) :
  - `CreateLocal`, `InviteUser`, `GetByID`, `List`, `Update`, `Delete`, `SetPassword`
  - `GetEffectiveAuthConfig` (func package-level)

### R5 — Middleware
- [x] Mettre à jour `middleware/auth.go` :
  - `AuthExtractor` : lire `type` claim → stocker `user_id` (global) ou `membership_id+workspace_id+role` (local) en contexte
  - Ajouter `RequireMembership` (global OU local)
  - `MemberRoleFromContext` pour récupérer le rôle injecté par local JWT
- [x] Mettre à jour `middleware/rbac.go` :
  - `workspaceAccess` utilise `WorkspaceMembershipRepository`
  - Injecte `wsMembershipIDKey` + `wsRoleKey` dans le contexte
  - `WorkspaceMembershipIDFromContext` exposé pour les handlers

### R6 — Handlers
- [x] Réécrire `handler/member.go` → `MembershipHandler`
  - `POST /api/workspaces/{ws_id}/members` (créer membre local)
  - `POST /api/workspaces/{ws_id}/members/invite` (inviter user plateforme existant)
  - `GET /api/workspaces/{ws_id}/members` (liste)
  - `GET /api/workspaces/{ws_id}/members/{membership_id}` (détail)
  - `PATCH /api/workspaces/{ws_id}/members/{membership_id}` (update)
  - `DELETE /api/workspaces/{ws_id}/members/{membership_id}` (hard delete)
- [x] Mettre à jour `handler/auth.go` :
  - `POST /api/auth/login/local` (login membre par workspace_slug+username+password)
  - `POST /api/auth/merge` (fusionner membership locale avec compte global)
- [x] Mettre à jour `handler/workspace.go` : ajouter `slug` dans Create, supprimer endpoints `/users`
- [x] Mettre à jour `handler/policy.go` : body `{membership_id, permission_code}`, path `policies/{membership_id}`
- [x] Mettre à jour `handler/gate.go` : hard delete, membership_id via contexte RBAC
- [x] Mettre à jour `cmd/server/main.go` : câbler nouveaux repos/services/handlers
- [x] Créer `handler/gate_pin.go` : CRUD gate_pins _(reporté à Phase 6)_
- [x] Créer `handler/credential.go` : gestion credentials user et membres
  - Platform users : `GET/POST /api/auth/me/credentials`, `POST /api/auth/me/api-tokens`, `DELETE /api/auth/me/credentials/{id}`, `PATCH /api/auth/me/password`
  - Local members (self) : mêmes 4 endpoints sur `/api/auth/local/me/…`
  - Admin : `GET/POST/DELETE /api/workspaces/{ws_id}/members/{membership_id}/credentials`, `POST …/password`
  - API tokens : format `gaty_<64hex>`, stockés en SHA-256 (lookup O(1) possible)

---

## Phase 6 : Guest Access (Code PIN Public)

- [x] Endpoint `POST /api/public/unlock` (gate_id dans le body)
- [x] Rate Limiting Redis (5 essais / 15 min / IP par gate_id, fenêtre fixe via ExpireNX)
- [x] Réponse à temps constant (padding via `time.Sleep`, minimum 400ms)
- [x] Recherche dans table `gate_pins`, validation bcrypt
- [x] Vérification des règles métier dans `metadata` JSONB (expires_at, allowed_days, allowed_hours_start/end)
- [x] Publication MQTT de la commande d'ouverture en cas de succès
- [ ] Écriture dans `audit_logs` (succès et échecs) _(table supprimée en Phase R, reporté)_
- [x] CRUD des PIN codes pour les admins (`POST/GET/DELETE /api/workspaces/{ws_id}/gates/{gate_id}/pins`)

---

## Phase 7 : SSO (Single Sign-On)

> Architecture adaptée au refactor Phase R : SSO workspace uniquement (local JWT), membership_credentials pour SSO_IDENTITY, auto-provisioning crée un `workspace_membership`.

- [x] Endpoint `GET /api/auth/sso/{ws_slug}/authorize` (redirection vers le provider OIDC via discovery)
- [x] Endpoint `GET /api/auth/sso/{ws_slug}/callback` (échange code→tokens, vérification ID token)
- [x] Validation du ID token OIDC (signature, issuer, audience via go-oidc/v3)
- [x] État anti-CSRF : state random stocké dans Redis (TTL 10 min), consommé à usage unique
- [x] Cache OIDC provider (sync.RWMutex) pour éviter les appels discovery répétés
- [x] Auto-Provisioning Just-In-Time : création `workspace_membership` + credential `SSO_IDENTITY` si auto_provision=true
- [x] Lecture des claims + role mapping via `role_claim` / `role_mapping` dans les settings
- [x] Stockage du credential SSO_IDENTITY dans `membership_credentials` (post-Phase R)
- [x] Redirection frontend vers `{frontendURL}/auth/sso/callback?access_token=...&refresh_token=...`
- [x] Gestion erreurs provider → redirection `?error={code}` (invalid_state, access_denied, server_error)
- [x] Endpoint `GET /api/workspaces/{ws_id}/sso-settings` (lecture config SSO, wsAdmin)
- [x] Endpoint `PATCH /api/workspaces/{ws_id}/sso-settings` (mise à jour config SSO, wsAdmin)
- [x] `BASE_URL` et `FRONTEND_URL` ajoutés à config.go (avec defaults dev)

---

## Phase 8 : Domaines Personnalisés

> Un domaine custom pointe vers **une gate spécifique** (page unlock/PIN pad).
> Deux modes de déploiement supportés — même backend, docker-compose différent.
>
> **Mode A — Proxy bundlé (Caddy)** : `docker-compose --profile caddy up`
> Caddy On-Demand TLS : appelle `GET /api/public/verify-domain` avant d'émettre un cert.
> DNS pointé sur le serveur → TLS Let's Encrypt automatique, zéro config.
>
> **Mode B — Proxy externe** : l'utilisateur apporte son proxy (nginx, traefik…).
> Il conserve le header `Host` et peut utiliser `GET /api/public/verify-domain`
> comme endpoint ACME `ask` s'il en a besoin. TLS géré par son proxy.

### 8.1 - Migration & Modèle
- [x] Migration `000006_custom_domains` :
  ```sql
  custom_domains (
    id UUID PK, workspace_id FK, gate_id FK → gates CASCADE,
    domain TEXT UNIQUE NOT NULL,
    dns_challenge_token TEXT NOT NULL DEFAULT encode(gen_random_bytes(24), 'hex'),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
  )
  INDEX ON custom_domains(gate_id), INDEX ON custom_domains(domain)
  ```
- [x] Model `CustomDomain` (`model/custom_domain.go`)
- [x] Repository `CustomDomainRepository` (`repository/custom_domain.go`)

### 8.2 - Backend endpoints
- [x] `POST   /api/workspaces/{ws_id}/gates/{gate_id}/domains` — ajouter un domaine custom (wsAdmin)
- [x] `GET    /api/workspaces/{ws_id}/gates/{gate_id}/domains` — lister les domaines d'une gate (wsAdmin)
- [x] `DELETE /api/workspaces/{ws_id}/gates/{gate_id}/domains/{domain_id}` — supprimer (wsAdmin)
- [x] `POST   /api/workspaces/{ws_id}/gates/{gate_id}/domains/{domain_id}/verify` — déclenche la vérification DNS TXT (wsAdmin)
  - Résout `_gaty.<domain>` TXT → compare au `dns_challenge_token`
  - Si OK : `verified_at=now()`
- [x] `GET /api/public/verify-domain?domain=xxx` — endpoint ACME (Caddy `ask`) ; 200 si vérifié, 403 sinon
- [x] `GET /api/public/domains/list` — liste de tous les domaines vérifiés (scripts d'automatisation proxy externe)
- [ ] `GET /api/public/resolve?domain=xxx` — résolution pour le frontend sur domaine custom _(Phase 9 — frontend)_

### 8.3 - Tenant Middleware
- [x] Mettre à jour `middleware/tenant.go` : Host header → lookup `custom_domains` → injecter `TenantTypeGate` + `gate_id` en contexte

### 8.4 - Infrastructure (docker-compose)
- [x] `configs/Caddyfile.prod` (On-Demand TLS) :
  ```
  { on_demand_tls { ask http://app:8080/api/public/verify-domain } }
  {$APP_DOMAIN} { reverse_proxy app:8080 }
  :443 { tls { on_demand } reverse_proxy app:8080 }
  ```
- [x] `docker-compose.yml` : profil `prod` pour le service Caddy (désactivé en dev)
- [ ] Documentation : comment configurer un proxy externe (nginx, traefik) pour le mode B _(Phase 11)_

---

## Phase 9 : Frontend React

> Stack : React 19 + Vite + TypeScript + Tailwind v4 + shadcn/ui (New York)
> Router : React Router v7 · State : TanStack Query v5 + Zustand · HTTP : Axios

### 9.1 - Setup & Architecture
- [x] Structure de dossiers (types/, lib/, store/, layouts/, pages/)
- [x] Configuration du routeur : React Router v7 (`createBrowserRouter`)
- [x] Client API Axios (`src/lib/api.ts`) : intercepteur JWT, refresh automatique sur 401, drain de file d'attente
- [x] Store d'authentification Zustand (`src/store/auth.ts`) : persist localStorage, `setAuth`, `logout`, `isAuthenticated`
- [x] Layout principal (`AppLayout`) : sidebar workspace-switcher + nav + user footer
- [x] Layout auth (`AuthLayout`) : centered card
- [ ] Thème dynamique : injection des couleurs du tenant via CSS variables _(Phase 11)_

### 9.2 - Pages Auth
- [x] Page Login (email/password) → JWT global
- [x] Page Register
- [x] Gestion du refresh token (intercepteur Axios, rotation automatique)
- [ ] Bouton "Se connecter avec SSO" _(Phase 11 — dépend du workspace slug)_

### 9.3 - Dashboard Workspace
- [x] Page liste des workspaces (`/workspaces`) : création inline, navigation
- [x] Page dashboard d'un workspace (`/workspaces/:wsId`) : grille de gates
- [x] Indicateur de statut gate : dot pulsant (online), WifiOff (offline), HelpCircle (unknown), polling 10s
- [x] Bouton "Open" rapide directement sur la card gate
- [x] Page de gestion des membres (`/workspaces/:wsId/members`) : invite par email ou création locale
- [x] Page Settings (`/workspaces/:wsId/settings`) : configuration SSO OIDC du workspace

### 9.4 - Gestion des Gates
- [x] Page détail d'une gate (`/workspaces/:wsId/gates/:gateId`) : statut + trigger
- [x] Bouton "Open gate" avec feedback loading/success
- [x] Gestion des PIN codes : liste + création + suppression + affichage expires_at
- [x] Gestion des domaines custom : liste + ajout + DNS challenge token + vérification + suppression
- [ ] Page permissions d'une gate (attribution membership_policies) _(Phase 10)_

### 9.5 - Vue Guest (Domaine ciblant une Gate)
- [x] Page PIN pad (`/unlock` et `/unlock/:gateId`) plein écran, mobile-first
- [x] Résolution automatique via `GET /api/public/resolve?domain=<host>` si pas de gateId URL
- [x] Feedback visuel : dot states, CheckCircle (succès), XCircle (erreur)
- [x] Gestion des erreurs : 429 rate limit, 403 PIN invalide, timeout
- [x] Auto-submit à 12 chiffres, bouton Confirm pour longueurs intermédiaires

### 9.6 - Temps Réel (SSE)
- [ ] Endpoint SSE backend (`GET /api/workspaces/:ws_id/gates/events`)
- [ ] Bridge MQTT → SSE via Redis Pub/Sub (multi-instance safe)
- [ ] Hook React `useGateEvents()` basé sur `EventSource` avec reconnexion auto
- [ ] Mise à jour automatique de l'UI lors d'un changement de statut _(polling 10s comme fallback déjà en place)_

### 9.7 - Backend : résolution domaine public
- [x] `GET /api/public/resolve?domain=xxx` — retourne `{gate_id, gate_name, workspace_id, workspace_slug, workspace_name}` si domaine vérifié

---

## Phase 10 : Tests & Qualité

### 10.1 - Tests Unitaires (Go)
- [ ] Tests du service RBAC (vérification des rôles et permissions)
- [ ] Tests de la validation PIN (bcrypt, règles métier metadata, temps constant)
- [ ] Tests du Tenant Resolution middleware
- [ ] Tests du rate limiter Redis (mock)
- [ ] Tests de l'auto-provisioning SSO (mapping rules)

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
