#!/bin/bash

# RAFT Cluster Startup Script
# Starts 5 nodes for a RAFT consensus cluster

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_DIR/bin"
LOG_DIR="$PROJECT_DIR/logs"

# Create log directory
mkdir -p "$LOG_DIR"

# Check if binaries exist
if [ ! -f "$BIN_DIR/node" ]; then
    echo "Node binary not found. Building..."
    cd "$PROJECT_DIR"
    make build
fi

# Stop any existing cluster
echo "Stopping any existing cluster..."
pkill -f "bin/node" 2>/dev/null || true
sleep 1

# Node addresses
NODE1="localhost:5001"
NODE2="localhost:5002"
NODE3="localhost:5003"
NODE4="localhost:5004"
NODE5="localhost:5005"

echo "Starting 5-node RAFT cluster..."
echo ""

# Start Node 1
echo "Starting Node 1 at $NODE1..."
"$BIN_DIR/node" -id node1 -address "$NODE1" \
    -peers "$NODE2,$NODE3,$NODE4,$NODE5" \
    > "$LOG_DIR/node1.log" 2>&1 &
echo "  PID: $!"

# Start Node 2
echo "Starting Node 2 at $NODE2..."
"$BIN_DIR/node" -id node2 -address "$NODE2" \
    -peers "$NODE1,$NODE3,$NODE4,$NODE5" \
    > "$LOG_DIR/node2.log" 2>&1 &
echo "  PID: $!"

# Start Node 3
echo "Starting Node 3 at $NODE3..."
"$BIN_DIR/node" -id node3 -address "$NODE3" \
    -peers "$NODE1,$NODE2,$NODE4,$NODE5" \
    > "$LOG_DIR/node3.log" 2>&1 &
echo "  PID: $!"

# Start Node 4
echo "Starting Node 4 at $NODE4..."
"$BIN_DIR/node" -id node4 -address "$NODE4" \
    -peers "$NODE1,$NODE2,$NODE3,$NODE5" \
    > "$LOG_DIR/node4.log" 2>&1 &
echo "  PID: $!"

# Start Node 5
echo "Starting Node 5 at $NODE5..."
"$BIN_DIR/node" -id node5 -address "$NODE5" \
    -peers "$NODE1,$NODE2,$NODE3,$NODE4" \
    > "$LOG_DIR/node5.log" 2>&1 &
echo "  PID: $!"

echo ""
echo "Cluster started successfully!"
echo ""
echo "Logs are available in: $LOG_DIR"
echo "  - tail -f $LOG_DIR/node1.log"
echo "  - tail -f $LOG_DIR/*.log"
echo ""
echo "To connect with client:"
echo "  $BIN_DIR/client"
echo ""
echo "To stop the cluster:"
echo "  ./scripts/stop_cluster.sh"
