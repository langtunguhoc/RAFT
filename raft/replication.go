package raft

import (
	"context"
	"log"
	"time"

	pb "raft-consensus/proto"
)

// sendHeartbeats sends periodic heartbeats to all followers
func (n *Node) sendHeartbeats() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-n.stopCh:
			return
		case <-ticker.C:
			n.mu.RLock()
			if n.state != Leader {
				n.mu.RUnlock()
				return
			}
			n.mu.RUnlock()

			// Send AppendEntries to all peers
			for _, peer := range n.peers {
				go n.sendAppendEntries(peer)
			}
		}
	}
}

// sendAppendEntries sends AppendEntries RPC to a specific peer
func (n *Node) sendAppendEntries(peer string) {
	if n.isPartitioned(peer) {
		return
	}

	client, ok := n.getPeerClient(peer)
	if !ok {
		return
	}

	n.mu.RLock()
	if n.state != Leader {
		n.mu.RUnlock()
		return
	}

	// Get entries to send
	nextIdx := n.nextIndex[peer]
	prevLogIndex := nextIdx - 1
	prevLogTerm := n.getLogTerm(prevLogIndex)

	// Get entries from nextIndex to end of log
	var entries []*pb.LogEntry
	if nextIdx < int64(len(n.log)) {
		for i := nextIdx; i < int64(len(n.log)); i++ {
			entry := n.log[i]
			entries = append(entries, &pb.LogEntry{
				Term:    entry.Term,
				Index:   entry.Index,
				Command: entry.Command,
			})
		}
	}

	req := &pb.AppendEntriesRequest{
		Term:         n.currentTerm,
		LeaderId:     n.id,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: n.commitIndex,
	}
	currentTerm := n.currentTerm
	n.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := client.AppendEntries(ctx, req)
	if err != nil {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if we're still leader
	if n.state != Leader || n.currentTerm != currentTerm {
		return
	}

	// If response term is greater, become follower
	if resp.Term > n.currentTerm {
		log.Printf("[%s] Received higher term %d from %s, becoming follower", n.id, resp.Term, peer)
		n.becomeFollower(resp.Term, "")
		n.mu.Unlock()
		n.resetElectionTimer()
		n.mu.Lock()
		return
	}

	if resp.Success {
		// Update nextIndex and matchIndex
		if len(entries) > 0 {
			n.matchIndex[peer] = entries[len(entries)-1].Index
			n.nextIndex[peer] = n.matchIndex[peer] + 1
			log.Printf("[%s] AppendEntries to %s succeeded, matchIndex=%d", n.id, peer, n.matchIndex[peer])
		}

		// Check if we can advance commit index
		n.advanceCommitIndex()
	} else {
		// Decrement nextIndex and retry
		if n.nextIndex[peer] > 1 {
			n.nextIndex[peer]--
			log.Printf("[%s] AppendEntries to %s failed, decrementing nextIndex to %d", n.id, peer, n.nextIndex[peer])
		}
	}
}

// advanceCommitIndex checks if we can advance the commit index
func (n *Node) advanceCommitIndex() {
	// Find the highest index that is replicated on a majority
	for idx := n.commitIndex + 1; idx < int64(len(n.log)); idx++ {
		if n.log[idx].Term != n.currentTerm {
			continue
		}

		// Count replications
		replicatedCount := 1 // Leader has it
		for _, peer := range n.peers {
			if n.matchIndex[peer] >= idx {
				replicatedCount++
			}
		}

		// Check if majority
		if replicatedCount >= n.quorumSize() {
			n.commitIndex = idx
			log.Printf("[%s] Advanced commit index to %d", n.id, n.commitIndex)
		}
	}
}

// AppendEntries handles AppendEntries RPC (heartbeat and log replication)
func (n *Node) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	resp := &pb.AppendEntriesResponse{
		Term:    n.currentTerm,
		Success: false,
	}

	// If request term is less than current term, reject
	if req.Term < n.currentTerm {
		log.Printf("[%s] Rejecting AppendEntries: leader term %d < current term %d", n.id, req.Term, n.currentTerm)
		return resp, nil
	}

	// If request term is greater or equal, update current term and become follower
	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term, req.LeaderId)
	} else if n.state != Follower {
		n.becomeFollower(n.currentTerm, req.LeaderId)
	}

	n.leaderId = req.LeaderId
	resp.Term = n.currentTerm

	// Reset election timer since we received valid AppendEntries from leader
	n.mu.Unlock()
	n.resetElectionTimer()
	n.mu.Lock()

	// Check if log contains an entry at prevLogIndex with matching term
	if req.PrevLogIndex > 0 {
		if req.PrevLogIndex >= int64(len(n.log)) {
			log.Printf("[%s] Log too short: prevLogIndex=%d, logLen=%d", n.id, req.PrevLogIndex, len(n.log))
			resp.MatchIndex = int64(len(n.log)) - 1
			return resp, nil
		}

		if n.log[req.PrevLogIndex].Term != req.PrevLogTerm {
			log.Printf("[%s] Term mismatch at index %d: expected %d, got %d",
				n.id, req.PrevLogIndex, req.PrevLogTerm, n.log[req.PrevLogIndex].Term)
			// Delete conflicting entry and all that follow
			n.log = n.log[:req.PrevLogIndex]
			resp.MatchIndex = int64(len(n.log)) - 1
			return resp, nil
		}
	}

	// Append new entries
	for _, entry := range req.Entries {
		// If existing entry conflicts with new one, delete it and all following
		if entry.Index < int64(len(n.log)) {
			if n.log[entry.Index].Term != entry.Term {
				n.log = n.log[:entry.Index]
			}
		}

		// Append entry if not already present
		if entry.Index >= int64(len(n.log)) {
			n.log = append(n.log, LogEntry{
				Term:    entry.Term,
				Index:   entry.Index,
				Command: entry.Command,
			})
			n.persist()
			log.Printf("[%s] Appended entry: index=%d, term=%d, command=%s",
				n.id, entry.Index, entry.Term, entry.Command)
		}
	}

	// Update commit index
	if req.LeaderCommit > n.commitIndex {
		lastNewIndex := int64(len(n.log)) - 1
		if req.LeaderCommit < lastNewIndex {
			n.commitIndex = req.LeaderCommit
		} else {
			n.commitIndex = lastNewIndex
		}
		log.Printf("[%s] Updated commit index to %d", n.id, n.commitIndex)
	}

	resp.Success = true
	resp.MatchIndex = int64(len(n.log)) - 1
	return resp, nil
}

// ProposeCommand proposes a new command to be replicated
func (n *Node) ProposeCommand(command string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state != Leader {
		return false
	}

	// Create new log entry
	newIndex := int64(len(n.log))
	entry := LogEntry{
		Term:    n.currentTerm,
		Index:   newIndex,
		Command: command,
	}

	n.log = append(n.log, entry)
	n.persist()
	n.matchIndex[n.id] = newIndex

	log.Printf("[%s] Leader appended new entry: index=%d, term=%d, command=%s",
		n.id, newIndex, n.currentTerm, command)

	return true
}

// WaitForCommit waits for an index to be committed with timeout
func (n *Node) WaitForCommit(index int64, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		n.mu.RLock()
		if n.commitIndex >= index {
			n.mu.RUnlock()
			return true
		}
		n.mu.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
	return false
}
