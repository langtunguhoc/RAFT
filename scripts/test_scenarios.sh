#!/bin/bash

# RAFT Test Scenarios Script
# Tests various failure scenarios

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_DIR/bin"

CLIENT="$BIN_DIR/client"

echo "=========================================="
echo "RAFT Consensus Test Scenarios"
echo "=========================================="
echo ""
echo "Make sure the cluster is running before executing tests!"
echo "Use: make run-cluster"
echo ""

# Helper function to run client commands
run_client() {
    echo "$1" | $CLIENT
}

# Test 1: Basic Operations
test_basic_ops() {
    echo "=========================================="
    echo "Test 1: Basic Key-Value Operations"
    echo "=========================================="
    echo ""

    echo "Setting key1=value1..."
    echo -e "set key1 value1\nquit" | $CLIENT
    sleep 1

    echo ""
    echo "Getting key1..."
    echo -e "get key1\nquit" | $CLIENT
    sleep 1

    echo ""
    echo "Checking cluster status..."
    echo -e "status\nquit" | $CLIENT

    echo ""
    echo "Test 1 Complete!"
    echo ""
}

# Test 2: Leader Failure
test_leader_failure() {
    echo "=========================================="
    echo "Test 2: Leader Failure Simulation"
    echo "=========================================="
    echo ""
    echo "This test requires manual steps:"
    echo ""
    echo "1. Run 'make client' and type 'status' to find the leader"
    echo "2. Kill the leader process (Ctrl+C in its terminal or kill PID)"
    echo "3. Wait 1-2 seconds and run 'status' again"
    echo "4. A new leader should be elected"
    echo ""
    echo "Example commands in client:"
    echo "  > status"
    echo "  [node1] *leader    term=1 commit=5 log=6"
    echo "  [node2]  follower  term=1 commit=5 log=6"
    echo "  ..."
    echo ""
    echo "  # Kill node1 process"
    echo ""
    echo "  > status"
    echo "  [node1] OFFLINE - connection failed"
    echo "  [node2] *leader    term=2 commit=5 log=6"
    echo "  ..."
    echo ""
}

# Test 3: Network Partition
test_network_partition() {
    echo "=========================================="
    echo "Test 3: Network Partition Simulation"
    echo "=========================================="
    echo ""
    echo "This test uses the partition command to simulate network splits."
    echo ""
    echo "Scenario: Split into two groups"
    echo "  Group A: node1, node2 (minority)"
    echo "  Group B: node3, node4, node5 (majority)"
    echo ""
    echo "Example commands in client:"
    echo ""
    echo "1. Partition node1 from majority:"
    echo "   > partition localhost:5001 localhost:5003,localhost:5004,localhost:5005"
    echo ""
    echo "2. Partition node2 from majority:"
    echo "   > partition localhost:5002 localhost:5003,localhost:5004,localhost:5005"
    echo ""
    echo "3. Check status - majority group should elect leader:"
    echo "   > status"
    echo ""
    echo "4. Try to write in minority (should fail or timeout):"
    echo "   > set test_key test_value"
    echo ""
    echo "5. Heal partitions:"
    echo "   > heal localhost:5001"
    echo "   > heal localhost:5002"
    echo ""
    echo "6. Check that logs sync:"
    echo "   > status"
    echo ""
}

# Test 4: Member Failure and Recovery
test_member_recovery() {
    echo "=========================================="
    echo "Test 4: Member Failure and Recovery"
    echo "=========================================="
    echo ""
    echo "This test demonstrates log synchronization after member rejoins."
    echo ""
    echo "Steps:"
    echo "1. Write some data while all nodes are running"
    echo "2. Kill a follower node"
    echo "3. Write more data (should succeed with remaining quorum)"
    echo "4. Restart the killed node"
    echo "5. Check that all nodes have the same log"
    echo ""
    echo "Quorum analysis for 5 nodes:"
    echo "  - Quorum size: 3 nodes (majority)"
    echo "  - Can tolerate: 2 node failures"
    echo "  - With 3 failures: Cannot commit new entries"
    echo ""
}

# Print usage
usage() {
    echo "Usage: $0 [test_number]"
    echo ""
    echo "Tests:"
    echo "  1 - Basic Key-Value Operations"
    echo "  2 - Leader Failure Simulation (instructions)"
    echo "  3 - Network Partition Simulation (instructions)"
    echo "  4 - Member Failure and Recovery (instructions)"
    echo ""
    echo "Without arguments, prints all test instructions."
}

# Main
case "$1" in
    1)
        test_basic_ops
        ;;
    2)
        test_leader_failure
        ;;
    3)
        test_network_partition
        ;;
    4)
        test_member_recovery
        ;;
    *)
        test_basic_ops
        echo ""
        test_leader_failure
        echo ""
        test_network_partition
        echo ""
        test_member_recovery
        ;;
esac
