#!/bin/bash

# RAFT Cluster Shutdown Script
# Stops all running RAFT nodes

echo "Stopping RAFT cluster..."

# Find and kill all node processes
pkill -f "bin/node" 2>/dev/null || true

# Wait a moment
sleep 1

# Check if any processes are still running
if pgrep -f "bin/node" > /dev/null 2>&1; then
    echo "Some processes still running, force killing..."
    pkill -9 -f "bin/node" 2>/dev/null || true
fi

echo "Cluster stopped."
