#!/bin/bash

# Number of instances to run inside the container
INSTANCE_COUNT=${INSTANCE_COUNT:-3}

# Expose each port for each instance
PORT_MAPPING=""
for i in $(seq 1 $INSTANCE_COUNT); do
  PORT=$((3000 + $i - 1))
  PORT_MAPPING="$PORT_MAPPING -p $PORT:$PORT"
done

sudo docker run $PORT_MAPPING \
    -e INSTANCE_COUNT="$INSTANCE_COUNT" \
    -e BPYP_WORKER_MULTIPLIER="2" \
    -e BPYP_POSTGRES_DIR_CONN="${BPYP_POSTGRES_DIR_CONN}" \
    -e BPYP_SUPABASE_URL="${BPYP_SUPABASE_URL}" \
    -e BPYP_SUPABASE_SERVICE_KEY="${BPYP_SUPABASE_SERVICE_KEY}" \
    -e BPYP_POSTGRES_JWT_SECRET="${BPYP_POSTGRES_JWT_SECRET}" \
    -e BPYP_WIT_URL="${BPYP_WIT_URL}" \
    -e BPYP_WIT_API_KEY="${BPYP_BEARER_API}" \
    -e OPENAI_API_KEY="${OPENAI_API_KEY}"\
    bpyp-go:latest
