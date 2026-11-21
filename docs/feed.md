# Feed d'événements (journal)

Statut : **implémenté (API)**. Front (card + drawer) hors de ce repo. Voir §10.

## 1. Objectif

Un journal chronologique des anomalies et transitions : échecs de call/run, transitions d'incident, seuils TLS ou disponibilité franchis.

Le feed ne remplace pas les incidents. Les deux se complètent :

| | Incident | Feed |
|---|---|---|
| Nature | État durable (ouvert → acquitté → résolu) | Trace ponctuelle, append-only |
| Question | « Quel est le problème en cours ? » | « Que s'est-il passé, et quand ? » |
| Notification | Oui | Non (sauf branchement volontaire) |
| Lifecycle | open / ack / resolve | Une ligne par événement, jamais modifiée |
| Lien | — | Vers l'incident ou le run, s'il existe |

Règle de conception : le feed **ne refait pas la machine d'état** des incidents. Il enregistre les transitions qui ont eu lieu. Les libellés sont des faits (`call.failed`, `incident.opened`), pas des états (`Triggered`, `OK`).

## 2. Sources déjà présentes

Rien de nouveau à détecter : tout ce qu'il faut est déjà calculé au moment où le résultat est enregistré.

| Source | Où elle est écrite | Ce qu'elle apporte |
|---|---|---|
| Run | `runs`, écrit par `RecordResultHandler` (worker) | statut, durée, compteurs par statut de call, raison |
| Call d'un run | `call_runs` | statut, HTTP status, latence (`timings`), erreur, position |
| Règle d'un call | `rule_runs` | règle évaluée, passée ou non, valeur observée |
| Signaux appliqués | `ApplySignalsHandler` (`call_failed`, `call_slow`, `workflow_failed`, `chain_broken`, `no_run`, `availability_low`) | cause d'un incident |
| Transitions d'incident | `incident_events` (`opened`, `failing`, `acknowledged`, `resolved`, `reopened`, `reminder`) | historique déjà persisté |
| Certificats TLS | `certificates`, `expiry_items`, `expiry_alerts` | expiration, seuils franchis |
| Workflow | événements `workflow.created/updated/deleted` (outbox) | modifications de configuration |
| Bibliothèque | `library.imported` (outbox) | imports |

Le temps réel passe déjà par Centrifugo (`run.*`, `call.*`, `incident.*`, `library.*`). Le feed pourra s'y brancher plus tard (`feed.created`).

## 3. Modèle

Une table dédiée, `feed_events`, alimentée dans la **même transaction** que l'écriture source.

```
feed_events
  id           uuid        PK
  user_id      uuid        FK users, filtre principal (v1 : scope utilisateur)
  workflow_id  uuid        nullable (SET NULL si workflow supprimé — historique conservé)
  run_id       uuid        nullable
  call_id      uuid        nullable
  incident_id  uuid        nullable, lien vers l'incident
  kind         text        vocabulaire fermé (voir §4)
  severity     text        info | success | warning | critical (toujours renseigné)
  at           timestamptz instant de l'événement (heure métier, pas heure d'écriture)
  title        text        phrase courte EN (fallback) ; le front i18n via kind + payload
  payload      jsonb       détails : latence, HTTP status, valeur observée, seuil…
```

Index : `(user_id, at DESC)` pour la liste principale, `(workflow_id, at DESC)` pour le filtre par workflow, `(incident_id)` pour le lien.

Pas de mise à jour ni de suppression ligne à ligne. La seule opération destructive est la purge par rétention.

## 4. Vocabulaire des événements (`kind`) — v1

Verbes de fait, en anglais dans le code. **Anomalies seulement** : pas de `call.passed`, pas de `run.passed`.

**Calls** (uniquement si anomalie)
- `call.failed` : au moins une règle fausse ou statut HTTP hors critère.
- `call.errored` : erreur technique (timeout, TLS, réseau).
- `call.slow` : latence au-dessus du seuil (signal `call_slow`).

Un call sans règle qui répond 200 → **aucune ligne feed**.

**Runs** (uniquement non réussis)
- `run.failed` / `run.errored` : résultat final en échec.
- `run.skipped` : passe ignorée (chevauchement, `overlap`).
- `run.timed_out` : bail expiré sans résultat.

**Incidents** (miroir de `incident_events`, avec lien)
- `incident.opened`, `incident.reopened`, `incident.acknowledged`, `incident.resolved`.
- Pas de `incident.failing` : signal interne, trop bruyant.

**Seuils et observations**
- `tls.threshold_crossed` : un événement **par palier** franchi (30, 14, 7, 1 jour), comme `expiry_alerts`.
- `tls.expired`.
- `availability.breached` / `availability.recovered` : une fois chacun au passage sous / au-dessus du seuil.
- `expiry.threshold_crossed`.

**Configuration**
- `workflow.created`, `workflow.updated`, `workflow.deleted`, `library.imported`.

Hors v1 (éventuel plus tard) : `run.passed` résumé optionnel par workflow ; jamais le détail call-par-call des succès.

## 5. Quand et comment l'écrire

- **Dans le worker, au moment de `RecordResultHandler`** : anomalies call/run écrites dans la même transaction. Pas de perte, pas de doublon grâce au `processed_events` déjà en place.
- **Transitions d'incident** : déjà dans `incident_events`. **Lecture directe** dans l'API feed (pas de duplication dans `feed_events`), fusionnées chronologiquement avec les lignes `feed_events`.
- **Seuils TLS** : un event par palier franchi (claim comme `expiry_alerts`), écrit dans `feed_events`.
- **Disponibilité** : projetée depuis `incident_events` (`availability_low` → `availability.breached` / `availability.recovered`), une fois à l'ouverture / résolution d'incident.
- **Configuration** (`workflow.*`, `library.imported`) : kinds réservés, writers hors v1.

## 6. Volume

Décision : **anomalies seulement**.

Un workflow de 10 calls / min ne génère des lignes feed que lorsque quelque chose échoue, ralentit, ou change d'état (incident / seuil). Les succès restent dans `runs` / `call_runs` / statistics.

## 7. Rétention

Rétention **propre au feed**, distincte de `RESULTS_RETENTION_DAYS` (runs).

- Défaut : **30 jours**
- Variable d'environnement : `FEED_RETENTION_DAYS`
- Purge par lots, même modèle que `DeleteRunsBefore`

Workflow supprimé : les lignes feed **restent** (`workflow_id` mis à NULL ou conservé sans FK stricte) pour garder l'historique.

## 8. API

```
GET /api/feed?cursor=…&limit=50&workflowId=…&kind=…&severity=…&from=…&to=…
```

- Scope v1 : **utilisateur courant** uniquement (pas de partage équipe).
- Pagination par curseur `(at, id)` : la liste bouge en continu, l'offset donnerait des doublons.
- Réponse : chaque ligne contient `kind`, `severity`, `at`, `title`, `workflowId`, `callId`, `runId`, `incidentId`, `payload`.
- Titres : `title` EN en fallback ; le front affiche via `kind` + `payload` (i18n).
- Temps réel : **hors v1** — refresh manuel / polling au départ. Plus tard : `feed.created` sur `users:{id}`.

### Affichage (front)

- **Card** sur le détail workflow, à côté des blocs incidents et last run : aperçu des derniers événements (quelques lignes).
- **Drawer** « Voir tout » : liste complète paginée (même `GET /api/feed`, filtre `workflowId` prérempli).
- Pas de page dédiée en v1.

## 9. Ce que le feed n'est pas

- Pas une source de vérité pour l'état d'un incident : l'API incidents reste la référence.
- Pas un moteur de notification : une ligne feed n'envoie aucun mail.
- Pas un outil d'analyse : les agrégats (disponibilité, latence moyenne) restent dans `statistics`.

## 10. Décisions

| # | Sujet | Décision |
|---|---|---|
| Q1 | Niveau de détail | Anomalies seulement |
| Q2 | Runs réussis | Non en v1 (`run.passed` éventuel plus tard, optionnel) |
| Q3 | Call sans règle / succès HTTP | Aucune ligne feed (`call.passed` non émis) |
| Q4 | Rétention | 30 jours, indépendante des runs (`FEED_RETENTION_DAYS`) |
| Q5 | Workflow supprimé | Historique conservé |
| Q6 | Temps réel | Refresh manuel en v1 ; Centrifugo ensuite |
| Q7 | UI | Card à côté des incidents / last run ; drawer pour tout voir |
| Q8 | Langue | `kind` + payload ; `title` EN fallback ; i18n front |
| Q9 | Périmètre | User only en v1 |
| Q10 | Sévérité | Sur chaque ligne feed |
| Q11 | Seuils TLS / dispo | TLS : un event par palier ; dispo : breached + recovered |

## 11. Étapes d'implémentation

1. Migration `feed_events` + index (+ FK `workflow_id` nullable / ON DELETE SET NULL).
2. Écriture dans `RecordResultHandler` (anomalies call/run), tests transactionnels.
3. API `GET /api/feed` : fusion `feed_events` + transitions `incident_events`, pagination curseur, tests HTTP.
4. Écriture seuils TLS / disponibilité au franchissement.
5. Purge `FEED_RETENTION_DAYS`.
6. Front : card aperçu (incidents / last run) + drawer liste complète, i18n sur `kind`.
7. (Plus tard) `feed.created` Centrifugo + éventuel `run.passed` optionnel.
