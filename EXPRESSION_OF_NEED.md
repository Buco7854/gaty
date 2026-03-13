# GATIE — Expression de Besoin Fonctionnel

## 1. Vision du produit

GATIE est une plateforme SaaS de contrôle de portails/barrières IoT. Elle permet à des organisations de gérer à distance l'ouverture, la fermeture et la supervision de leurs accès physiques (portails, barrières, portes), avec un contrôle granulaire des permissions, des plages horaires, et un accès invité par code PIN.

---

## 2. Concepts clés

### 2.1 Membres et rôles

L'application fonctionne avec un unique niveau d'identité : les **membres**. Chaque membre appartient à une instance GATIE et possède un rôle :

- **ADMIN** : accès total à la gestion (portails, membres, plannings, paramètres)
- **MEMBER** : accès restreint aux portails sur lesquels des permissions lui ont été attribuées

### 2.2 Portails (Gates)

Un portail représente un équipement physique (portail, barrière, porte connectée) contrôlable à distance. Chaque portail possède :

- Un **nom**
- Un **statut** en temps réel (en ligne, ouvert, fermé, hors ligne, non réactif, indisponible…)
- Des **actions configurables** : ouverture, fermeture, remontée de statut
- Des **métadonnées en direct** (température, niveau batterie, signal, etc.)
- Un **jeton d'authentification** pour l'appareil physique

### 2.3 Permissions

Les droits d'accès sont attribués **par membre et par portail** avec les permissions suivantes :

- **Déclencher l'ouverture** d'un portail
- **Déclencher la fermeture** d'un portail
- **Consulter le statut** et les données en direct d'un portail
- **Gérer** un portail (configuration, codes PIN, domaines, permissions des autres membres)

### 2.4 Plannings horaires (Schedules)

Les plannings permettent de restreindre temporellement un accès. Ils sont composés d'expressions combinables :

- **Plage horaire** : jours de la semaine + heure de début/fin (ex. : lundi–vendredi 08h–18h)
- **Plage de jours de la semaine** : jours consécutifs avec wrap-around (ex. : samedi–dimanche)
- **Plage de dates** : dates calendaires (ex. : 01/06/2026–31/08/2026)
- **Plage de jours du mois** : jours récurrents (ex. : du 1er au 15 de chaque mois)
- **Plage de mois** : mois récurrents (ex. : juin–septembre)

Les expressions sont combinables avec des opérateurs logiques **ET**, **OU**, **NON** pour créer des règles complexes (ex. : "lundi–vendredi 08h–18h ET pas en août").

Les plannings peuvent être attachés à :
- Un **membre sur un portail** : le membre ne peut agir que pendant les plages autorisées
- Un **code PIN** : le code n'est valide que pendant les plages autorisées

Il existe deux types de plannings :
- **Plannings d'administration** : créés par les admins, utilisables sur n'importe quel membre ou code
- **Plannings personnels** : créés par un membre pour ses propres besoins

---

## 3. Fonctionnalités détaillées

### 3.1 Setup initial

Au premier lancement, l'application détecte qu'aucun membre n'existe et propose un écran de configuration initiale :

- Création du premier compte administrateur (nom d'utilisateur + mot de passe)
- Connexion automatique après création
- Cette étape n'est disponible qu'une seule fois

### 3.2 Authentification

#### 3.2.1 Connexion par mot de passe

- Un membre se connecte avec son **nom d'utilisateur** et son **mot de passe**
- Le système délivre un jeton d'accès (courte durée) et un jeton de rafraîchissement (longue durée)
- Le rafraîchissement est automatique et transparent pour l'utilisateur
- Déconnexion : révocation du jeton de rafraîchissement

#### 3.2.2 Connexion SSO (Single Sign-On)

- Les fournisseurs SSO (OIDC) sont configurés au niveau de l'instance
- L'écran de connexion membre affiche les boutons SSO disponibles
- Flux : redirection vers le fournisseur → callback → échange de code → connexion
- **Auto-provisioning** : si activé, un membre est automatiquement créé lors de sa première connexion SSO
- Mapping de rôle : le rôle du membre peut être déduit d'un claim du token SSO

#### 3.2.3 Jetons API (API Tokens)

Chaque membre peut créer des **jetons API** pour un accès programmatique :

- Libellé descriptif
- Date d'expiration optionnelle
- Restriction optionnelle des permissions : le jeton n'aura accès qu'aux portails et permissions sélectionnés
- Restriction temporelle optionnelle : le jeton est soumis à un planning
- Le jeton brut n'est affiché qu'une seule fois à la création

Les administrateurs peuvent aussi créer et révoquer des jetons pour les autres membres.

#### 3.2.4 Changement de mot de passe

- Un membre peut changer son propre mot de passe (ancien + nouveau requis)
- Un administrateur peut réinitialiser le mot de passe d'un membre (sans connaître l'ancien)

### 3.3 Gestion des membres

*Réservé aux administrateurs.*

- **Créer** un membre : nom d'utilisateur, nom d'affichage optionnel, mot de passe, rôle (membre ou admin)
- **Lister** les membres avec pagination
- **Consulter** les détails d'un membre
- **Modifier** un membre : nom d'affichage, nom d'utilisateur, rôle
- **Supprimer** un membre (suppression définitive)

#### 3.3.1 Configuration d'authentification par membre

Chaque membre peut hériter de la configuration par défaut ou avoir une surcharge individuelle pour :

- Activation/désactivation de la connexion par mot de passe
- Activation/désactivation du SSO
- Activation/désactivation des jetons API

Trois états possibles : **hériter** (utilise la valeur par défaut), **activé**, **désactivé**.

### 3.4 Gestion des portails

#### 3.4.1 CRUD des portails

*Création et suppression réservées aux administrateurs. Modification réservée aux administrateurs et gestionnaires du portail.*

- **Créer** un portail : nom, type d'intégration, configuration des actions
- **Lister** les portails :
  - Un administrateur voit tous les portails
  - Un membre ne voit que les portails sur lesquels il a au moins une permission
- **Consulter** les détails d'un portail : statut en direct, métadonnées, codes d'accès, domaines
- **Modifier** la configuration d'un portail
- **Supprimer** un portail (suppression définitive)

#### 3.4.2 Actions sur un portail

- **Ouvrir** un portail : envoie la commande d'ouverture à l'appareil
- **Fermer** un portail : envoie la commande de fermeture (si configuré)
- Les actions respectent les permissions et les plannings horaires du membre

#### 3.4.3 Configuration des actions

Chaque portail possède trois actions configurables indépendamment : **ouverture**, **fermeture**, **remontée de statut**. Chaque action peut être de type :

- **MQTT** : publication d'un message sur un topic du broker
- **HTTP** : appel à un webhook externe (URL, méthode, headers, body)
- **Aucune** : action désactivée

#### 3.4.4 Statut et données en direct

- L'appareil pousse son statut vers le serveur (via MQTT ou HTTP entrant)
- Le statut est interprété via un **mapping configurable** (correspondance entre les champs reçus et les statuts affichés)
- Des **règles de statut** permettent de surcharger le statut affiché selon les métadonnées (ex. : si `battery < 20` → statut "low_battery")
- Des **transitions de statut** permettent de programmer un changement automatique après un délai (ex. : si statut "open" depuis 30s → passer à "closed")
- Le système détecte automatiquement les appareils **non réactifs** si aucune donnée n'est reçue dans un délai configurable (TTL)
- Les **statuts personnalisés** peuvent être définis en plus des statuts par défaut (open, closed, unavailable)

#### 3.4.5 Configuration des métadonnées

L'administrateur peut configurer quelles métadonnées sont affichées en direct, avec :

- Clé (chemin dans les données reçues, ex. : `lora.snr`)
- Libellé affiché (ex. : "Signal LoRa")
- Unité (ex. : "dBm")

#### 3.4.6 Jeton d'appareil (Gate Token)

Chaque portail possède un jeton secret utilisé par l'appareil physique pour s'authentifier auprès du serveur :

- Généré automatiquement à la création du portail (affiché une seule fois)
- **Rotation** possible : génère un nouveau jeton et invalide l'ancien
- Consultable par les administrateurs

### 3.5 Codes d'accès (PIN / Mot de passe)

*Gestion réservée aux gestionnaires du portail.*

Les codes d'accès permettent un accès public à un portail sans compte membre.

#### 3.5.1 Types de codes

- **Code PIN** : suite de chiffres (minimum 4), saisie via pavé numérique
- **Mot de passe** : chaîne de texte libre

#### 3.5.2 Création et gestion

- **Créer** un code : type (PIN/mot de passe), libellé, métadonnées de contrôle
- **Lister** les codes d'un portail
- **Modifier** un code : libellé, métadonnées
- **Supprimer** un code
- **Attacher un planning** : le code n'est valide que pendant les plages horaires définies
- **Détacher un planning**

#### 3.5.3 Métadonnées de contrôle d'un code

- **Date d'expiration** : le code devient invalide après cette date
- **Nombre d'utilisations maximum** : le code se désactive après N utilisations
- **Durée de session** : si défini, le code ouvre une session temporaire au lieu d'un déclenchement unique
- **Permissions de session** : quelles actions sont autorisées pendant la session (ouvrir, fermer, consulter le statut)

#### 3.5.4 Accès public par code

Deux modes de fonctionnement :

**Mode instantané (one-shot)** :
- L'utilisateur saisit un code PIN ou mot de passe
- Si valide : le portail s'ouvre immédiatement
- Pas de session, pas de contexte persistant

**Mode session** :
- L'utilisateur saisit un code configuré avec une durée de session
- Une session temporaire est créée avec les permissions définies dans le code
- L'utilisateur peut ensuite ouvrir/fermer le portail et consulter son statut pendant la durée de la session
- La session expire automatiquement

#### 3.5.5 Sécurité des codes

- **Limitation de tentatives** : maximum de tentatives par IP et par portail sur une fenêtre de temps (anti brute-force)
- **Limitation globale** : maximum de tentatives par IP tous portails confondus
- **Temps de réponse constant** : protection contre les attaques par timing

### 3.6 Permissions et politiques d'accès

#### 3.6.1 Attribution des permissions

- Un administrateur ou gestionnaire de portail peut **accorder** ou **révoquer** des permissions à un membre sur un portail
- Permissions disponibles : ouverture, fermeture, consultation du statut, gestion
- Les permissions sont indépendantes les unes des autres

#### 3.6.2 Restrictions temporelles par membre

- Un planning peut être attaché à un couple **membre + portail**
- Le membre ne pourra agir sur ce portail que pendant les plages autorisées
- La restriction s'applique en plus des permissions (il faut la permission ET être dans la plage horaire)

#### 3.6.3 Permissions par défaut

- L'administrateur peut configurer des **permissions par défaut** qui s'appliquent à tous les nouveaux membres
- Configuration via la page des paramètres

### 3.7 Domaines personnalisés

Chaque portail peut avoir un ou plusieurs **domaines personnalisés** pointant vers sa page d'accès public.

#### 3.7.1 Gestion des domaines

- **Ajouter** un domaine personnalisé à un portail
- **Vérifier** le domaine par challenge DNS : l'administrateur doit créer un enregistrement TXT `_gatie.{domain}` avec le jeton fourni
- **Supprimer** un domaine
- **Lister** les domaines d'un portail avec leur état de vérification

#### 3.7.2 Fonctionnement

- Un domaine vérifié redirige automatiquement vers la page d'accès du portail associé
- Le certificat TLS est provisionné automatiquement (via le proxy)
- La résolution du domaine vers le portail est publique (pas d'authentification requise)

### 3.8 Temps réel (Server-Sent Events)

L'application diffuse les événements en temps réel :

- **Changements de statut** des portails : mis à jour instantanément dans l'interface
- **Métadonnées en direct** : température, signal, batterie… actualisés en continu
- Les événements transitent de l'appareil vers le serveur, puis sont redistribués à tous les clients connectés
- Reconnexion automatique en cas de perte de connexion

### 3.9 Paramètres de l'instance

*Réservé aux administrateurs.*

- **Configuration d'authentification par défaut des membres** :
  - Activer/désactiver la connexion par mot de passe
  - Activer/désactiver le SSO
  - Activer/désactiver les jetons API
  - Nombre maximum de jetons API par membre
  - Durée de session par défaut
- **Fournisseurs SSO** : consultation des fournisseurs configurés (la configuration se fait au niveau de l'infrastructure)
- **Permissions par défaut** : grille de permissions par portail applicables aux nouveaux membres

### 3.10 Health check

- Le système expose un point de contrôle de santé vérifiant la connectivité à la base de données et au cache

---

## 4. Interfaces utilisateur

### 4.1 Écrans d'administration (membres authentifiés)

#### 4.1.1 Connexion

- Formulaire nom d'utilisateur + mot de passe
- Boutons SSO si des fournisseurs sont configurés

#### 4.1.2 Setup initial

- Formulaire de création du premier administrateur (nom d'utilisateur, mot de passe, confirmation)

#### 4.1.3 Tableau de bord des portails

- Grille responsive de cartes représentant les portails
- Chaque carte affiche : nom, statut (badge coloré), type d'intégration
- Bouton d'ouverture rapide directement sur la carte
- Bouton d'accès aux détails/paramètres
- Bouton de création de portail (admin uniquement)
- Mise à jour en temps réel des statuts via SSE

#### 4.1.4 Détail d'un portail

- Statut en direct avec badge coloré
- Boutons d'ouverture et fermeture
- **Section données en direct** : métadonnées clé-valeur actualisées en temps réel
- **Section jeton d'appareil** (admin) : afficher/masquer, copier, rotation avec confirmation
- **Section codes d'accès** : liste des codes avec libellé, type, expiration, planning ; création/édition/suppression
- **Section domaines personnalisés** : liste avec statut de vérification, instructions DNS, boutons vérifier/supprimer
- **Dialogue de configuration** : actions (ouvrir/fermer/statut), métadonnées affichées, statuts personnalisés, règles de statut, TTL, transitions de statut

#### 4.1.5 Gestion des membres

- Liste des membres avec avatar, nom, rôle
- Création de membre via formulaire modal
- Édition des informations (nom, rôle)
- **Dialogue de paramétrage par membre** avec onglets :
  - **Permissions** : grille portails × permissions (cases à cocher)
  - **Plannings** : sélection d'un planning par portail
  - **Surcharges d'authentification** : trois boutons (hériter/activé/désactivé) par méthode

#### 4.1.6 Gestion des plannings

- Deux onglets : plannings personnels et plannings d'administration
- Carte de planning affichant : nom, type d'expression, description, résumé lisible de la règle
- **Éditeur d'expression visuel** : construction arborescente avec opérateurs ET/OU/NON, types de règles sélectionnables, imbrication avec code couleur

#### 4.1.7 Paramètres

- Configuration des méthodes d'authentification par défaut
- Liste des fournisseurs SSO (lecture seule)
- Grille des permissions par défaut

#### 4.1.8 Gestion des jetons API personnels

- Accessible depuis le menu utilisateur
- Formulaire de création : libellé, expiration, planning, restriction de permissions
- Liste des jetons existants avec suppression
- Affichage unique du jeton brut à la création avec bouton copier

### 4.2 Écrans publics (accès invité)

#### 4.2.1 Portail d'accès (Gate Portal)

Page d'accueil d'un portail accessible via domaine personnalisé ou lien direct. Trois options :

- Saisir un code PIN
- Saisir un mot de passe
- Se connecter en tant que membre

Si une session est active (PIN avec session ou membre connecté) :
- Boutons ouvrir/fermer selon les permissions
- Données en direct si permission de consultation
- Lien vers le tableau de bord (si membre admin)

#### 4.2.2 Pavé numérique (PIN Pad)

- Affichage du nom du portail
- Indicateurs visuels (points) du PIN saisi
- Pavé numérique 3×4 avec retour arrière
- Bouton de confirmation
- Possibilité de basculer en mode mot de passe
- Retour visuel : succès (coche verte), erreur (croix rouge), trop de tentatives

#### 4.2.3 Saisie mot de passe

- Champ de saisie masqué
- Bouton de confirmation
- Navigation vers les autres méthodes

#### 4.2.4 Connexion membre (contexte portail)

- Formulaire nom d'utilisateur + mot de passe
- Boutons SSO si disponibles
- Après connexion réussie : redirection automatique vers le portail avec session active
- Lien vers les autres méthodes d'accès

### 4.3 Éléments transversaux

- **Sélecteur de thème** : mode clair / mode sombre
- **Sélecteur de langue** : internationalisation
- **Design responsive** : mobile-first, adaptation tablette et desktop
- **Navigation latérale** (desktop) / menu hamburger (mobile)
- **États de chargement** : spinners sur toutes les opérations asynchrones
- **Messages d'erreur** : alertes colorées avec descriptions claires
- **Infobulles** : aide contextuelle sur les boutons d'action
- **Copier dans le presse-papiers** : avec feedback visuel de confirmation

---

## 5. Règles métier transversales

### 5.1 Sécurité

- Les mots de passe respectent des exigences de complexité configurable (longueur minimale, majuscule, minuscule, chiffre)
- Les jetons API sont hashés en base et ne sont affichés qu'une seule fois
- Les codes PIN sont hashés (bcrypt) et jamais exposés en clair
- Les endpoints sensibles sont protégés par limitation de débit (rate limiting)
- Les réponses aux tentatives de code sont à durée constante (anti-timing)
- Les états anti-CSRF sont à usage unique avec expiration courte

### 5.2 Contrôle d'accès

- Toute action sur un portail vérifie : (1) la permission du membre, (2) le planning horaire si applicable
- Les administrateurs ont un accès implicite à tous les portails
- Les membres ne voient que les portails sur lesquels ils ont des permissions

### 5.3 Temps réel

- Les statuts de portail sont propagés en temps réel depuis l'appareil jusqu'à l'interface
- Un appareil qui ne communique plus est automatiquement marqué comme "non réactif" après un délai configurable
- Les transitions de statut automatiques se déclenchent après un délai défini

### 5.4 Gestion des sessions

- Les sessions membres ont une durée configurable avec rafraîchissement automatique
- Les sessions PIN ont une durée définie par le code utilisé
- La déconnexion révoque immédiatement la session côté serveur
