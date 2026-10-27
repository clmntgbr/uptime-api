# Protéger l'API — guide de sécurité

Périmètre : API Go/Fiber (monitoring HTTP, extraction par regex, variables, proxy, webhooks) et `worker-calls`.

## 1. Modèle de menace

Le produit appelle des URLs configurées par l'utilisateur. On ne peut donc pas « interdire » les appels sortants, on doit les **encadrer**.

| Menace | Gravité | Statut |
|---|---|---|
| SSRF (accès réseau interne) | Haute | Dial SSRF + refus `SSRF_ALLOW_PRIVATE` en prod |
| DoS par regex / gros body | Moyenne | Pattern ≤500, scan regex ≤1 Mo, body LimitReader |
| Proxy ouvert utilisé comme pivot | Moyenne | Même dialer SSRF ; schémas http(s) only |
| IDOR / fuite entre tenants | Haute | Filtrage owner en place ; tests IDOR à étendre |
| Fuite de secrets (variables, headers) | Haute | Chiffrement/masque existants ; message SSRF sans IP |
| Abus de ressources (spam de monitors) | Moyenne | Rate limit + BodyLimit API ; quotas plan à compléter |
| SQLi / XSS / injection commande | Faible | Déjà couvert ; header CRLF rejeté |

## 2. SSRF

Déjà en place : blocage des IP privées au dial, désactivable via `SSRF_ALLOW_PRIVATE=true`.

### À faire
- ~~Refuser le démarrage en prod~~ **Fait** : `worker-calls` refuse `SSRF_ALLOW_PRIVATE=true` si `GO_ENV=production|prod`.
- Filtrer **après résolution DNS** (protège contre le DNS rebinding : on valide l'IP réellement utilisée, pas le hostname). **Fait** dans `internal/infrastructure/httpclient/ssrf.go` : le dialer résout, contrôle chaque IP, puis se connecte directement à l'IP validée.
- Bloquer : `127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16` (**metadata cloud 169.254.169.254**), `100.64.0.0/10`, `0.0.0.0/8`, `::1`, `fc00::/7`, `fe80::/10`, et les IPv4 mappées en IPv6 (`::ffff:a.b.c.d`).
- Autoriser uniquement les schémas `http` / `https`.
- **Redirections** : revalider chaque hop (le dialer le fait déjà si le client est unique), limiter à 5, interdire `https → http` si besoin.
- Liste de ports autorisés si possible (80, 443, 8080, 8443…).
- Ne jamais renvoyer au client le détail brut des erreurs réseau (permet de cartographier le réseau interne) : message générique + code. **Fait** : le message `destination is not allowed: <host>` n'inclut plus l'IP résolue (testé).

Implémentation de référence : `internal/infrastructure/httpclient/ssrf.go` (`IsBlocked`, `dialer.DialContext`) et `executor.go` (`CheckRedirect`, `MaxRedirects = 5`). Le proxy passe par le même transport, donc par le même dialer.

## 3. Regex d'extraction (ReDoS)

Point important : le package standard `regexp` de Go est en **RE2 (temps linéaire)**, il n'y a donc pas de backtracking catastrophique. Le risque reste réel dans deux cas :
- usage d'une lib type `regexp2` / PCRE (backtracking) → à éviter ;
- regex complexe × body très gros → CPU élevé, même en temps linéaire.

### À faire
- Rester sur `regexp` (RE2). Interdire `regexp2`.
- Limiter la longueur du pattern (ex. 500 caractères) et le compiler **à la sauvegarde** (rejet 422 si invalide).
- Ne lancer l'extraction que sur les **N premiers Ko/Mo** du body (ex. 1 Mo).
- Exécuter l'extraction avec un timeout (un `regexp` Go n'est pas annulable : l'exécuter dans une goroutine bornée + abandon du résultat, et limiter la concurrence avec un pool/semaphore).
- Limiter le nombre de groupes de capture et la taille des valeurs extraites.

```go
const maxPattern = 500
const maxBodyForRegex = 1 << 20 // 1 Mo

func compileUserRegex(p string) (*regexp.Regexp, error) {
    if len(p) > maxPattern { return nil, errors.New("pattern trop long") }
    return regexp.Compile(p)
}
```

## 4. Proxy configurable par l'utilisateur

Un proxy défini par l'utilisateur est un vecteur de pivot (et d'exfiltration des headers/secrets envoyés).

- Option la plus sûre : **pas de proxy défini par l'utilisateur**, uniquement des proxies gérés par vous (liste admin).
- Sinon : appliquer les **mêmes règles SSRF** à l'hôte du proxy, schémas limités (`http`, `https`, `socks5`), jamais de credentials dans les logs.
- Avertir dans l'UI que les headers et le body transitent par le proxy.
- Plan payant / vérification de compte avant d'autoriser cette fonctionnalité.

## 5. Limites, quotas, anti-DoS

### Entrées API (Fiber)
```go
app := fiber.New(fiber.Config{
    BodyLimit:    1 * 1024 * 1024, // 1 Mo
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
})
// Fiber v3 : fiber.Ctx est une valeur. Le limiter compte par IP par défaut.
app.Use(limiter.New(limiter.Config{Max: 100, Expiration: time.Minute}))
```
Implémenté dans `cmd/api/main.go` (`BodyLimit`, timeouts, limiter via `RATE_LIMIT_MAX`). Le rate limit par utilisateur n'existe pas encore.
- Rate limit strict sur login / création de compte / création de monitor.
- Validation systématique : longueurs max de chaînes, nombre max de headers, de variables, d'étapes de workflow.

### Appels sortants (worker-calls)
- **Taille de réponse plafonnée** : `io.LimitReader` dans `httpclient/executor.go`. Valeur actuelle : 50 Mio (`engine.MaxBodyBytes`). Recommandation : baisser à 2–5 Mo, décision ouverte (impact sur l'extraction des gros corps).
- Timeout global par call, timeout de dial, de TLS handshake et de headers.
- Concurrence max par worker **et par utilisateur** (évite qu'un tenant sature les autres).
- Intervalle minimum entre deux exécutions d'un monitor (ex. ≥ 30 s selon le plan).
- Quotas par plan : nombre de monitors, de groupes, de variables, de webhooks, rétention des logs.
- Backoff / circuit breaker si une cible répond en erreur en boucle.
- Protection anti-amplification : un utilisateur ne doit pas pouvoir faire cibler un même tiers par des milliers de monitors (limite par domaine cible).

## 6. Authentification et autorisation

- **Toujours filtrer par propriétaire/tenant** dans chaque requête (`WHERE id = ? AND owner_id = ?`) : l'`uuid.Parse` valide le format, pas le droit d'accès (IDOR).
- Tests automatisés : un utilisateur A ne peut ni lire, ni modifier, ni exécuter les ressources de B.
- Tokens : durée courte, rotation, révocation. Auth via Clerk (JWT) ; pas de clé API dans le produit à ce jour (point « clés hashées » N/A).
- Comparaisons de secrets en temps constant (`subtle.ConstantTimeCompare`).
- Endpoints admin / internes (métriques, pprof, health détaillé) non exposés publiquement.

## 7. Secrets et variables

- Variables sensibles chiffrées au repos (AES-GCM, clé hors base), masquées dans les réponses API (`****`) après création.
- **Masquer** dans les logs et l'historique des calls : `Authorization`, `Cookie`, `X-API-Key`, tokens en query string, mots de passe de proxy.
- Les templates `{{token}}` restent de la **substitution simple** : pas d'`eval`, pas de moteur de template avec exécution (`text/template` avec fonctions), pas d'appel shell.
- Ne jamais substituer une variable d'un utilisateur dans un autre contexte tenant.
- Échappement approprié lors de l'injection dans URL / headers (interdire `\r\n` dans les valeurs de headers : header injection).

## 8. Webhooks (log + redirection)

> **Statut** : non implémenté. Seul le webhook Clerk entrant existe (`/webhooks/clerk`, signature svix). Les points ci-dessous sont une cible pour une future fonctionnalité de webhooks sortants.

- Endpoint d'ingestion : taille de body plafonnée, rate limit par endpoint, token d'URL non devinable (≥ 128 bits aléatoires).
- **Redirection** vers une URL cible = SSRF : mêmes protections que §2 (client dédié).
- Signature HMAC (`X-Signature`) sur les webhooks sortants ; horodatage + rejet des rejeux.
- Ne pas suivre les redirections 3xx sur l'envoi sortant, ou les revalider.
- Logs des payloads : rétention limitée, accès restreint au propriétaire.

## 9. Infrastructure et observabilité

- TLS partout (API, Kafka, Centrifugo, base) ; Kafka/Centrifugo/DB **non exposés** sur Internet, auth activée.
- Conteneurs non-root (OK, stage `production`), filesystem en lecture seule si possible, pas de secrets dans les images. À faire : `.dockerignore` (sinon `.env` est copié dans l'image dev).
- **Egress filtré au niveau réseau** pour `worker-calls` (défense en profondeur : l'accès à la DB, aux services internes et au metadata cloud doit être impossible depuis ce worker, même si le filtre applicatif échoue).
- Workers isolés de l'API principale (pas les mêmes credentials DB).
- Headers de sécurité (`X-Content-Type-Options: nosniff`, `Strict-Transport-Security`), CORS restrictif (liste d'origines explicite, pas de `*` avec credentials).
- Logs structurés + alertes : pics de 4xx/5xx, tentatives SSRF bloquées, regex rejetées, dépassements de quotas.
- Dépendances : `govulncheck ./...` et Dependabot/Renovate en CI.

## 10. Checklist avant mise en production

- [x] `SSRF_ALLOW_PRIVATE=false` et démarrage refusé sinon (`worker-calls` refuse en `GO_ENV=production|prod`)
- [x] Blocage metadata cloud + IPv6 + IPv4 mappées testé (`httpclient.IsBlocked` + dial après résolution DNS)
- [x] Redirections limitées (5) et revalidées ; `https → http` refusé
- [x] `regexp` standard uniquement, pattern ≤ 500 car., scan regex plafonné à 1 Mo, valeur extraite ≤ 4 Ko
- [x] Taille de réponse plafonnée (`LimitReader`, `MaxBodyBytes`)
- [x] Timeouts dial / TLS / total (timeout du call) + API `Read/Write/IdleTimeout`
- [x] Rate limit global configurable (`RATE_LIMIT_MAX`) + BodyLimit API 1 Mo
- [ ] Concurrence limitée par utilisateur dans `worker-calls`
- [x] Proxy utilisateur soumis au même dialer SSRF (schémas `http`/`https` uniquement)
- [ ] Filtrage par tenant sur toutes les requêtes (tests IDOR exhaustifs)
- [x] Secrets chiffrés/masqués ; erreur SSRF sans IP résolue dans le message (corrigé, testé)
- [ ] `.dockerignore` (`.env`, `.git`, `tmp/`, `bin/` hors des images)
- [ ] Egress réseau du worker restreint
- [ ] `govulncheck` en CI
- [x] Header values : rejet de `\r\n` (header injection)

## 11. Tests de non-régression

- [x] Appels vers loopback / privés / metadata / IPv4-mapped → rejetés (`TestIsBlocked`, `TestSession_BlocksPrivateDestinations`)
- [x] Message de blocage sans IP résolue (`TestSession_BlocksPrivateDestinations`, nom DNS vers loopback)
- [x] Redirection `https → http` → rejetée (`TestSession_RejectsHTTPSToHTTPRedirect`)
- [x] Réponse trop grande → `body_too_large` (`TestSession_LimitsRedirectsAndBody`)
- [x] Regex trop longue → rejetée à la sauvegarde (`TestNormalizeExtraction_RegexPatternLength`)
- [x] Header avec `\r\n` → rejeté (`TestNew_RejectsHeaderCRLF`)
- [ ] Utilisateur B accédant à une ressource de A → 404 (à compléter côté handler tests)
