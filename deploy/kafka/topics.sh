#!/usr/bin/env bash
# Creates Kafka topics (partitions, retention) and, when KAFKA_SECURITY=sasl,
# the per-service SCRAM users and their ACLs. Idempotent: safe to re-run.
set -euo pipefail

BIN=/opt/kafka/bin
BOOTSTRAP="${KAFKA_BOOTSTRAP:-kafka:9092}"
RF="${REPLICATION_FACTOR:-1}"
SECURITY="${KAFKA_SECURITY:-plaintext}"

COMMAND_CONFIG=()
if [[ "${SECURITY}" == "sasl" ]]; then
  : "${KAFKA_ADMIN_PASSWORD:?KAFKA_ADMIN_PASSWORD is required}"
  ADMIN_PROPS=/tmp/admin.properties
  cat > "${ADMIN_PROPS}" <<EOF
security.protocol=SASL_SSL
sasl.mechanism=SCRAM-SHA-512
sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required username="admin" password="${KAFKA_ADMIN_PASSWORD}";
ssl.truststore.type=PEM
ssl.truststore.location=/scripts/secrets/ca.pem
EOF
  COMMAND_CONFIG=(--command-config "${ADMIN_PROPS}")
fi

DAY_MS=86400000

# name partitions retention_ms
TOPICS=(
  "workflow.jobs 12 $((1 * DAY_MS))"
  "workflow.jobs.retry 12 $((1 * DAY_MS))"
  "workflow.jobs.dlq 12 $((30 * DAY_MS))"
  "workflow.results 12 $((7 * DAY_MS))"
  "workflow.results.retry 12 $((7 * DAY_MS))"
  "workflow.results.dlq 12 $((30 * DAY_MS))"
  "events 6 $((7 * DAY_MS))"
  "events.retry 6 $((7 * DAY_MS))"
  "events.dlq 6 $((30 * DAY_MS))"
)

echo "Waiting for Kafka at ${BOOTSTRAP}..."
until "${BIN}/kafka-topics.sh" --bootstrap-server "${BOOTSTRAP}" "${COMMAND_CONFIG[@]}" --list >/dev/null 2>&1; do
  sleep 2
done

for entry in "${TOPICS[@]}"; do
  read -r name partitions retention <<<"${entry}"
  "${BIN}/kafka-topics.sh" --bootstrap-server "${BOOTSTRAP}" "${COMMAND_CONFIG[@]}" \
    --create --if-not-exists \
    --topic "${name}" \
    --partitions "${partitions}" \
    --replication-factor "${RF}" \
    --config "retention.ms=${retention}" \
    --config "max.message.bytes=4194304"
  "${BIN}/kafka-configs.sh" --bootstrap-server "${BOOTSTRAP}" "${COMMAND_CONFIG[@]}" \
    --alter --entity-type topics --entity-name "${name}" \
    --add-config "retention.ms=${retention},max.message.bytes=4194304" >/dev/null
  echo "topic ${name}: partitions=${partitions} retention.ms=${retention}"
done

if [[ "${SECURITY}" != "sasl" ]]; then
  echo "KAFKA_SECURITY=${SECURITY}: skipping SCRAM users and ACLs"
  exit 0
fi

create_user() {
  local user="$1" password="$2"
  "${BIN}/kafka-configs.sh" --bootstrap-server "${BOOTSTRAP}" "${COMMAND_CONFIG[@]}" \
    --alter --entity-type users --entity-name "${user}" \
    --add-config "SCRAM-SHA-512=[password=${password}]"
}

acl() {
  "${BIN}/kafka-acls.sh" --bootstrap-server "${BOOTSTRAP}" "${COMMAND_CONFIG[@]}" --add --force "$@"
}

create_user scheduler "${KAFKA_SCHEDULER_PASSWORD:?}"
create_user worker "${KAFKA_WORKER_PASSWORD:?}"
create_user worker-calls "${KAFKA_WORKER_CALLS_PASSWORD:?}"

# scheduler: write jobs only.
acl --allow-principal User:scheduler --operation Write --operation Describe --topic workflow.jobs

# worker-calls: read jobs (+retry), write results, its own retry and dlq.
acl --allow-principal User:worker-calls --operation Read --operation Describe \
  --topic workflow.jobs --topic workflow.jobs.retry
acl --allow-principal User:worker-calls --operation Write --operation Describe \
  --topic workflow.results --topic workflow.jobs.retry --topic workflow.jobs.dlq
acl --allow-principal User:worker-calls --operation Read \
  --group worker-calls --group worker-calls.retry

# worker: read results and events (+retry), write events, retry and dlq of what it reads.
acl --allow-principal User:worker --operation Read --operation Describe \
  --topic workflow.results --topic workflow.results.retry --topic events --topic events.retry
acl --allow-principal User:worker --operation Write --operation Describe \
  --topic events --topic events.retry --topic events.dlq \
  --topic workflow.results.retry --topic workflow.results.dlq
acl --allow-principal User:worker --operation Read \
  --group worker --group worker.retry --group worker-events --group worker-events.retry

echo "Kafka users and ACLs configured"
