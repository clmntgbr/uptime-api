# Monitoring de workflows d'API — Documentation technique

## Vision et périmètre

Le produit surveille des enchaînements d'appels API appelés **workflows** : un workflow contient un ou plusieurs calls HTTP exécutés à intervalle régulier, avec des variables passées d'un call au suivant, et toutes les traces sont conservées. Il est volontairement plus simple que FlowForge : pas de conditions ni de boucles, seulement des calls qui s'enchaînent avec un statut par call.

**Phase 1 : uniquement les calls.** Les webhooks, les serveurs et les échéances viendront après (voir la section « Prévu plus tard »).

**Cible** : équipes techniques, petites équipes produit et intégrateurs qui maintiennent des connecteurs (ERP, paiement, transport) chez plusieurs clients.

| Élément | Définition |
| --- | --- |
| Workflow | Liste ordonnée d'un ou plusieurs calls, exécutée à intervalle régulier |
| Call | Requête HTTP, variables à extraire de la réponse, règles d'alerte |
| Run | Une exécution complète d'un workflow, avec le détail de chaque call |
| Incident | Ouvert après des échecs confirmés, fermé après des réussites confirmées |

**Principes de conception**

- **Règles lisibles** : les alertes s'écrivent comme des phrases (« le temps de réponse dépasse 800 ms »).
- **Tout est tracé** : chaque call est stocké avec ses détails (latence, timings, requête, réponse, règles évaluées), pour comprendre après coup pourquoi une alerte a eu lieu.
- **Pas de logique cachée** : un call n'utilise que les variables produites avant lui. Aucun branchement, aucune étape conditionnelle.
- **Frontière nette** : les workers d'exécution (`worker-xxxx`) n'accèdent jamais à Postgres. Ils renvoient un message par Kafka, et le worker principal (`worker`) l'enregistre en base.
- **Léger côté produit** : Go, Kafka, Postgres, pas de navigateur headless.

**Inclus dans la phase 1** : workflows et calls, variables (statiques, de workflow, globales), règles dont le certificat TLS, historique complet, test sans sauvegarde, incidents et notifications (e-mail, Slack, Teams, webhook sortant), authentification Clerk, API REST.

**Exclus de la phase 1** : webhooks, serveurs, échéances, agent installé sur les serveurs, plusieurs régions, tests de navigateur.

**Positionnement** : Better Stack, Checkly, Postman Monitors et Uptime Kuma couvrent chacun une partie. L'offre ici est l'enchaînement simple de calls avec variables, les traces complètes de chaque exécution et l'hébergement en Europe.

## Concepts et modèle

Un **workflow** est une liste ordonnée d'un ou plusieurs calls. Chaque call peut produire des variables que les calls suivants du même workflow réutilisent, et rien d'autre : un call ne voit jamais les variables d'un call placé après lui.

| Concept | Définition |
| --- | --- |
| Workflow | Liste ordonnée de calls, avec un intervalle, des variables statiques et un statut global |
| Call | Requête HTTP (méthode, URL, headers, query, body), variables à extraire de la réponse, règles d'alerte |
| Run | Une passe complète d'un workflow : un enregistrement par call avec tous ses détails |
| Variable | Valeur nommée utilisable dans l'URL, les headers, le body et les valeurs des règles, avec la syntaxe `{{portée.nom}}` |
| Règle | Condition d'alerte lisible (voir la section suivante) |

**Trois portées de variables**

| Portée | Syntaxe | Définie par | Durée de vie |
| --- | --- | --- | --- |
| `global` | `{{global.api_secret}}` | L'organisation, partagée par tous les workflows | Permanente, secrets masqués |
| `static` | `{{static.base_url}}` | Le workflow, saisie manuelle | Permanente, fixe |
| `workflow` | `{{workflow.token}}` | Un call du workflow, extraite de sa réponse | Un seul run, recréée à chaque exécution |

Variables intégrées disponibles partout : `{{timestamp}}`, `{{uuid}}`, `{{run.id}}`.

**Règles de résolution**

- Les calls s'exécutent dans l'ordre. Une variable `workflow.x` n'est utilisable que par les calls situés après celui qui l'extrait.
- À la sauvegarde, on refuse tout workflow qui référence une variable inexistante ou produite plus bas. L'erreur nomme le call et la variable.
- Si une extraction échoue à l'exécution (champ absent, réponse non JSON), la variable n'existe pas : les calls qui en dépendent passent en `skipped` avec la raison, les autres continuent.
- Une valeur secrète (portée `global` marquée secret) est remplacée par `••••` dans les logs, les traces et l'interface.
- Limites du MVP : 20 calls par workflow, 50 variables par workflow, valeur d'une variable limitée à 4 Ko.

**Extraction** : JSONPath sur le corps (`$.access_token`), nom d'en-tête (`Set-Cookie`), code de statut, ou expression régulière sur le corps.

**Exemple de définition**

```yaml
workflow: commandes-api
interval: 5m
static:
  base_url: https://api.exemple.com
calls:
  - name: login
    method: POST
    url: "{{static.base_url}}/auth/token"
    body: '{"secret": "{{global.api_secret}}"}'
    extract:
      token: $.access_token
    rules:
      - status is 200
      - response time exceeds 800 ms
  - name: liste
    method: GET
    url: "{{static.base_url}}/orders"
    headers:
      Authorization: "Bearer {{workflow.token}}"
    extract:
      first_id: $.items[0].id
    rules:
      - status is 200
      - json $.items exists
  - name: detail
    method: GET
    url: "{{static.base_url}}/orders/{{workflow.first_id}}"
    rules:
      - status is 200
      - certificate expires in less than 14 days
```

Dans cet exemple, `detail` utilise `first_id` produit par `liste`, qui utilise `token` produit par `login`. Inverser l'ordre de `login` et `liste` est refusé à la sauvegarde. Un workflow peut aussi ne contenir qu'un seul call.

## Règles d'alerte

Une règle est une phrase : **\[sujet\] \[opérateur\] \[valeur\]**. On en empile autant qu'on veut sur un call, et une seule règle en échec met le call en `failed`. Il n'y a ni ET/OU imbriqués, ni scripts : c'est ce qui garde le tout lisible.

**Exemples** : « le code HTTP n'est pas 2xx », « le temps de réponse dépasse 800 ms », « le corps ne contient pas `ok` », « `$.status` n'est pas `up` », « le certificat expire dans moins de 14 jours ».

**Sujets et opérateurs**

| Sujet | Opérateurs | Valeur |
| --- | --- | --- |
| Code HTTP | est, n'est pas, est dans | Un code (`200`) ou une classe (`2xx`) |
| Temps de réponse | dépasse | Durée en ms ou s |
| Corps | contient, ne contient pas | Texte ou variable |
| Champ JSON | est, n'est pas, existe, n'existe pas, dépasse, est sous | Chemin JSONPath + valeur |
| En-tête | est, contient, existe | Nom + valeur |
| Taille de la réponse | dépasse | Octets, Ko ou Mo |
| Certificat TLS | expire dans moins de, chaîne invalide, nom non correspondant | Nombre de jours |

Les valeurs peuvent contenir des variables : `le champ $.user.id est {{workflow.user_id}}`.

**Règle par défaut** : sans règle explicite, un call est en échec si le code n'est pas 2xx ou s'il y a une erreur réseau (DNS, connexion, TLS, timeout).

**Certificats sur les calls** : le worker-calls lit la chaîne de certificats pendant la connexion TLS de chaque call, sans requête supplémentaire. Il stocke l'émetteur, la date d'expiration et la validité de la chaîne, et vérifie aussi les certificats intermédiaires. Comme un call tourne souvent, la règle fonctionne par paliers (30, 14, 7 et 1 jours par défaut) : l'incident s'ouvre au premier palier franchi, un rappel part à chaque palier suivant, et il se ferme quand le certificat est renouvelé. La vérification TLS peut être désactivée pour un call (certificat auto-signé), le certificat est alors lu et signalé quand même.

**Évaluation**

1. Le `worker-calls` exécute le call et capture la réponse.
2. Toutes les règles sont évaluées, sans court-circuit, pour que l'historique montre l'ensemble des écarts.
3. Chaque règle produit un résultat stocké : phrase, valeur observée, valeur attendue, réussite ou échec. Exemple : « temps de réponse dépasse 800 ms : observé 1 240 ms, échec ».
4. Le statut du call est `success` si tout passe, `failed` si au moins une règle échoue, `error` si le call n'a pas pu s'exécuter, `skipped` si une variable manque.

**Interface** : un bouton « Tester maintenant » exécute le workflow, affiche chaque règle en vert ou rouge avec la valeur observée, et ne laisse qu'une trace courte (voir la section Exécution). Les règles se construisent avec trois listes déroulantes (sujet, opérateur, valeur) plus un bouton « Ajouter une règle », avec la phrase complète affichée en aperçu.

**Réglages séparés des règles** : le nombre d'échecs consécutifs avant d'ouvrir un incident et la durée de récupération sont des réglages du workflow, pas des règles (voir la section Alertes et incidents).

## Exécution des workflows

Un workflow s'exécute en séquence dans un `worker-calls`. Le `scheduler` prépare un job complet, le worker enchaîne les calls sans jamais toucher la base, puis publie un message de résultat que le `worker` enregistre.

**Cycle d'un run**

1. Le `scheduler` repère les workflows dont `next_run_at` est dépassé et qui ne tournent pas déjà. Il pose un bail (`running`, avec une échéance) en base, construit un job autonome (définition du workflow, variables statiques et globales) et le publie sur le topic Kafka `workflow.jobs`, avec la clé `workflow_id` et un léger décalage aléatoire pour éviter les pics.
2. Un `worker-calls` consomme le job et reprend le `run_id` fourni dans le message.
3. Pour chaque call, dans l'ordre : résolution des variables, requête HTTP, capture de la réponse, extraction des variables, évaluation des règles.
4. Le worker publie un seul message sur `workflow.results` avec tous les détails, puis valide son offset. Il n'écrit rien d'autre.
5. Le `worker` consomme le résultat, l'écrit dans une transaction Postgres, met à jour le statut du workflow, libère le bail, évalue les incidents, prévient Centrifugo et publie les événements.

Un workflow ne s'exécute jamais deux fois en parallèle : si le bail est encore valide, la nouvelle passe est enregistrée en `skipped` avec la raison `overlap`. Si un `worker-calls` plante avant d'avoir publié son résultat, le job est relivré par Kafka ; à défaut, le bail expire et le run est marqué `error` (raison `timeout`) avant la passe suivante.

**Job autonome** : le message du `scheduler` contient tout ce dont le worker a besoin (calls, règles, variables). Les valeurs secrètes sont chiffrées pour la clé des `worker-calls` (chiffrement à enveloppe, clé distincte de celle de la base), car Kafka conserve les messages sur disque : aucune valeur secrète n'y figure en clair, et seul le worker les déchiffre, en mémoire, le temps de l'exécution.

**Traitement des jobs par Kafka** : une partition est lue dans l'ordre, mais les jobs de workflows différents sont indépendants. Chaque `worker-calls` traite donc jusqu'à 50 jobs en parallèle et ne valide ses offsets que jusqu'au plus ancien job terminé. La livraison est au moins une fois : un run peut être rejoué après une panne, ce que l'identifiant de run (`run_id`) rend sans danger côté base.

**Statuts**

| Niveau | Statuts | Sens |
| --- | --- | --- |
| Call | `success`, `failed`, `error`, `skipped` | Règles toutes passées, au moins une en échec, call non exécutable (réseau, timeout), variable manquante |
| Workflow (run) | `success`, `failed` | `success` si tous les calls sont en `success`, sinon `failed` avec le nombre de calls par statut |

**Informations stockées pour chaque call**

| Catégorie | Contenu |
| --- | --- |
| Requête | Méthode, URL finale (variables résolues), headers (secrets masqués), query, body (tronqué), taille |
| Réponse | Code, headers, body (tronqué), taille, protocole (HTTP/1.1, HTTP/2), compression |
| Timings | DNS, connexion TCP, négociation TLS, temps jusqu'au premier octet, transfert, total, mesurés avec `httptrace` en Go |
| Réseau | IP résolue, version IP, redirections suivies |
| TLS | Version, suite de chiffrement, émetteur, chaîne, date d'expiration du certificat |
| Règles | Phrase, valeur observée, valeur attendue, résultat |
| Variables | Variables extraites (secrets masqués) et variables utilisées |
| Erreur | Type (dns, connect, tls, timeout, body\_too\_large) et message |
| Contexte | Run, workflow, ordre dans le workflow, horodatage de début et de fin |

**Paramètres**

- Timeout par défaut de 30 s par call (maximum configurable de 60 s) et de 5 minutes par workflow. Redirections suivies jusqu'à 5, cookies conservés entre les calls d'un même run.
- Corps limité : 256 Ko stockés en base, au-delà le corps est tronqué et le début conservé dans le stockage objet jusqu'à 5 Mo. Un indicateur `truncated` signale la coupe.
- Version d'IP au choix (IPv4, IPv6 ou les deux), vérification TLS activable ou non, proxy optionnel.
- Intervalle minimal : 1 minute (offre payante), 5 minutes (offre gratuite).

**Test sans sauvegarde** : « Tester maintenant » publie un job avec le déclencheur `test`. Il suit le même chemin que les runs planifiés, avec le même résultat enregistré, mais conservé 24 heures seulement et sans effet sur les incidents. L'`api` répond tout de suite avec le `run_id` ; le front reçoit le résultat par Centrifugo.

**Régions** : une région européenne au MVP. Plusieurs régions viendront ensuite : le workflow s'exécute alors dans chaque région choisie, et une panne n'est confirmée que si plusieurs régions la voient.

## Alertes et incidents

Les règles décident si un résultat est bon ou mauvais ; les incidents décident quand prévenir quelqu'un. Le `worker` gère ce cycle de façon événementielle : à chaque résultat enregistré, il met à jour l'état et ouvre ou ferme l'incident. Un incident s'ouvre après plusieurs échecs confirmés et se ferme après plusieurs réussites, pour éviter les alertes sur un échec isolé.

**Cycle d'un incident**

1. Un run passe en `failed` : le `worker` l'enregistre, puis évalue l'état du workflow.
2. La confirmation se déclenche : le `worker` place le workflow en état `confirming` et avance son prochain passage de quelques secondes ; le `scheduler` le relance jusqu'à atteindre le nombre d'échecs consécutifs demandé (3 par défaut).
3. Le `worker` ouvre l'incident, avec la règle en échec, la valeur observée et le lien vers la trace complète.
4. Il publie une demande de notification sur `notifications.requests` : un `worker-notify` envoie les messages sur les canaux choisis. Un rappel est renvoyé à intervalle réglable tant que l'incident reste ouvert.
5. L'incident se ferme après le nombre de réussites consécutives configuré (2 par défaut), et une notification de rétablissement part.

Les alertes de certificat suivent leurs propres paliers (voir la section Règles d'alerte) mais passent par le même circuit d'incident et de notification.

**Réglages par workflow**

| Réglage | Défaut | Rôle |
| --- | --- | --- |
| Échecs avant alerte | 3 | Confirmation avant d'ouvrir l'incident |
| Réussites avant résolution | 2 | Confirmation du rétablissement |
| Rappel | 30 min | Intervalle entre deux relances tant que l'incident dure |
| Fenêtre de maintenance | Aucune | Plages horaires récurrentes sans alerte, avec fuseau |
| Canaux | E-mail de l'équipe | Où envoyer les notifications |

**Canaux de notification**

| Canal | Phase 1 |
| --- | --- |
| E-mail | Oui |
| Webhook sortant (JSON signé) | Oui |
| Slack et Microsoft Teams | Oui |
| SMS et appel téléphonique | Plus tard (via un fournisseur de télécoms) |
| Astreinte et escalade | Plus tard |

**Contenu d'une notification** : nom du workflow et du call, phrase de la règle en échec, valeur observée contre valeur attendue, heure de début, lien vers le détail. Aucun corps de réponse ni secret n'est inclus.

**Actions sur un incident** : prise en compte (l'incident reste ouvert mais les rappels s'arrêtent), mise en sourdine pour une durée, résolution manuelle, commentaire interne.

**Historique** : chaque incident garde sa chronologie (ouverture, notifications envoyées, prise en compte, résolution) et reste consultable avec les traces des runs concernés.

## Containers

La phase 1 tient en 9 containers sur 4 réseaux Docker. Règle de base : les workers d'exécution (`worker-xxxx`) n'accèdent jamais à Postgres. Seuls `api`, `scheduler` et `worker` vivent dans le réseau `data` ; tous les autres communiquent uniquement par Kafka.

| Container | Image | Rôle | Réseau | Scalable |
| --- | --- | --- | --- | --- |
| `caddy` | Caddy | Reverse proxy, TLS automatique, route `/api` et `/connection/websocket` | `edge` | Non |
| `api` | Go (Fiber) | API REST, validation des jetons Clerk, configuration des workflows, lecture des résultats, jetons de connexion Centrifugo, publication des jobs de test | `edge`, `backend`, `data` | Oui |
| `scheduler` | Go | Planifie les workflows (bail en base), relance les confirmations, purge et agrège | `backend`, `data` | Une instance active |
| `worker` | Go | Worker principal événementiel : consomme les résultats des workers, les écrit en base, ouvre et ferme les incidents, prévient Centrifugo, demande les notifications | `backend`, `data` | Oui |
| `worker-calls` | Go | Exécute les workflows : requêtes HTTP, extraction des variables, évaluation des règles | `egress` | Oui |
| `worker-notify` | Go | Envoie les notifications (e-mail, Slack, Teams, webhook sortant) | `egress` | Oui |
| `kafka` | Apache Kafka (KRaft) | Messaging interne | `backend`, `egress` | Non au départ (3 brokers en production) |
| `centrifugo` | Centrifugo | Notifications temps réel vers le front | `edge`, `backend` | Non |
| `postgres` | PostgreSQL | Source de vérité | `data` | Non |

Un stockage objet compatible S3, hébergé dans l'UE, reçoit les gros corps de réponse. Il est externe au compose.

**Réseaux**

- `edge` : exposé à Internet, uniquement `caddy`, `api` et `centrifugo`. `api` y sort aussi vers Clerk pour récupérer les clés de validation des jetons.
- `backend` : services internes et Kafka, sans sortie vers Internet.
- `data` : `postgres` et les trois seuls services autorisés à lui parler (`api`, `scheduler`, `worker`), sans sortie vers Internet.
- `egress` : `worker-calls` et `worker-notify`, qui contactent les URLs des clients et les services de notification, avec sortie Internet filtrée (plages privées bloquées) et accès uniquement à Kafka.

**Extrait de `docker-compose.yml`**

```yaml
services:
  caddy:
    image: caddy:2
    ports: ["80:80", "443:443"]
    networks: [edge]

  api:
    build: ./services/api
    environment:
      DATABASE_URL: postgres://api@postgres:5432/monitor
      KAFKA_BROKERS: kafka:9092
      CLERK_JWKS_URL: https://<domaine-clerk>/.well-known/jwks.json
    networks: [edge, backend, data]
    depends_on: [postgres, kafka]

  scheduler:
    build: ./services/scheduler
    environment:
      DATABASE_URL: postgres://scheduler@postgres:5432/monitor
      KAFKA_BROKERS: kafka:9092
    networks: [backend, data]

  worker:
    build: ./services/worker
    environment:
      DATABASE_URL: postgres://worker@postgres:5432/monitor
      KAFKA_BROKERS: kafka:9092
      CENTRIFUGO_API_URL: http://centrifugo:8000/api
    networks: [backend, data]
    deploy:
      replicas: 2

  worker-calls:
    build: ./services/worker-calls
    environment:
      KAFKA_BROKERS: kafka:9092
    read_only: true
    cap_drop: [ALL]
    user: "10001"
    mem_limit: 512m
    networks: [egress]
    deploy:
      replicas: 2

  worker-notify:
    build: ./services/worker-notify
    environment:
      KAFKA_BROKERS: kafka:9092
    read_only: true
    cap_drop: [ALL]
    user: "10001"
    networks: [egress]

  kafka:
    image: apache/kafka:3.9.0
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: INTERNAL://:9092,CONTROLLER://:9093
      KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka:9092
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_AUTHORIZER_CLASS_NAME: org.apache.kafka.metadata.authorizer.StandardAuthorizer
    volumes: [kafkadata:/var/lib/kafka/data]
    networks: [backend, egress]

  centrifugo:
    image: centrifugo/centrifugo:v6
    networks: [edge, backend]

  postgres:
    image: postgres:17
    volumes: [pgdata:/var/lib/postgresql/data]
    networks: [data]

networks:
  edge:
  backend:
    internal: true
  data:
    internal: true
  egress:

volumes:
  kafkadata:
  pgdata:
```

Les `worker-xxxx` n'ont aucun accès à Postgres : ils publient leurs résultats sur Kafka et le `worker` les écrit en base. Chaque service a son propre rôle Postgres aux droits limités et son propre compte Kafka avec des ACL (voir la section Sécurité). Les versions d'images sont à figer au moment du déploiement.

## Messaging : Kafka

Kafka est la seule messagerie interne. Les jobs, les résultats des workers et les demandes de notification y circulent, et rien d'autre ne relie les services qui sortent sur Internet à la base de données.

**Ce que Kafka impose** : un topic est un journal découpé en partitions ordonnées. Les consumers d'un même groupe se partagent les partitions et valident (commit) la position atteinte, sans acquitter message par message. Cinq conséquences guident la conception :

1. **Un job lent bloque ceux qui le suivent dans sa partition.** D'où le traitement parallèle avec validation des offsets au plus ancien job terminé (voir Exécution).
2. **Pas de retry avec délai natif.** On passe par un topic de retry.
3. **Pas de dead letter natif.** On passe par un topic `.dlq`.
4. **L'ordre n'est garanti qu'à l'intérieur d'une partition.** Les messages d'un même workflow portent la clé `workflow_id` et restent donc ordonnés.
5. **Le nombre de partitions plafonne le parallélisme** : un groupe ne peut pas avoir plus de consumers actifs que de partitions. On démarre à 12 partitions pour les topics de workflows.

**Pourquoi un `worker` qui persiste**

| Option | Verdict |
| --- | --- |
| Les `worker-xxxx` écrivent directement en base | Rejetée : un worker qui appelle des URLs arbitraires deviendrait un point d'entrée vers Postgres |
| Les `worker-xxxx` appellent l'`api` en HTTP ou gRPC | Rejetée : couplage synchrone, résultats perdus si l'`api` est indisponible |
| `worker-xxxx` publient sur Kafka, le `worker` écrit en base | Retenue : découplage, résultats rejouables, frontière de sécurité nette |
| Outbox transactionnelle et CDC (Debezium) | Inutile ici : utile pour publier depuis la base, pas pour la remplir |

**Topics**

| Topic | Clé | Producteur | Consumer group | Contenu | Rétention |
| --- | --- | --- | --- | --- | --- |
| `workflow.jobs` | `workflow_id` | `scheduler`, `api` (tests) | `worker-calls` | Job autonome d'un run | 1 jour |
| `workflow.results` | `workflow_id` | `worker-calls` | `worker` | Résultat complet d'un run | 7 jours |
| `notifications.requests` | `org_id` | `worker` | `worker-notify` | Demande d'envoi sur un canal | 3 jours |
| `notifications.results` | `org_id` | `worker-notify` | `worker` | Statut d'envoi | 3 jours |
| `events` | `workflow_id` | `worker` | Aucun au MVP | `result.recorded`, `incident.opened`, `incident.resolved` | 7 jours |
| `*.retry` et `*.dlq` | Identique au topic d'origine | Le service qui échoue | Le même groupe (retry), l'équipe (dlq) | Messages à rejouer ou à examiner | 30 jours pour les dlq |

Les topics de workflows ont 12 partitions, les autres 6. Les topics sont créés par l'infrastructure (la création automatique est désactivée), avec leurs rétentions.

**Règles de production et de consommation**

- Producteurs idempotents (`enable.idempotence`) avec `acks=all`.
- Livraison au moins une fois : un offset n'est validé qu'après le traitement complet, et les écritures du `worker` sont idempotentes (clé unique sur `run_id` ou sur l'identifiant de l'événement).
- **Transaction unique dans le `worker`** : résultat, état du workflow et état d'incident sont écrits dans la même transaction Postgres, puis l'offset est validé. Les demandes de notification sont d'abord inscrites dans la table `notifications` avec le statut `pending`, puis publiées ; si le `worker` plante entre les deux, le message est relu, les écritures sont ignorées (idempotence) et les `pending` sont republiées.
- **Retries** : une erreur transitoire (base indisponible, par exemple) est retentée 3 fois sur place avec un délai croissant, puis le message part sur `*.retry` avec un en-tête `attempt` et une heure `not_before` ; le consumer de retry met la partition en pause jusqu'à cette heure. Après 5 tentatives, ou immédiatement pour un message invalide (poison), il part en `*.dlq` avec une alerte interne.
- **Rééquilibrages** : adhésion statique (`group.instance.id`) et assignation coopérative, pour qu'un déploiement ne provoque pas de tempête de rééquilibrages. `max.poll.interval.ms` est fixé à 10 minutes, au-dessus de la durée maximale d'un workflow (5 minutes).
- **Taille** : `max.message.bytes` à 4 Mo. Au-delà de 256 Ko, un corps est écrit dans le stockage objet et le message ne porte que sa référence (`body_ref`).
- **Schémas** : JSON versionné (champ `v`) au MVP, avec un contrat par topic. Protobuf et un registre de schémas pourront suivre.
- **Client Go** : franz-go, qui gère la pause des partitions, la validation manuelle des offsets et l'assignation coopérative.

**Enveloppe d'un résultat** (`workflow.results`)

```json
{
  "v": 1,
  "id": "01J9X...",
  "type": "workflow.result",
  "org_id": "...",
  "workflow_id": "...",
  "run_id": "...",
  "trigger": "schedule",
  "status": "failed",
  "calls": [
    {
      "call_id": "...",
      "order": 1,
      "status": "failed",
      "http_status": 200,
      "timings_ms": {"dns": 12, "connect": 30, "tls": 41, "ttfb": 980, "total": 1240},
      "tls": {"expires_at": "2026-12-01T00:00:00Z", "issuer": "..."},
      "rules": [
        {"text": "temps de réponse dépasse 800 ms", "observed": "1240 ms", "passed": false}
      ],
      "body_ref": "s3://.../run/call/body"
    }
  ],
  "emitted_at": "2026-10-13T16:30:00Z"
}
```

Les notifications envoyées à Centrifugo ne portent jamais de corps, de headers ni de variables : seulement des identifiants, un statut et un compteur.

**Exploitation** : KRaft sans ZooKeeper. Un seul broker avec réplication à 1 au départ, suffisant pour développer et pour une bêta où les résultats sont écrits en base en quelques secondes. En production : 3 brokers, réplication à 3, `min.insync.replicas` à 2. Le retard des consumers (consumer lag) de chaque groupe est surveillé et alerté.

**Évolution** : un consommateur d'analytique (ClickHouse, entrepôt de données) peut se brancher sur `workflow.results` et `events` sans toucher aux workers. Si la charge d'exploitation de Kafka pèse trop, Redpanda reprend la même API et le même client.

## Modèle de données Postgres

Postgres est la source de vérité, accessible uniquement à `api`, `scheduler` et `worker`. Chaque table métier porte un `org_id` (multi-tenant) et toute requête de l'`api` est filtrée par organisation. Les tables de résultats, qui grossissent vite, sont partitionnées par date et écrites uniquement par le `worker`.

**Configuration**

| Table | Contenu | Points d'attention |
| --- | --- | --- |
| `organizations`, `users` | Copie locale des organisations et utilisateurs Clerk : identifiants, plan, quotas, rôle | L'authentification est déléguée à Clerk, synchronisée par ses webhooks |
| `api_keys` | Clés d'API par organisation : préfixe, hash, portée, dernière utilisation | Jamais stockées en clair, révocables |
| `global_variables` | Variables globales de l'organisation | Valeur chiffrée si `secret` |
| `workflows` | Nom, intervalle, variables statiques, réglages d'incident, canaux, activation | Écrite par l'`api` |
| `calls` | Position dans le workflow, méthode, URL, headers, query, body, extractions, timeout | Validation de l'ordre des variables à l'écriture |
| `rules` | Règles d'alerte d'un call : sujet, opérateur, valeur, paliers | Valeurs pouvant contenir des variables |
| `notification_channels` | E-mail, Slack, Teams, webhook sortant | Configuration chiffrée |

**État courant**

| Table | Contenu | Points d'attention |
| --- | --- | --- |
| `workflow_state` | Statut, `next_run_at`, bail (`lease_until`), échecs et réussites consécutifs, état `confirming` | Lue par le `scheduler`, écrite par le `scheduler` et le `worker` |
| `certificates` | Certificats vus par les calls : hôte, émetteur, expiration, dernière observation, palier déjà notifié | Base d'un futur inventaire des échéances |

**Résultats (partitionnés)**

| Table | Contenu | Points d'attention |
| --- | --- | --- |
| `runs` | Une passe de workflow : statut, début, fin, déclencheur (planifié, manuel, test), variables non secrètes | Partition mensuelle, clé unique sur `run_id` |
| `call_results` | Un call : requête, réponse, timings, réseau, TLS, erreur, variables extraites | Corps tronqué, `body_ref` au-delà de 256 Ko |
| `rule_results` | Phrase de la règle, valeur observée, valeur attendue, réussite | Lié au résultat qui l'a produit |

**Incidents**

| Table | Contenu |
| --- | --- |
| `incidents` | Workflow et call concernés, règle en échec, valeur observée, ouverture, résolution, état de prise en compte |
| `incident_events` | Chronologie : ouverture, notifications, prise en compte, rappels, résolution |
| `notifications` | Canal, statut (`pending`, `sent`, `failed`), tentative, heure d'envoi, clé d'idempotence |
| `audit_log` | Qui a modifié quoi, non modifiable, conservé 12 mois |

**Volumétrie (hypothèse)** : 100 workflows de 3 calls exécutés chaque minute produisent 432 000 `call_results` par jour. À environ 5 Ko par résultat (métadonnées et corps compressé), cela fait 2,2 Go par jour, soit 65 Go sur 30 jours.

**Rétention par offre**

| Offre | Métadonnées et règles | Corps de réponse |
| --- | --- | --- |
| Gratuite | 7 jours | 24 heures |
| Pro | 30 jours | 7 jours |
| Équipe | 90 jours | 30 jours |

Les runs de test sont supprimés après 24 heures quelle que soit l'offre. La suppression se fait en détachant ou supprimant les partitions échues, ce qui est instantané et évite les gros `DELETE`. Les statuts agrégés (disponibilité par jour, latence médiane et p95 par heure) sont calculés à part par le `scheduler` et conservés plus longtemps, pour tracer des courbes sur des mois sans garder les détails.

**Si la base devient lourde** : TimescaleDB (compression et agrégats continus) reste compatible avec Postgres et suffit largement avant d'envisager ClickHouse.

## API REST et temps réel

L'`api` expose une API JSON versionnée (`/v1`) utilisée par le front et par les clients qui veulent automatiser leur configuration. Centrifugo ne sert qu'à prévenir le front qu'il y a du nouveau : les données se relisent toujours par l'API.

**Authentification** : Clerk gère les comptes, les sessions et les organisations. Le front envoie le jeton de session Clerk à l'`api`, qui vérifie sa signature avec les clés publiques de Clerk, puis lit l'utilisateur, l'organisation active et son rôle (propriétaire, éditeur, lecteur). Les organisations Clerk correspondent aux organisations du produit (`org_id`), et leurs créations, modifications et suppressions sont synchronisées par les webhooks de Clerk reçus sur `POST /v1/webhooks/clerk` (signature vérifiée). Pour l'automatisation, des clés d'API par organisation, hachées en base, révocables, en lecture seule ou en lecture et écriture. Pagination par curseur, filtres par période et statut.

**Endpoints principaux**

| Ressource | Endpoints |
| --- | --- |
| Workflows et calls | `GET/POST /v1/workflows`, `GET/PUT/DELETE /v1/workflows/{id}`, `POST /v1/workflows/{id}/run` (exécution manuelle), `POST /v1/workflows/test` (test sans sauvegarde, répond `202` avec le `run_id`) |
| Exécutions | `GET /v1/workflows/{id}/runs`, `GET /v1/runs/{id}` (détail de chaque call avec requête, réponse, timings et règles) |
| Variables | `GET/POST/PUT/DELETE /v1/variables` (globales) |
| Incidents | `GET /v1/incidents`, `POST /v1/incidents/{id}/ack`, `POST /v1/incidents/{id}/resolve` |
| Certificats | `GET /v1/certificates` (certificats vus par les calls, triés par expiration) |
| Canaux | `GET/POST /v1/channels`, `POST /v1/channels/{id}/test` |
| Clés d'API | `GET/POST/DELETE /v1/api-keys` |
| Clerk | `POST /v1/webhooks/clerk` (synchronisation des utilisateurs et organisations) |

**Temps réel avec Centrifugo**

1. Le `worker` enregistre un résultat, un incident ou un statut de test en base.
2. Après le commit, il envoie une notification légère à Centrifugo par son API serveur (HTTP). Si Centrifugo est indisponible, l'envoi est abandonné sans bloquer l'écriture.
3. Le front reçoit la notification et relit l'objet concerné par l'API.

| Canal | Événements | Contenu du message |
| --- | --- | --- |
| `org:{org_id}` | `run.completed`, `test.completed` | Identifiants du workflow et du run, statut, compteurs |
| `org:{org_id}` | `incident.opened`, `incident.resolved` | Identifiant de l'incident, workflow, call |

**Règles**

- Un canal par organisation. Le jeton de connexion Centrifugo est signé par l'`api` après validation du jeton Clerk, et n'autorise que les canaux de l'organisation de l'utilisateur.
- Aucun corps, header ni variable ne transite par Centrifugo.
- À la connexion ou après une reconnexion, le front relit l'état par l'API. L'historique de canal Centrifugo n'est qu'un confort.
- Le front déduplique par identifiant d'événement, car Kafka livre au moins une fois.
- Le résultat d'un test arrive par `test.completed` ; le front relit alors le run par `GET /v1/runs/{id}`.

## Sécurité de la plateforme

Le risque principal est que les clients choisissent les URLs que la plateforme appelle : sans précaution, un client pourrait faire contacter le réseau interne ou les services cloud par les workers (SSRF). La seconde exposition est la quantité de données sensibles stockées : tokens, corps de réponse et variables.

**Frontière de sécurité** : aucun worker d'exécution (`worker-calls`, `worker-notify`) n'accède à Postgres. Seuls `api`, `scheduler` et `worker` vivent dans le réseau `data`, chacun avec son propre rôle Postgres aux droits limités. Un worker compromis ne peut donc ni lire ni modifier les données des clients : il ne peut que publier des messages sur ses propres topics Kafka.

**Protection contre le SSRF (`worker-calls` et `worker-notify`)**

- Seuls les schémas `http` et `https` sont acceptés.
- Le worker résout le nom une fois, vérifie que l'IP n'est pas dans une plage interdite, puis se connecte directement à cette IP. Il refait la vérification après chaque redirection, ce qui bloque le DNS rebinding.
- Plages bloquées : `127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16` (dont la metadata cloud), `100.64.0.0/10`, IPv6 locales et mappées.
- Le réseau `egress` n'a de route que vers Kafka et Internet, pas vers Postgres, Centrifugo ni l'`api`.
- Limites par requête : timeout, nombre de redirections, taille de corps lue (arrêt au-delà), débit par organisation.
- User-Agent identifiable et IP de sortie fixes et documentées, pour que les clients autorisent le monitoring dans leur pare-feu.

**Kafka : authentification et ACL**

Chaque service se connecte avec son propre compte (SASL/SCRAM) sur une liaison TLS, et n'a de droits que sur ses topics.

| Service | Lecture | Écriture |
| --- | --- | --- |
| `scheduler` | Aucune | `workflow.jobs` |
| `api` | Aucune | `workflow.jobs` (runs de test uniquement) |
| `worker-calls` | `workflow.jobs`, `workflow.jobs.retry` | `workflow.results`, `workflow.jobs.retry`, `workflow.jobs.dlq` |
| `worker` | `workflow.results`, `notifications.results`, leurs `.retry` | `notifications.requests`, `events`, `.retry` et `.dlq` des topics qu'il lit |
| `worker-notify` | `notifications.requests` | `notifications.results` |

**Secrets et données**

- Variables `secret` et configurations de canaux chiffrées au repos (AES-GCM, clé hors base). Les valeurs secrètes sont masquées dans les logs, les traces, les notifications et l'interface, y compris dans les headers `Authorization`, `Cookie` et les champs ressemblant à des secrets.
- Les secrets transportés dans les jobs sont chiffrés pour la clé des `worker-calls` : Kafka stocke les messages sur disque, donc aucune valeur secrète n'y figure en clair. Les disques des brokers sont chiffrés et la rétention de `workflow.jobs` est limitée à un jour.
- Les corps de réponse sont par nature sensibles : stockage chiffré, rétention courte selon l'offre, suppression sur demande.
- Clés d'API affichées une seule fois à la création, puis stockées hachées.

**Isolation des clients et accès**

- Toutes les requêtes de l'`api` sont filtrées par `org_id`, avec des tests automatiques qui vérifient qu'un utilisateur ne peut lire aucune donnée d'une autre organisation.
- Rôles (propriétaire, éditeur, lecteur) portés par Clerk, et clés d'API à portée limitée.
- Journal d'audit des modifications de configuration et des accès aux corps de réponse.
- Authentification forte (TOTP) proposée par Clerk pour tous les comptes.

**RGPD** : les corps de réponse peuvent contenir des données personnelles. Le client est responsable de traitement, la plateforme est sous-traitante (accord de traitement, hébergement dans l'UE, rétention documentée, suppression sur demande). Clerk devient un sous-traitant supplémentaire pour les identités des utilisateurs : vérifier la localisation des données et l'accord de traitement. Cette position est à faire valider par un juriste avant l'ouverture commerciale.

**Conteneurs** : utilisateur non root, système de fichiers en lecture seule, capacités Linux retirées, limites de CPU et de mémoire, images minimales figées.

## Multi-tenant, plans et quotas

L'unité de facturation et d'isolation est l'organisation. Trois offres, avec des quotas qui suivent ce qui coûte réellement : fréquence d'exécution, nombre de calls et durée de conservation des traces.

**Offres envisagées** (prix et quotas à valider avec les premiers testeurs)

|  | Gratuite | Pro | Équipe |
| --- | --- | --- | --- |
| Prix indicatif | 0 € | 19 €/mois | 79 €/mois |
| Workflows | 3 | 25 | 100 |
| Calls par workflow | 5 | 20 | 20 |
| Intervalle minimal | 5 min | 1 min | 1 min |
| Utilisateurs | 1 | 5 | 20 |
| Rétention des métadonnées | 7 jours | 30 jours | 90 jours |
| Rétention des corps | 24 h | 7 jours | 30 jours |
| Canaux | E-mail | E-mail, Slack, Teams, webhook | Tous |
| Plusieurs régions | Non | Non | Oui (quand disponible) |

**Application des quotas**

- L'`api` refuse la création au-delà du quota de workflows ou de calls, et un intervalle inférieur au minimum de l'offre.
- Le `scheduler` ne planifie pas les workflows au-delà du quota en cas de changement d'offre (ils sont suspendus, jamais supprimés).
- Chaque limite atteinte déclenche un e-mail d'avertissement à 80 % et à 100 %.

**Facturation** : abonnement mensuel ou annuel via un prestataire de paiement (Stripe), changement d'offre immédiat, facture européenne avec TVA. Pas de facturation à l'usage au MVP.

**Organisations et équipes** : un utilisateur peut appartenir à plusieurs organisations, grâce aux organisations Clerk. Les rôles sont propriétaire (facturation et suppression), éditeur (configuration) et lecteur (consultation et prise en compte des incidents).

**Coût d'infrastructure à surveiller** : la rétention des corps de réponse est le premier poste de stockage, et la fréquence d'exécution le premier poste de calcul. Les quotas de l'offre gratuite doivent rester assez bas pour que son coût par organisation soit négligeable. Clerk facture par utilisateur actif : ce coût entre dans le calcul de la marge de chaque offre.

## Observabilité et déploiement

Un outil de monitoring qui tombe sans prévenir détruit la confiance : la plateforme se surveille elle-même, et un contrôle externe vérifie qu'elle est vivante.

**Surveillance de la plateforme**

| Signal | Pourquoi | Seuil d'alerte indicatif |
| --- | --- | --- |
| Retard du `scheduler` (écart entre `next_run_at` et la publication du job) | Les workflows ne partent plus à l'heure | Plus de 30 s |
| Consumer lag du groupe `worker-calls` sur `workflow.jobs` | Workers saturés ou à l'arrêt | Croissance continue pendant 5 min |
| Consumer lag du groupe `worker` sur `workflow.results` | Résultats écrits trop tard, incidents en retard | Plus de 60 s de retard |
| Messages dans les topics `.dlq` | Messages que personne ne sait traiter | Supérieur à 0 |
| Taux d'échec des notifications (`notifications.results`) | Les clients ne sont pas prévenus | Supérieur à 2 % |
| Durée des runs par rapport au timeout | Workflows proches de leur limite | Plus de 80 % du timeout |
| Espace disque Kafka et Postgres | Saturation par les messages et les corps stockés | 80 % |
| Partitions sans consumer actif | Un groupe est sous-dimensionné ou en panne | Supérieur à 0 |

**Outils** : logs structurés en JSON (`slog`), métriques Prometheus exposées par chaque service (et par un exporter Kafka pour le lag), traces OpenTelemetry sur le chemin d'un run (job, exécution, résultat, écriture), tableaux de bord Grafana. Aucun corps de requête ni secret dans les logs.

**Qui surveille le surveilleur** : le `scheduler` émet un heartbeat sortant vers un service tiers (un autre fournisseur, ou une petite instance séparée) toutes les minutes. S'il cesse, l'équipe est prévenue par un canal indépendant de la plateforme.

**Déploiement**

| Sujet | Choix |
| --- | --- |
| Hébergement | Un ou deux VPS en Europe avec Docker Compose au départ, Kubernetes seulement si un besoin de mise à l'échelle l'impose |
| CI/CD | GitLab CI : tests, lint, scan des images et des dépendances, build, déploiement par étapes |
| Migrations | Outil de migrations versionnées exécuté avant le démarrage de l'`api`, toujours compatibles avec la version précédente du code |
| Topics Kafka | Créés par l'infrastructure (scripts versionnés), avec partitions, rétentions et ACL |
| Environnements | Développement (compose local), préproduction, production |
| Sauvegardes | Archivage continu des journaux Postgres et sauvegardes complètes quotidiennes, restauration testée chaque mois |
| Configuration | Variables d'environnement et secrets montés, jamais dans l'image |
| Disponibilité | `api` et `worker` en au moins 2 instances, Kafka à 3 brokers en production |
| Mises à jour | Redéploiement progressif des consumers avec adhésion statique, pour éviter les rééquilibrages en cascade |

**Tests** : tests unitaires du moteur de règles et de la résolution des variables, tests d'intégration avec Postgres et Kafka réels (testcontainers), test de relivraison (un worker tué en plein run) et de rejeu idempotent, tests de charge sur `worker-calls`, tests de non-régression de l'isolation entre organisations et de la protection SSRF.
