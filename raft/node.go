package raft

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
	"encoding/json"
	"path/filepath"
    "os"
	"strings"

	pb "raft-consensus/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NodeState represents the state of a RAFT node
type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

func (s NodeState) String() string {
	switch s {
	case Follower:
		return "follower"
	case Candidate:
		return "candidate"
	case Leader:
		return "leader"
	default:
		return "unknown"
	}
}

// LogEntry represents a log entry in RAFT
type LogEntry struct {
	Term    int64
	Index   int64
	Command string
}

// Node represents a RAFT node
type Node struct {
	pb.UnimplementedRaftServiceServer

	mu sync.RWMutex

	// Node identification
	id      string
	address string
	peers   []string // Addresses of other nodes

	// Persistent state (should be persisted to disk in production)
	currentTerm int64
	votedFor    string
	log         []LogEntry

	// Volatile state on all servers
	commitIndex int64
	lastApplied int64
	state       NodeState
	leaderId    string

	// Volatile state on leaders
	nextIndex  map[string]int64
	matchIndex map[string]int64

	// Timers
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	// Network partition simulation
	partitioned    map[string]bool
	partitionedMu  sync.RWMutex

	// Key-value store (state machine)
	kvStore   map[string]string
	kvStoreMu sync.RWMutex

	// gRPC connections to peers
	peerConns   map[string]pb.RaftServiceClient
	peerConnsMu sync.RWMutex

	// Channels
	stopCh chan struct{}

	// Random source for election timeout
	rand *rand.Rand
}

// Config holds configuration for a RAFT node
type Config struct {
	ID                   string
	Address              string
	Peers                []string
	ElectionTimeoutMin   time.Duration
	ElectionTimeoutMax   time.Duration
	HeartbeatInterval    time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig(id, address string, peers []string) *Config {
	return &Config{
		ID:                   id,
		Address:              address,
		Peers:                peers,
		ElectionTimeoutMin:   150 * time.Millisecond,
		ElectionTimeoutMax:   300 * time.Millisecond,
		HeartbeatInterval:    50 * time.Millisecond,
	}
}

// NewNode creates a new RAFT node
func NewNode(config *Config) *Node {
	n := &Node{
		id:          config.ID,
		address:     config.Address,
		peers:       config.Peers,
		currentTerm: 0,
		votedFor:    "",
		log:         make([]LogEntry, 0),
		commitIndex: 0,
		lastApplied: 0,
		state:       Follower,
		leaderId:    "",
		nextIndex:   make(map[string]int64),
		matchIndex:  make(map[string]int64),
		partitioned: make(map[string]bool),
		kvStore:     make(map[string]string),
		peerConns:   make(map[string]pb.RaftServiceClient),
		stopCh:      make(chan struct{}),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	n.readPersist()

    if len(n.log) == 0 {
        n.log = append(n.log, LogEntry{Term: 0, Index: 0, Command: ""})
    }
	
	n.restoreStateMachine()
    return n
}

// Start starts the RAFT node
func (n *Node) Start() {
	log.Printf("[%s] Starting RAFT node at %s", n.id, n.address)
	log.Printf("[%s] Peers: %v", n.id, n.peers)

	// Connect to peers
	go n.connectToPeers()

	// Start election timer
	n.resetElectionTimer()

	// Start applying committed entries
	go n.applyCommittedEntries()
}

// Stop stops the RAFT node
func (n *Node) Stop() {
	close(n.stopCh)
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	if n.heartbeatTimer != nil {
		n.heartbeatTimer.Stop()
	}
}

// connectToPeers establishes gRPC connections to all peers
func (n *Node) connectToPeers() {
	for _, peer := range n.peers {
		go n.connectToPeer(peer)
	}
}

func (n *Node) connectToPeer(peer string) {
	for {
		select {
		case <-n.stopCh:
			return
		default:
		}

		conn, err := grpc.Dial(peer, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("[%s] Failed to connect to peer %s: %v", n.id, peer, err)
			time.Sleep(1 * time.Second)
			continue
		}

		n.peerConnsMu.Lock()
		n.peerConns[peer] = pb.NewRaftServiceClient(conn)
		n.peerConnsMu.Unlock()

		log.Printf("[%s] Connected to peer %s", n.id, peer)
		return
	}
}

// getPeerClient returns a gRPC client for a peer
func (n *Node) getPeerClient(peer string) (pb.RaftServiceClient, bool) {
	n.peerConnsMu.RLock()
	defer n.peerConnsMu.RUnlock()
	client, ok := n.peerConns[peer]
	return client, ok
}

// isPartitioned checks if we are partitioned from a peer
func (n *Node) isPartitioned(peer string) bool {
	n.partitionedMu.RLock()
	defer n.partitionedMu.RUnlock()
	return n.partitioned[peer]
}

// getRandomElectionTimeout returns a random election timeout
func (n *Node) getRandomElectionTimeout() time.Duration {
	min := 150
	max := 300
	return time.Duration(min+n.rand.Intn(max-min)) * time.Millisecond
}

// resetElectionTimer resets the election timer
func (n *Node) resetElectionTimer() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}

	timeout := n.getRandomElectionTimeout()
	n.electionTimer = time.AfterFunc(timeout, func() {
		n.startElection()
	})
}

// getLastLogInfo returns the last log index and term
func (n *Node) getLastLogInfo() (int64, int64) {
	if len(n.log) == 0 {
		return 0, 0
	}
	lastEntry := n.log[len(n.log)-1]
	return lastEntry.Index, lastEntry.Term
}

// getLogTerm returns the term of a log entry at given index
func (n *Node) getLogTerm(index int64) int64 {
	if index < 0 || index >= int64(len(n.log)) {
		return 0
	}
	return n.log[index].Term
}

// applyCommittedEntries applies committed log entries to the state machine
func (n *Node) applyCommittedEntries() {
	for {
		select {
		case <-n.stopCh:
			return
		case <-time.After(10 * time.Millisecond):
			n.mu.Lock()
			for n.lastApplied < n.commitIndex {
				n.lastApplied++
				if n.lastApplied < int64(len(n.log)) {
					entry := n.log[n.lastApplied]
					n.applyToStateMachine(entry)
				}
			}
			n.mu.Unlock()
		}
	}
}

// saveKVStore dumps the current kvStore map to a JSON file (database snapshot)
// Note: This function assumes the caller already holds n.kvStoreMu
func (n *Node) saveKVStore() {
    // Convert the map to indented JSON for readability
    data, err := json.MarshalIndent(n.kvStore, "", "  ")
    if err != nil {
        log.Printf("[%s] Error marshaling database: %v", n.id, err)
        return
    }

    // Define the file path in the logs folder (e.g., "logs/node1_database.json")
    filename := filepath.Join("logs", fmt.Sprintf("%s_database.json", n.id))

    // Write to file (create if not exists, overwrite if exists)
    err = os.WriteFile(filename, data, 0644)
    if err != nil {
        log.Printf("[%s] Error saving database to disk: %v", n.id, err)
    } else {
        // Optional: Uncomment for debugging if you want to see when it saves
        // fmt.Printf("[%s] Database saved to %s\n", n.id, filename)
    }
}

// raft/node.go

func (n *Node) applyToStateMachine(entry LogEntry) {
    n.kvStoreMu.Lock()
    defer n.kvStoreMu.Unlock()

    // --- PARSING PART (Your original logic) ---
    // Parse command: "SET key value" or "DELETE key"
    var op, key, value string
    
    // This reads the string entry.Command and fills the variables
    fmt.Sscanf(entry.Command, "%s %s %s", &op, &key, &value)

    switch op {
    case "SET":
        n.kvStore[key] = value
        log.Printf("[%s] Applied: SET %s = %s (index=%d, term=%d)", n.id, key, value, entry.Index, entry.Term)
    case "DELETE":
        delete(n.kvStore, key)
        log.Printf("[%s] Applied: DELETE %s (index=%d, term=%d)", n.id, key, entry.Index, entry.Term)
    }
    
    // --- SAVING PART (New Requirement) ---
    // This creates the _database.json file
    n.saveKVStore()
}
// // applyToStateMachine applies a log entry to the key-value store
// func (n *Node) applyToStateMachine(entry LogEntry) {
// 	n.kvStoreMu.Lock()
// 	defer n.kvStoreMu.Unlock()

// 	// Parse command: "SET key value" or "DELETE key"
// 	var op, key, value string
// 	fmt.Sscanf(entry.Command, "%s %s %s", &op, &key, &value)

// 	switch op {
// 	case "SET":
// 		n.kvStore[key] = value
// 		log.Printf("[%s] Applied: SET %s = %s (index=%d, term=%d)", n.id, key, value, entry.Index, entry.Term)
// 	case "DELETE":
// 		delete(n.kvStore, key)
// 		log.Printf("[%s] Applied: DELETE %s (index=%d, term=%d)", n.id, key, entry.Index, entry.Term)
// 	}
// }

// GetValue gets a value from the key-value store
func (n *Node) GetValue(key string) (string, bool) {
	n.kvStoreMu.RLock()
	defer n.kvStoreMu.RUnlock()
	value, ok := n.kvStore[key]
	return value, ok
}

// quorumSize returns the number of nodes needed for a quorum
func (n *Node) quorumSize() int {
	return (len(n.peers)+1)/2 + 1
}

// GetState returns the current state of the node
func (n *Node) GetState() NodeState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state
}

// GetLeaderId returns the current leader ID
func (n *Node) GetLeaderId() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.leaderId
}

// GetCurrentTerm returns the current term
func (n *Node) GetCurrentTerm() int64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.currentTerm
}

type PersistentState struct {
    CurrentTerm int64
    VotedFor    string
    Log         []LogEntry
}

func (n *Node) getStorageFilename() string {
    return filepath.Join("logs", fmt.Sprintf("%s_storage.json", n.id))
}

func (n *Node) persist() {
    state := PersistentState{
        CurrentTerm: n.currentTerm,
        VotedFor:    n.votedFor,
        Log:         n.log,
    }

    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        log.Printf("[%s] Error marshaling state: %v", n.id, err)
        return
    }

    err = os.WriteFile(n.getStorageFilename(), data, 0644)
    if err != nil {
        log.Printf("[%s] Error writing state to file: %v", n.id, err)
    }
}

func (n *Node) readPersist() {
    filename := n.getStorageFilename()
    if _, err := os.Stat(filename); os.IsNotExist(err) {
        return
    }

    data, err := os.ReadFile(filename)
    if err != nil {
        log.Printf("[%s] Error reading state file: %v", n.id, err)
        return
    }

    var state PersistentState
    err = json.Unmarshal(data, &state)
    if err != nil {
        log.Printf("[%s] Error unmarshaling state: %v", n.id, err)
        return
    }


    n.currentTerm = state.CurrentTerm
    n.votedFor = state.VotedFor
    n.log = state.Log
    
    
    log.Printf("[%s] Restored state from disk: Term=%d, LogLen=%d", n.id, n.currentTerm, len(n.log))
}

// restoreStateMachine re-executes all commands in the log to rebuild the kvStore
func (n *Node) restoreStateMachine() {
    n.mu.Lock()
    defer n.mu.Unlock()

    log.Printf("[%s] Replaying %d log entries to restore state...", n.id, len(n.log))

    for _, entry := range n.log {
        if entry.Command == "" {
            continue
        }

        parts := strings.Fields(entry.Command)
        
        if len(parts) >= 3 && parts[0] == "SET" {
            key := parts[1]
            val := parts[2]
            n.kvStore[key] = val
        } else if len(parts) >= 2 && parts[0] == "DELETE" {
            key := parts[1]
            delete(n.kvStore, key)
        }
    }

    if len(n.log) > 0 {
        lastIndex := n.log[len(n.log)-1].Index
        n.commitIndex = lastIndex
        n.lastApplied = lastIndex
    }
}