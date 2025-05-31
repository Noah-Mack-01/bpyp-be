#!/bin/sh
# Number of instances to run
INSTANCE_COUNT=${INSTANCE_COUNT:-3}
BASE_PORT=${PORT:-3000}

# Calculate optimal worker count per instance based on available CPUs
TOTAL_CPUS=$(nproc)
export BPYP_WORKER_MULTIPLIER=${BPYP_WORKER_MULTIPLIER:-2}
echo "Starting $INSTANCE_COUNT instances on a $TOTAL_CPUS CPU system with $BPYP_WORKER_MULTIPLIER workers"

# Launch instances
for i in $(seq 1 $INSTANCE_COUNT); do
  export PORT=$(($BASE_PORT + $i - 1))
  echo "Starting instance $i on port $PORT"
  ./server &
done

# Wait for all background processes
wait