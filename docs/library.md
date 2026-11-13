# Bibliothèque — documentation (brouillon)

**Statut :** back implémenté (API, persistance, tests). Interface canvas à faire.
**Périmètre :** API + interface (canvas). Pas de partage entre utilisateurs pour l'instant.

## Objectif

Permettre à l'utilisateur de garder des **calls réutilisables** (modèles) dans une bibliothèque personnelle. Depuis le canvas d'un workflow, il glisse un modèle de la bibliothèque : cela crée un **vrai call** dans le workflow, identique à un call créé à la main, qu'il peut ensuite modifier librement.

Exemples : « Login ERP », « GET /health », « Création facture Stripe », « Webhook Slack d'alerte ».

## Concepts

| Concept | Définition |
| --- | --- |
| Modèle (library item) | Copie figée de la définition d'un call, stockée dans la bibliothèque de l'utilisateur |
| Insertion | Création d'un call dans un workflow à partir d'un modèle |
| Call | Toujours un call normal une fois inséré. Aucun lien vivant avec le modèle |

**Principe clé : copie, pas référence.** Insérer un modèle copie sa définition dans le workflow. Modifier ou supprimer un modèle n'affecte jamais les calls déjà insérés, et modifier un call n'affecte jamais le modèle. Évite les effets de bord silencieux sur des workflows en production.

## Contenu d'un modèle

Un modèle contient les mêmes champs que le payload de création d'un call (`CallRequest`) :

- `name`, `description`
- `method`, `url`, `headers`, `query`, `body`
- `timeoutMs`, `verifyTls`, `ipVersion`, `proxyUrl`
- `rules` (règles d'alerte)

**Hors modèle (v1) :**

- **Variables extraites** (`/calls/:callId/variables`) : gérées séparément dans l'API des calls, donc pas incluses. À reconsidérer si les modèles doivent embarquer leur extraction.
- **Position** : c'est le workflow qui la décide à l'insertion.
- **Résultats et historique** : jamais copiés.

## Propriété

Tout est lié à l'utilisateur authentifié (`user_id`), jamais à une organisation en v1.

- **Modèles** : créés avec le `user_id` de l'appelant. Lecture, modification, suppression et insertion ne voient que les modèles de l'utilisateur courant. Un modèle d'un autre utilisateur est traité comme inexistant (même comportement que les workflows).
- **Insertion** : le workflow cible doit appartenir à l'utilisateur (`loadOwnedWorkflow`, comme la création de call). Le modèle doit aussi lui appartenir.
- **From-call** : le call source doit appartenir à un workflow de l'utilisateur. La copie ne contient que la définition du call, jamais ses runs ni son workflow.
- **Import** : les modèles importés sont créés avec le `user_id` de l'appelant. Rien n'est partagé.
- **Calls insérés** : appartiennent au workflow, donc à l'utilisateur, comme tout call créé à la main.

## Validation

- Un modèle est validé **comme un call** à l'enregistrement (mêmes règles que `domain/call` : méthode, URL, timeout, headers, proxy, règles). Un modèle enregistré est donc toujours insérable sans erreur propre au modèle.
- Les erreurs qui dépendent du **contexte du workflow** restent possibles à l'insertion, notamment :
  - une URL, un header ou un body qui référence une variable `{{nom}}` absente du workflow au moment de l'insertion (variables validées dans `AddCall`, ordre des variables) ;
  - la limite de calls par workflow (`MaxCalls`) ;
  - le plan de l'utilisateur ou ses quotas.
- L'interface doit afficher l'erreur renvoyée par l'API telle quelle, sans créer de call partiel.

## Import dans la bibliothèque

Un modèle peut aussi venir d'un fichier ou d'un texte existant, sans ressaisie. Formats v1 :

| Source | Entrée | Un modèle produit |
| --- | --- | --- |
| **OpenAPI 3.x** | Document JSON ou YAML | Une opération (`paths` × méthode) |
| **cURL** | Une ou plusieurs commandes `curl ...` | Une commande |
| **Postman** | Collection v2.1 (JSON) | Une requête (dossiers aplatis, nom du dossier conservé dans `description`) |

Hors v1 : HAR, Insomnia, Bruno, Swagger 2.0, Postman v2.0.

### Flux en deux temps

L'import ne sauvegarde rien tout de suite. L'utilisateur voit d'abord ce qui sera créé :

1. **Prévisualisation** : `POST /library/import/preview` renvoie les modèles candidats et les avertissements.
2. **Confirmation** : `POST /library/import` avec les candidats retenus (éventuellement renommés ou décochés) crée les modèles.

Cela évite d'encombrer la bibliothèque avec 80 opérations d'un OpenAPI dont on n'a besoin que de 3.

### Règles de conversion

**OpenAPI**

- `method` et `path` → `method` et `url`. Le serveur (`servers[0].url`) préfixe l'URL. Si présent, il devient une variable `{{base_url}}` proposée dans l'avertissement, et n'est pas figé dans l'URL.
- `summary` → `name` (repli : `operationId`, puis `METHOD path`). `description` → `description` (tronquée à 500 caractères, avec avertissement).
- Paramètres `in: query` → `query`, valeur d'exemple (`example`, `default`, premier élément d'`enum`) ou vide. Paramètres `in: header` → `headers`. Paramètres `in: path` → `{id}` remplacé par `{{id}}` et noté comme variable à fournir.
- `requestBody` (`application/json` en priorité) → `body`, à partir de l'`example` ou d'un exemple généré depuis le schéma (champs à leur valeur par défaut, sinon vide).
- `security` → aucun secret inséré. Un avertissement indique le schéma attendu (`bearerAuth`, `apiKey`) et propose un header `Authorization: Bearer {{token}}`.
- Les opérations marquées `deprecated: true` sont importées mais signalées.

**cURL**

- Options gérées : `-X`/`--request`, `-H`/`--header`, `-d`/`--data`/`--data-raw`/`--data-binary` (passe la méthode en `POST` si `-X` absent), `--url`, `-G` + `-d` (les données deviennent `query`), `-u` (converti en header `Authorization: Basic …`, avertissement secret), `-m`/`--max-time` → `timeoutMs`, `-k`/`--insecure` → `verifyTls: false`, `-x`/`--proxy` → `proxyUrl`, `-4` / `-6` → `ipVersion`.
- Options ignorées avec avertissement : `-o`, `-b`, `-c`, `-F`/`--form` (multipart non supporté en v1), `-L` (suivi de redirection, non supporté en v1 : avertissement à l'import).
- Plusieurs commandes sur plusieurs lignes (`\` de continuation) : un modèle par commande.

**Postman**

- Dossiers aplatis. Le chemin du dossier va dans `description`.
- `request.method`, `request.url` (chaîne ou objet `raw`), `request.header` (désactivés ignorés), `request.url.query` (désactivés ignorés), `request.body.raw` → `body`. Corps `formdata` et `urlencoded` : avertissement, non converti en v1.
- Les variables de collection et d'environnement `{{nom}}` sont **conservées telles quelles** (mêmes noms que les variables de workflow, à créer côté workflow).
- `auth` de collection ou de requête : même traitement que `security` OpenAPI.

### Secrets à l'import

Les trois sources contiennent souvent de vrais tokens. Règle v1 (décidée) : les valeurs importées sont **conservées telles quelles**, comme pour une saisie manuelle. Pas de remplacement automatique par variable.

- La prévisualisation signale quand même les headers sensibles (`Authorization`, `X-Api-Key`, `Cookie`, `X-Auth-Token`, etc.) pour que l'utilisateur sache ce qu'il importe, sans bloquer l'import.
- Les valeurs ne sont jamais écrites dans les logs d'import.

### Sécurité du flux

- **Pas de téléchargement d'URL** côté serveur en v1 (OpenAPI ou Postman par lien) : risque SSRF. Le fichier ou le texte est envoyé dans le corps de la requête. Un import par URL pourra venir plus tard, derrière le même contrôle que les URL de calls.
- Taille maximale du contenu : 2 Mo en v1, refus au-delà.
- Parsing avec les mêmes limites que la validation d'un call (méthode, URL, timeout, headers) : un candidat invalide n'est pas importé et apparaît dans `skipped` avec sa raison. Rien n'est silencieux.
- Les références YAML/JSON externes (`$ref` vers un fichier ou une URL) ne sont pas suivies. Les `$ref` internes (`#/components/...`) sont résolus.

### Routes d'import

| Méthode | Route | Rôle |
| --- | --- | --- |
| `POST` | `/library/import/preview` | Analyse le contenu, renvoie les candidats et les avertissements. Rien n'est écrit |
| `POST` | `/library/import` | Crée les modèles retenus. Transaction unique : tout ou rien |

Corps de prévisualisation :

```json
{
  "format": "openapi | curl | postman",
  "content": "…contenu brut du fichier ou de la commande…",
  "baseUrl": "https://api.example.com/v1"
}
```

`baseUrl` est optionnel. Il est appliqué à chaque candidat **sans hôte propre**, avant validation et avant affichage de la prévisualisation :

| URL du candidat | Résultat avec `baseUrl` |
| --- | --- |
| `{{base_url}}/files` (OpenAPI servers) | `https://api.example.com/v1/files`, avertissement `base_url` retiré |
| `/files` (chemin seul) | `https://api.example.com/v1/files` |
| `https://other.io/x` (hôte propre) | inchangée |
| `{{tenant}}/x` (autre variable) | inchangée, la variable reste à fournir dans le workflow |

Un `baseUrl` invalide (pas une URL `http(s)` absolue, plus de 2048 caractères) donne `400`. Sans `baseUrl`, les URL restent telles qu'importées.

Réponse de prévisualisation (forme) :

```json
{
  "items": [
    {
      "key": "op-1",
      "payload": { "name": "Lister les factures", "method": "GET", "url": "{{base_url}}/invoices", "...": "..." },
      "warnings": [
        { "code": "secret_literal", "field": "headers.Authorization", "message": "Token littéral détecté" }
      ]
    }
  ],
  "skipped": [
    { "source": "POST /legacy", "reason": "multipart non supporté" }
  ]
}
```

Confirmation : `POST /library/import` avec `{ "items": [{ "key": "op-1", "name": "…", "payload": { … } }] }`. Le serveur revalide chaque `payload` comme un modèle (pas de confiance dans le client). Réponse `201` : les modèles créés. Les `warnings` (dont `sensitive_header`) sont informatifs et ne bloquent pas l'import. Au plus 50 items par confirmation. Import tout ou rien : une seule transaction, et tout est validé avant la première écriture.

### Interface

- Bouton « Importer » dans la bibliothèque : choix du format, puis dépôt de fichier ou collage de texte (la zone accepte aussi une commande cURL copiée depuis le navigateur ou Postman).
- Écran de prévisualisation : liste des candidats avec cases à cocher, nom éditable, avertissements en évidence, et les `skipped` affichés à part.
- Après confirmation, les modèles apparaissent dans la bibliothèque. Rien n'est ajouté à un workflow.

## Secrets

Un modèle est souvent utilisé pour des calls authentifiés. Risque : un token mis en dur dans un header (`Authorization`, `X-Api-Key`) se retrouve en clair dans la bibliothèque.

**Règle v1 (décidée) :** les valeurs sont stockées telles quelles dans `payload`, sans protection particulière. Les modèles ne sont visibles que par leur propriétaire. Conséquence assumée : un token littéral reste en clair dans la bibliothèque. Les variables de type `secret` restent chiffrées côté workflow, mais pas dans un modèle. À revoir si la bibliothèque devient partageable.

## Modèle de données (proposition)

```sql
CREATE TABLE library_items (
  id          UUID PRIMARY KEY,
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
  payload     JSONB NOT NULL,           -- même forme que CallRequest
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX library_items_user_idx ON library_items (user_id, name);
```

- Propriété : `user_id` obligatoire. Pas d'`org_id` en v1 (pas encore d'organisations). Quand les organisations arriveront, la bibliothèque devra passer par l'organisation, à définir à ce moment-là.
- `payload` en JSONB : la forme évolue avec `CallRequest` sans migration lourde. Validation à l'écriture via le même code que `domain/call`.

## API

| Méthode | Route | Rôle |
| --- | --- | --- |
| `GET` | `/library` | Liste des modèles de l'utilisateur (nom, description, méthode, URL, date) |
| `POST` | `/library` | Crée un modèle à partir d'un payload `CallRequest` |
| `POST` | `/library/from-call/:callId` | Crée un modèle à partir de n'importe quel call de l'utilisateur, quel que soit son workflow (`201`). Un call d'un autre utilisateur donne `404` |
| `GET` | `/library/:id` | Détail d'un modèle |
| `PUT` | `/library/:id` | Modifie un modèle (n'affecte pas les calls insérés) |
| `DELETE` | `/library/:id` | Supprime un modèle (n'affecte pas les calls insérés) |
| `POST` | `/workflows/:id/calls/from-library/:itemId` | Insère un modèle dans le workflow (`201`) |

### Insertion

`POST /workflows/:id/calls/from-library/:itemId` avec un body optionnel :

```json
{ "position": 3 }
```

- Sans `position` : le call est ajouté en fin de workflow (comportement de `AddCall`).
- Avec `position` : le call est inséré à cette place, les calls suivants sont décalés, dans **la même transaction**.
- Réponse : le call créé, au même format que `POST /workflows/:id/calls`.
- Effets : même événement `workflow.updated` que la création manuelle, donc mise à jour temps réel (Centrifugo) sans code spécifique.
- Position invalide (hors de `1..n+1`) : `400`. Corps vide ou `{}` : ajout en fin.
- Les CRUD de la bibliothèque n'émettent aucun événement (pas de temps réel, pas d'outbox) : les modèles ne sont pas synchronisés côté canvas.

**Pourquoi un endpoint dédié plutôt que « GET modèle puis POST call » côté front :** une seule transaction, pas de call créé à moitié si le réordonnancement échoue, et la logique de copie reste côté serveur (le front ne reconstruit pas le payload).

Le réordonnancement est aujourd'hui un endpoint séparé (`PUT /workflows/:id/calls/reorder`). Pour l'insertion au milieu, la version serveur doit reprendre la logique de réordonnancement dans le même `WithTransaction`.

## Interface (canvas)

- Panneau « Bibliothèque » à côté du canvas, liste des modèles avec recherche par nom.
- Glisser un modèle sur le canvas : l'emplacement de dépôt donne la `position`. Dépôt hors zone : fin du workflow.
- Au dépôt : appel de l'endpoint d'insertion, puis le call apparaît comme n'importe quel call créé (pas d'état « modèle »).
- Bouton « Enregistrer dans la bibliothèque » sur un call existant (appelle `POST /library/from-call/:callId`).
- Erreur d'insertion : message affiché, aucun call ajouté au canvas (le canvas se met à jour via l'événement temps réel, pas en optimiste).

## Cas limites

- **Modèle supprimé pendant une insertion :** 404, rien n'est créé.
- **Modèle modifié entre-temps :** l'insertion prend la version courante au moment de la requête.
- **Même modèle inséré plusieurs fois :** plusieurs calls indépendants, avec des noms identiques. Proposer un suffixe (« (2) ») côté front, ou laisser les doublons.
- **Workflow au maximum de calls :** erreur de validation, pas de remplacement automatique.

## Hors périmètre (plus tard)

- Partage entre utilisateurs ou organisations, et bibliothèque publique.
- Versions des modèles et mise à jour « appliquer la nouvelle version » sur les calls insérés.
- Paramètres à remplir à l'insertion (placeholders type `{{baseUrl}}` résolus par l'utilisateur).
- Export des modèles (JSON, ou réutilisation du format YAML de `workflow:import` de la CLI).
- Import par URL (OpenAPI ou Postman hébergé), après contrôle SSRF.

## Décisions v1

0. Propriété : tout est lié à `user_id`, pas d'organisation ni de partage.
1. Variables extraites : hors modèle.
2. Secrets : valeurs stockées telles quelles, pas de protection particulière (voir « Secrets »).
3. Pas de limite de modèles par utilisateur, doublons de nom acceptés.
4. Import : fichier max 2 Mo, `curl -L` ignoré avec avertissement.

## Questions ouvertes

Aucune pour la v1. À reprendre avant la v2 : partage de bibliothèque (impact sur la règle des secrets) et suivi des redirections.
