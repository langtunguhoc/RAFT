package raft

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	pb "raft-consensus/proto"
)

// ClientRequest handles client key-value operations
func (n *Node) ClientRequest(ctx context.Context, req *pb.ClientRequestMessage) (*pb.ClientRequestResponse, error) {
	n.mu.RLock()
	state := n.state
	leaderId := n.leaderId
	n.mu.RUnlock()

	resp := &pb.ClientRequestResponse{
		Success:  false,
		LeaderId: leaderId,
	}

	// If not leader, redirect to leader
	if state != Leader {
		if leaderId != "" {
			resp.Error = fmt.Sprintf("Not leader, redirect to %s", leaderId)
		} else {
			resp.Error = "No leader available"
		}
		return resp, nil
	}

	switch strings.ToUpper(req.Operation) {
	case "GET":
		value, ok := n.GetValue(req.Key)
		if ok {
			resp.Success = true
			resp.Value = value
		} else {
			resp.Error = "Key not found"
		}

	case "SET":
		command := fmt.Sprintf("SET %s %s", req.Key, req.Value)
		if n.ProposeCommand(command) {
			// Wait for commit
			n.mu.RLock()
			index := int64(len(n.log)) - 1
			n.mu.RUnlock()

			if n.WaitForCommit(index, 5000*time.Millisecond) {
				resp.Success = true
				resp.Value = req.Value
				log.Printf("[%s] SET %s=%s committed", n.id, req.Key, req.Value)
			} else {
				resp.Error = "Set Commit timeout"
			}
		} else {
			resp.Error = "Failed to propose command"
		}

	case "DELETE":
		command := fmt.Sprintf("DELETE %s", req.Key)
		if n.ProposeCommand(command) {
			// Wait for commit
			n.mu.RLock()
			index := int64(len(n.log)) - 1
			n.mu.RUnlock()

			if n.WaitForCommit(index, 5000*time.Millisecond) {
				resp.Success = true
				log.Printf("[%s] DELETE %s committed", n.id, req.Key)
			} else {
				resp.Error = "Delete Commit timeout"
			}
		} else {
			resp.Error = "Failed to propose command"
		}

	default:
		resp.Error = "Unknown operation. Use GET, SET, or DELETE"
	}

	return resp, nil
}

// GetStatus returns the current status of the node
func (n *Node) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	n.partitionedMu.RLock()
	var partitioned []string
	for addr := range n.partitioned {
		if n.partitioned[addr] {
			partitioned = append(partitioned, addr)
		}
	}
	n.partitionedMu.RUnlock()

	return &pb.GetStatusResponse{
		NodeId:      n.id,
		State:       n.state.String(),
		CurrentTerm: n.currentTerm,
		LeaderId:    n.leaderId,
		CommitIndex: n.commitIndex,
		LastApplied: n.lastApplied,
		LogLength:   int64(len(n.log)),
		Peers:       n.peers,
		Partitioned: partitioned,
	}, nil
}

// Partition simulates network partition by disconnecting from specified addresses
func (n *Node) Partition(ctx context.Context, req *pb.PartitionRequest) (*pb.PartitionResponse, error) {
	n.partitionedMu.Lock()
	defer n.partitionedMu.Unlock()

	for _, addr := range req.Addresses {
		n.partitioned[addr] = true
		log.Printf("[%s] Partitioned from %s", n.id, addr)
	}

	return &pb.PartitionResponse{
		Success: true,
		Message: fmt.Sprintf("Partitioned from %d addresses", len(req.Addresses)),
	}, nil
}

// Heal restores all connections after network partition
func (n *Node) Heal(ctx context.Context, req *pb.HealRequest) (*pb.HealResponse, error) {
	n.partitionedMu.Lock()
	defer n.partitionedMu.Unlock()

	count := 0
	for addr := range n.partitioned {
		if n.partitioned[addr] {
			n.partitioned[addr] = false
			count++
			log.Printf("[%s] Healed connection to %s", n.id, addr)
		}
	}

	return &pb.HealResponse{
		Success: true,
		Message: fmt.Sprintf("Healed %d connections", count),
	}, nil
}
