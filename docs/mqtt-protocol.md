# Gaty MQTT Protocol

## Modes disponibles

| Mode | Usage | Config requise |
|------|-------|----------------|
| `MQTT_GATIE` | Protocole natif Gaty — format fixe, aucune config | Aucune |
| `MQTT_CUSTOM` | Payload/mapping personnalisé | open/close: `payload` ; status: `mapping` |
| `NONE` | Désactivé | — |

`MQTT_GATIE` est recommandé pour les devices Gaty. `MQTT_CUSTOM` est pour les devices tiers.

---

## Topics MQTT

Les topics sont fixes, dérivés des IDs workspace et gate.

```
Statut  (device → serveur) : workspace_{wsID}/gates/{gateID}/status
Commande (serveur → device) : workspace_{wsID}/gates/{gateID}/command
```

---

## Protocole MQTT_GATIE

### Commande (serveur → device)

Le serveur publie sur `workspace_{wsID}/gates/{gateID}/command` :

```json
{ "action": "open" }
```
ou
```json
{ "action": "close" }
```

Champs :
- `action` : `"open"` ou `"close"` (string, obligatoire)

### Statut (device → serveur)

Le device publie sur `workspace_{wsID}/gates/{gateID}/status` :

```json
{
  "token": "<gate_jwt>",
  "status": "open",
  "battery": 85,
  "signal_dbm": -70
}
```

Champs :
- `token` : JWT du device, **obligatoire** — utilisé pour l'authentification
- `status` : statut courant (`"open"`, `"closed"`, `"unavailable"`, ou statut custom), **obligatoire**
- Autres champs : données supplémentaires (batterie, signal, etc.)

Le device doit envoyer le statut sous forme de chaîne finale (ex: `"open"`) — pas de traduction côté serveur en mode `MQTT_GATIE`.

Seuls les champs configurés dans `meta_config` sont extraits et stockés. Chaque clé est lue directement depuis la racine du payload. Exemple : `battery` → configurer `key: "battery"` dans le meta config.

---

## Protocole MQTT_CUSTOM

Topics identiques à `MQTT_GATIE`.

### Commande (serveur → device)

Config dans `open_config.config` / `close_config.config` :

```json
{
  "payload": { "cmd": 1, "target": "open" }
}
```

- `payload` : objet JSON à publier tel quel, **obligatoire**

### Statut (device → serveur)

Le device publie sur le topic de statut. Le payload doit toujours contenir `token` à la racine pour l'authentification.

Config dans `status_config.config` :

```json
{
  "mapping": {
    "status": {
      "field": "state",
      "values": { "1": "open", "0": "closed" }
    }
  }
}
```

- `mapping.status.field` : chemin dot-noté vers la valeur de statut dans le payload, **obligatoire**
- `mapping.status.values` : table de traduction `valeur_brute → statut_app`, **obligatoire** — doit couvrir `"open"` et `"closed"`

Les métadonnées sont configurées via `meta_config` (commun à tous les modes). Chaque clé est lue directement depuis la racine du payload via dot-notation.

Exemple de payload device :

```json
{
  "token": "<gate_jwt>",
  "state": 1,
  "batt": 92
}
```

Avec `meta_config: [{key: "batt", label: "Batterie", unit: "%"}]` → status `"open"`, meta `{"batt": 92}`

---

## Authentification

Dans les deux modes, le device **doit** inclure son JWT (`token`) à la racine du payload de statut.

Vérification en deux étapes :
1. Signature JWT vérifiée avec le secret serveur
2. Lookup DB par `gate_id` + token pour détecter les rotations

Un token révoqué (après rotation) est rejeté même si la signature est valide.

