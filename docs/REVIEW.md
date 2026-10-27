# Review du projet uptime-api

Date : 2026-10-27. Branche `main`.

## Vérifié OK

- `go build ./...` : OK.
- `go vet ./...` : aucune erreur.
- `go test ./...` : tous les tests passent (les tests d'intégration Postgres/Kafka sont probablement skip sans Docker, non vérifié ici).
- Chiffrement des secrets (`internal/infrastructure/crypto/cipher.go`) : AES-256-GCM, nonce aléatoire, clé 32 octets. Correct.
- Token Centrifugo (`internal/infrastructure/centrifugo/token.go`) : canal restreint au user, `sub` et `exp` présents. Correct.
- SSRF (`internal/infrastructure/httpclient/ssrf.go`) : résolution DNS unique, IP vérifiée puis dial direct, contrôle sur les redirections. Plages privées, loopback, link-local et métadonnées cloud bloquées. `SSRF_ALLOW_PRIVATE` désactivé par défaut. Correct.
- Requêtes SQL des repositories `read/` et `write/` : paramétrées, filtrées par `user_id`. Pas d'injection vue. `TRUNCATE` de `purge.go` : noms de tables issus de `information_schema` puis quotés.
- Pas de fichier `.env` suivi par git (`.gitignore` OK). Seul `.env.dist` est suivi.

## P1 : haute priorité

Aucun point bloquant trouvé dans le code applicatif.

## P2 : priorité moyenne

### 1. Pas de `.dockerignore` : corrigé
- Fichier : `.dockerignore` (ajouté à la racine, non commité).
- Problème initial : `COPY . .` dans le stage `base` copiait `.env`, `.git/`, `tmp/`, `bin/` dans les layers. Le stage `development` hérite de `base`, donc l'image dev contenait le `.env`. Le stage `production` ne copie que les binaires, donc l'image prod n'était pas touchée.
- Fix appliqué : `.env`, `.env.*`, `.git`, `tmp`, `bin`, `coverage*`, `*.log`, `docs`, `.cursor`, `.github`, `deploy`, `caddy`, `centrifugo`, `compose*.yaml`, `codecov.yml`, `README.md` exclus.
- Vérification : build du stage `base`, puis `ls /app` dans l'image : `.env`, `.git`, `tmp`, `bin`, `docs` absents ; `migrations/` présent (embarqué via `go:embed`).

### 2. TLS 1.0 autorisé pour les appels monitorés : corrigé
- Fichier : `internal/infrastructure/httpclient/executor.go`.
- Problème initial : `MinVersion: tls.VersionTLS10` en dur.
- Fix appliqué : minimum par défaut TLS 1.2 (`Config.MinTLSVersion`). Configurable au niveau worker-calls via `HTTP_MIN_TLS_VERSION` (`1.0`, `1.1`, `1.2`, `1.3`, défaut `1.2`), parsé et validé au démarrage (`httpclient.ParseMinTLSVersion`).
- Limite : le réglage est global au worker, pas par call. Un réglage par call demande un champ dans le modèle `call` (non fait).
- Test : `TestSession_RejectsTLSBelowMinimum` (serveur TLS 1.0 rejeté par défaut), `TestParseMinTLSVersion`.

### 3. Panic dans le middleware webhook : corrigé
- Fichier : `internal/interfaces/http/middleware/user_webhook_middleware.go`, `cmd/api/di/container.go`.
- Problème initial : `svix.NewWebhook` appelé à chaque requête, `panic(err)` en cas d'erreur.
- Fix appliqué : `NewUserWebhookMiddleware` construit le client svix une fois et retourne une erreur. Le conteneur DI fait `log.Fatalf` si `CLERK_WEBHOOK_SECRET` est invalide. Un secret vide est accepté par svix, donc le démarrage ne casse pas en dev : les webhooks sont alors rejetés (401).

### 4. Pas de `recover` middleware Fiber : corrigé
- Fichier : `cmd/api/main.go`.
- Problème : aucun `recover.New()`. Tout panic dans un handler produit une connexion coupée au lieu d'un 500 JSON.
- Fix appliqué : `app.Use(recover.New())` ajouté en tête de chaîne, avant `helmet`, pour couvrir aussi les panics de helmet et cors (écart par rapport au fix proposé « après helmet »).

## P3 : priorité basse

### 5. Secrets Centrifugo avec valeurs par défaut en dev
- Fichier : `compose.dev.yaml:14-15` (`CENTRIFUGO_API_KEY:-dev-centrifugo-api-key`, `CENTRIFUGO_TOKEN_SECRET:-dev-centrifugo-token-secret`).
- Problème : si ce compose sert hors dev, les secrets par défaut sont connus.
- Fix : retirer les défauts, ou les rendre obligatoires (`${VAR:?}`).

### 6. Dockerfile non épinglé
- Fichier : `Dockerfile`.
- Problème : `golangci-lint@latest` (stage dev), `alpine:latest` (stage prod), `golang:1.26-alpine` non figé par digest.
- Fix : épingler versions (`golangci-lint@vX.Y.Z`, `alpine:3.x`).

### 7. Documentation en retard sur le code
- Fichier : `docs/main.md:525`, `:629`.
- Problème : la doc cite `worker-notify`, qui n'existe pas (`cmd/` contient `api`, `cli`, `scheduler`, `worker`, `worker-calls`).
- Fix : corriger le nom ou retirer la mention.

### 8. Header `Server-Timing` expose la durée interne
- Fichier : `cmd/api/main.go` (middleware final).
- Problème : information mineure sur la charge serveur. Acceptable, à garder en tête.
- Fix : facultatif, derrière un flag de debug.

### 9. Token Centrifugo valable 24 h sans révocation
- Fichier : `internal/infrastructure/centrifugo/token.go:36`.
- Problème : un token volé reste utilisable 24 h. Risque limité car canal scopé au user.
- Fix : réduire la durée ou renouveler côté client ; à arbitrer.

## Architecture et qualité du code

### Architecture : conforme
- `domain`, `application`, `engine`, `contract` n'importent aucun `infrastructure` ni `interfaces`.
- `interfaces/http` n'importe `infrastructure` nulle part (le middleware webhook importe seulement le SDK svix, autorisé par `.cursor/rules/architecture.mdc`).
- Les usages d'`infrastructure` dans `cmd/` sont des composition roots ou des commandes CLI (`config`, `migrate`, `purge`, `crypto`). Acceptable.
- Structure des packages conforme au `architecture.mdc`.

### P2 : erreurs ignorées sur les IDs (Kafka)
- Fichiers : `internal/application/command/run/record_result.go:123-125` (`toRun`), `record_call_progress.go:50-52`, `internal/application/command/schedule/dispatch_due_runs.go:298-299`, `run_manual.go:177-178`.
- Problème : `runID, _ := uuid.Parse(...)`. Un ID invalide donne `uuid.Nil` sans erreur, et ce Nil est persisté ou utilisé pour un upsert. Le message Kafka est alors accepté silencieusement.
- Scénario : un message `contract.Result` avec `RunID` malformé écrit un run d'ID `00000000-…`.
- Fix : retourner une erreur (ou message non-retryable / DLQ) quand `Parse` échoue.

### P3 : points mineurs
- `internal/application/event/dedup/with_dedup.go:46` : `_ = releaser.UnmarkProcessed(...)` ignore l'erreur. Logger au minimum.
- `internal/infrastructure/httpclient/executor.go:54` : `jar, _ := cookiejar.New(nil)`. Sans options, l'erreur est impossible, acceptable, à commenter.
- `internal/infrastructure/config/database.go:37,42` et `cmd/*/di/container.go` : `log.Fatalf` dans les constructeurs. Acceptable en démarrage, mais rend le wiring difficile à tester.
- Fichiers volumineux : `engine/runner.go` (509 l.), `read/statistics_repository.go` (436 l.), `kafka/consumer.go` (432 l.). Pas bloquant, candidats à découpage si ça grossit.

## Autres points contrôlés

### P2 : outbox relay sans verrou
- Fichier : `internal/infrastructure/persistence/outbox/repository.go:55-61` (`FetchUnpublished`).
- Problème : `SELECT ... WHERE published_at IS NULL` sans `FOR UPDATE SKIP LOCKED`. Si deux instances `worker` tournent, elles publient les mêmes events. Le dedup consumer limite les dégâts, mais c'est du double travail et du double envoi Kafka.
- Fix : `clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}` dans la même transaction que `MarkPublished`.

### P2 : CI ne lance pas tous les tests
- Fichier : `.github/workflows/tests.yml`.
- Problème : le job `unit-tests` couvre `domain`, `engine`, `application`, `infrastructure`, `cmd`. Les jobs de couverture ne couvrent que `interfaces/http/handler/test/...`. `internal/interfaces/http/middleware`, `presenter`, `validation` ne sont couverts par aucun job. Pas de job `go vet` ni `golangci-lint` alors que le Dockerfile installe golangci-lint.
- Fix : ajouter `go vet ./...` et un job lint ; vérifier la couverture des packages hors handler.

### P3 : chemins d'erreur à vérifier
- `write/workflow_repository.go:173` : `GetByID` renvoie `nil, nil` si absent. Les appelants doivent tester `nil` (vérifié dans `update_workflow.go` et `delete_workflow.go`, à vérifier partout).
- Ownership : `ErrWrongOwner` (403) vs `ErrNotFound` (404). Si le mapping HTTP diffère, un attaquant peut tester l'existence d'IDs. Vérifier le mapping dans `handler/workflow_helpers.go`.
- Relay : publication puis `MarkPublished` non atomique. Crash entre les deux = doublon. Acceptable grâce au dedup, à documenter.

### Vérifié OK
- Ownership appliquée côté write (`ErrWrongOwner` dans workflow/call commands) et côté read (`user_id` dans les requêtes).
- Routes : toutes les routes `/api` passent par `AuthenticateMiddleware`. Seuls health checks et webhook Clerk (protégé par signature svix) sont publics.
- `purge` : refuse sans `--force`. Tables protégées : `users`, `goose_db_version`.

## Notes

- Tests d'intégration (`internal/infrastructure/persistence/*_integration_test.go`) non exécutés ici. À lancer avec Docker.
- Revue faite par lecture ciblée, pas exhaustive sur chaque handler HTTP et chaque use case.
