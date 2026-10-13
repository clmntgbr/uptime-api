#!/usr/bin/env bash
# Formats KRaft storage with the admin SCRAM credential on first boot, then
# starts the broker. Service credentials and ACLs are created by topics.sh.
set -euo pipefail

: "${CLUSTER_ID:?CLUSTER_ID is required}"
: "${KAFKA_ADMIN_PASSWORD:?KAFKA_ADMIN_PASSWORD is required}"

CONFIG=/tmp/server.properties
sed "s|\${KAFKA_ADMIN_PASSWORD}|${KAFKA_ADMIN_PASSWORD}|g" /etc/kafka-custom/server.properties > "${CONFIG}"

/opt/kafka/bin/kafka-storage.sh format \
  --ignore-formatted \
  --cluster-id "${CLUSTER_ID}" \
  --config "${CONFIG}" \
  --add-scram "SCRAM-SHA-512=[name=admin,password=${KAFKA_ADMIN_PASSWORD}]"

exec /opt/kafka/bin/kafka-server-start.sh "${CONFIG}"
