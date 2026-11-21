# Emails de notification

Catalogue des e-mails envoyés par `worker-notify`. Canal unique en v1 : SMTP.

Circuit commun : producteur (`worker` / `worker-scheduler`) → Kafka `notifications.requests` → `worker-notify` → `mail.Render` → SMTP. Les destinataires viennent de `workflow_notification_settings.emails` (ou e-mail créateur en fallback pour les kinds system).

## Template

- **Layout partagé** : `internal/infrastructure/mail/layout.go` (titre, message, tableau de détails).
- **Un fichier par e-mail** : `incident_opened.go`, `expiry_threshold.go`, `run_failed.go`, … — chacun enregistre son `kind` et remplit `Content` (subject / title / rows).
- **Dispatch** : `render.go` → `mail.Render(req)`.

Les secrets, corps HTTP et variables ne sont jamais inclus. Sujet : `[Uptime] …`.

Pour ajouter un e-mail : créer `internal/infrastructure/mail/<kind_with_underscores>.go` avec `init() { register(...) }`, puis documenter ici.

## Réglages (par workflow)

| Famille | Toggle | Défaut |
| --- | --- | --- |
| Incidents | `incidentsEnabled` | activé |
| TLS / expiry | `tlsEnabled` | activé |
| Runs | `runsEnabled` + `runStatuses` + `runMode` (`every` \| `transition`) | désactivé ; statuses `failed`,`error` ; mode `transition` |
| Workflow system | — | toujours envoyé (créateur / fallback user) |

Liste vide d’e-mails → pas d’envoi (sauf chemins system qui résolvent l’e-mail user en fallback).

API : `GET/PUT /api/workflows/:id/notification-settings`.

## Catalogue live

| Kind | Quand | Gate | Exemple de sujet |
| --- | --- | --- | --- |
| `incident.opened` | Incident ouvert (seuils `open_after`) | Incidents | `[Uptime] Incident · call_failed` |
| `incident.reopened` | Rechute dans la fenêtre de reopen | Incidents | `[Uptime] Incident · call_failed` |
| `incident.resolved` | Incident résolu (`close_after`) | Incidents | `[Uptime] Incident · availability_low` |
| `incident.reminder` | Incident encore ouvert (rappels scheduler) | Incidents | `[Uptime] Incident · call_failed` |
| `expiry.threshold` | Palier TLS franchi (30/14/7/1 j…) | TLS | `[Uptime] Certificate · api.example.com:443` |
| `expiry.expired` | Certificat expiré (`daysLeft < 0`) | TLS | `[Uptime] Certificate · api.example.com:443` |
| `expiry.renewed` | Cert renouvelé après alerte (plus aucun palier franchi) | TLS | `[Uptime] Certificate · api.example.com:443` |
| `run.failed` | Run en échec métier | Runs | `[Uptime] Run · failed` |
| `run.errored` | Erreur technique / timeout lease | Runs | `[Uptime] Run · error` |
| `run.success` | Run OK (opt-in via `runStatuses`) | Runs | `[Uptime] Run · success` |
| `workflow.disabled` | Workflow désactivé car **aucun call** au dispatch | System | `[Uptime] Workflow disabled · Checkout` |
| `workflow.one_shot.done` | Once/at terminé et désactivé | System | `[Uptime] One-shot done · Nightly export` |

### Champs utiles par kind

| Kind | `type` | `subject` | Autres |
| --- | --- | --- | --- |
| `incident.*` | kind d’incident (`call_failed`, `availability_low`, `no_run`, …) | — | `incident_id`, `severity` ; message métier (ex. « Availability breached ») |
| `expiry.*` | `tls_certificate` | `host:port` | `days_left`, `threshold_days` (threshold only), `expires_at` |
| `run.*` | `failed` / `error` | — | `item_id` = run id |
| `workflow.disabled` / `workflow.one_shot.done` | — | nom du workflow | — |

## Catalogue prévu (pas encore d’engine produit)

Kinds prêts côté template / contrat. **Aucun producteur** tant que quota, Stripe ou auth maison n’existent pas.

| Kind | Gate | Prio | Note |
| --- | --- | --- | --- |
| `quota.warning` | Account | P1 | 80 % runs / calls — à brancher sur le compteur quota |
| `quota.exhausted` | Account | P1 | 100 % |
| `billing.payment_failed` | Account | P1 | Stripe webhook |
| `billing.invoice` | Account | P2 | |
| `billing.trial_ending` | Account | P2 | |
| `auth.login_new_device` | Account | P2 | Clerk / maison |
| `auth.email_changed` | Account | P2 | |

Publier un `contract.NotificationRequest` avec `kind`, `user_id`, `workflow_id` et `message` (le contrat exige encore `workflow_id` même pour account-level — UUID du workflow ou du projet selon le producteur) ; `worker-notify` résout l’e-mail user si `recipients` est vide.

## Hors scope (P3 / volontairement exclus)

| Kind / sujet | Pourquoi |
| --- | --- |
| `incident.acknowledged` | Bruyant |
| `run.skipped` | Volume / bruit |
| `workflow.disabled.manual` / `workflow.enabled` | Opt-in plus tard |
| `library.imported` / `notify.recipient_added` | UI / soft confirm |
| Chaque ligne du **feed** | Feed = journal, pas moteur mail |

## Hors scope canaux

| Canal | Statut |
| --- | --- |
| Slack / Teams / webhook | Plus tard |
| SMS / appel | Plus tard |
| Confirmation d’adresse / unsubscribe | Plus tard |

## Opérations

- Variables : `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD` (voir `.env.dist`).
- Sans `SMTP_HOST` : `worker-notify` logue la demande, marque le résultat `sent` côté log-only (pas d’échec SMTP).
- Topic Kafka : `notifications.requests` → résultats sur `notifications.results`.
- Migration runs settings : `migrations/00027_notification_runs_settings.sql`.

## Ajouter un e-mail

1. Créer `internal/infrastructure/mail/<name>.go` avec `init() { register(...) }` qui remplit `Content` (title, subject, rows) — le HTML passe par `layout.go`.
2. Publier un `contract.NotificationRequest` (`kind`, `user_id`, `workflow_id`, `message` obligatoires).
3. Passer par `notifyresolve.PrepareRequest` si la famille incidents / TLS / runs s’applique.
4. Documenter la ligne dans ce fichier.
