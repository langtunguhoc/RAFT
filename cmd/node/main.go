package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"raft-consensus/raft"
	pb "raft-consensus/proto"

	"google.golang.org/grpc"
)

func main() {
	// Parse command line arguments
	id := flag.String("id", "", "Node ID (required)")
	address := flag.String("address", "", "Node address (host:port) (required)")
	peers := flag.String("peers", "", "Comma-separated list of peer addresses")
	flag.Parse()

	if *id == "" || *address == "" {
		log.Fatal("Usage: node -id <node_id> -address <host:port> -peers <peer1:port,peer2:port,...>")
	}

	// Parse peer addresses
	var peerList []string
	if *peers != "" {
		peerList = strings.Split(*peers, ",")
		// Trim whitespace
		for i := range peerList {
			peerList[i] = strings.TrimSpace(peerList[i])
		}
	}

	log.Printf("Starting RAFT node %s at %s", *id, *address)
	log.Printf("Peers: %v", peerList)

	// Create RAFT node
	config := raft.DefaultConfig(*id, *address, peerList)
	node := raft.NewNode(config)

	// Create gRPC server
	lis, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *address, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRaftServiceServer(grpcServer, node)

	// Start RAFT node
	node.Start()

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Printf("Shutting down node %s...", *id)
		node.Stop()
		grpcServer.GracefulStop()
	}()

	// Start gRPC server
	log.Printf("Node %s listening on %s", *id, *address)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
