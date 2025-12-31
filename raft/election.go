package raft

import (
	"context"
	"log"
	"sync"
	"time"

	pb "raft-consensus/proto"
)

// startElection starts a new election
func (n *Node) startElection() {
	n.mu.Lock()

	// Become candidate
	n.state = Candidate
	n.currentTerm++
	n.votedFor = n.id
	n.persist()
	currentTerm := n.currentTerm
	lastLogIndex, lastLogTerm := n.getLastLogInfo()

	log.Printf("[%s] Starting election for term %d", n.id, currentTerm)

	n.mu.Unlock()

	// Vote for self
	votesReceived := 1
	votesNeeded := n.quorumSize()
	var votesMu sync.Mutex
	var wg sync.WaitGroup

	// Request votes from all peers
	for _, peer := range n.peers {
		if n.isPartitioned(peer) {
			log.Printf("[%s] Skipping partitioned peer %s", n.id, peer)
			continue
		}

		wg.Add(1)
		go func(peer string) {
			defer wg.Done()

			client, ok := n.getPeerClient(peer)
			if !ok {
				log.Printf("[%s] No connection to peer %s", n.id, peer)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			req := &pb.RequestVoteRequest{
				Term:         currentTerm,
				CandidateId:  n.id,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}

			resp, err := client.RequestVote(ctx, req)
			if err != nil {
				log.Printf("[%s] RequestVote to %s failed: %v", n.id, peer, err)
				return
			}

			n.mu.Lock()
			defer n.mu.Unlock()

			// Check if we're still candidate and in the same term
			if n.state != Candidate || n.currentTerm != currentTerm {
				return
			}

			// If response term is greater, become follower
			if resp.Term > n.currentTerm {
				log.Printf("[%s] Received higher term %d from %s, becoming follower", n.id, resp.Term, peer)
				n.becomeFollower(resp.Term, "")
				return
			}

			// Count vote
			if resp.VoteGranted {
				votesMu.Lock()
				votesReceived++
				log.Printf("[%s] Received vote from %s (total: %d/%d)", n.id, peer, votesReceived, votesNeeded)
				votes := votesReceived
				votesMu.Unlock()

				// Check if we won
				if votes >= votesNeeded {
					n.becomeLeader()
				}
			}
		}(peer)
	}

	// Wait for vote responses with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(150 * time.Millisecond):
	}

	// Check if we became leader or need to start new election
	n.mu.Lock()
	if n.state == Candidate {
		// Still candidate, reset election timer for next election
		n.mu.Unlock()
		n.resetElectionTimer()
	} else {
		n.mu.Unlock()
	}
}

// becomeFollower transitions the node to follower state
func (n *Node) becomeFollower(term int64, leaderId string) {
	log.Printf("[%s] Becoming follower for term %d (leader: %s)", n.id, term, leaderId)
	n.state = Follower
	n.currentTerm = term
	n.votedFor = ""
	n.leaderId = leaderId
	n.persist()
	// Stop heartbeat timer if running
	if n.heartbeatTimer != nil {
		n.heartbeatTimer.Stop()
		n.heartbeatTimer = nil
	}
}

// becomeLeader transitions the node to leader state
func (n *Node) becomeLeader() {
	if n.state == Leader {
		return
	}

	log.Printf("[%s] Becoming leader for term %d", n.id, n.currentTerm)
	n.state = Leader
	n.leaderId = n.id

	// Initialize nextIndex and matchIndex for all peers
	lastLogIndex, _ := n.getLastLogInfo()
	for _, peer := range n.peers {
		n.nextIndex[peer] = lastLogIndex + 1
		n.matchIndex[peer] = 0
	}

	// Stop election timer
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}

	// Start sending heartbeats
	go n.sendHeartbeats()
}

// RequestVote handles RequestVote RPC
func (n *Node) RequestVote(ctx context.Context, req *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Check if partitioned from this candidate
	// (In real implementation, we wouldn't receive this request if partitioned)

	log.Printf("[%s] Received RequestVote from %s for term %d", n.id, req.CandidateId, req.Term)

	resp := &pb.RequestVoteResponse{
		Term:        n.currentTerm,
		VoteGranted: false,
	}

	// If request term is less than current term, reject
	if req.Term < n.currentTerm {
		log.Printf("[%s] Rejecting vote: candidate term %d < current term %d", n.id, req.Term, n.currentTerm)
		return resp, nil
	}

	// If request term is greater, update current term and become follower
	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term, "")
	}

	resp.Term = n.currentTerm

	// Check if we can grant vote
	// 1. We haven't voted for anyone in this term, or we already voted for this candidate
	// 2. Candidate's log is at least as up-to-date as ours
	canVote := (n.votedFor == "" || n.votedFor == req.CandidateId)

	if canVote {
		// Check if candidate's log is at least as up-to-date as ours
		lastLogIndex, lastLogTerm := n.getLastLogInfo()

		// Candidate's log is more up-to-date if:
		// 1. Its last log term is greater, OR
		// 2. Same last log term but longer log
		logUpToDate := req.LastLogTerm > lastLogTerm ||
			(req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)

		if logUpToDate {
			log.Printf("[%s] Granting vote to %s for term %d", n.id, req.CandidateId, req.Term)
			n.votedFor = req.CandidateId
			resp.VoteGranted = true
			n.persist()
			// Reset election timer since we granted a vote
			n.mu.Unlock()
			n.resetElectionTimer()
			n.mu.Lock()
		} else {
			log.Printf("[%s] Rejecting vote: candidate log not up-to-date (candidate: %d/%d, ours: %d/%d)",
				n.id, req.LastLogIndex, req.LastLogTerm, lastLogIndex, lastLogTerm)
		}
	} else {
		log.Printf("[%s] Rejecting vote: already voted for %s", n.id, n.votedFor)
	}

	return resp, nil
}
