package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	pb "raft-consensus/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var nodes = []string{}

func main() {
	nodeAddrs := flag.String("nodes", "localhost:5001,localhost:5002,localhost:5003,localhost:5004,localhost:5005",
		"Comma-separated list of node addresses")
	flag.Parse()

	nodes = strings.Split(*nodeAddrs, ",")
	for i := range nodes {
		nodes[i] = strings.TrimSpace(nodes[i])
	}

	fmt.Println("RAFT Key-Value Store Client")
	fmt.Println("Commands:")
	fmt.Println("  get <key>              - Get value for key")
	fmt.Println("  set <key> <value>      - Set key to value")
	fmt.Println("  delete <key>           - Delete key")
	fmt.Println("  status                 - Show all nodes status")
	fmt.Println("  status <node_addr>     - Show specific node status")
	fmt.Println("  partition <node> <addrs> - Partition node from addresses")
	fmt.Println("  heal <node>            - Heal node's partitions")
	fmt.Println("  help                   - Show this help")
	fmt.Println("  quit                   - Exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "get":
			if len(parts) < 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			handleGet(parts[1])

		case "set":
			if len(parts) < 3 {
				fmt.Println("Usage: set <key> <value>")
				continue
			}
			handleSet(parts[1], strings.Join(parts[2:], " "))

		case "delete":
			if len(parts) < 2 {
				fmt.Println("Usage: delete <key>")
				continue
			}
			handleDelete(parts[1])

		case "status":
			if len(parts) > 1 {
				handleStatus(parts[1])
			} else {
				handleStatusAll()
			}

		case "partition":
			if len(parts) < 3 {
				fmt.Println("Usage: partition <node_addr> <comma_separated_addrs>")
				continue
			}
			handlePartition(parts[1], parts[2])

		case "heal":
			if len(parts) < 2 {
				fmt.Println("Usage: heal <node_addr>")
				continue
			}
			handleHeal(parts[1])

		case "help":
			fmt.Println("Commands:")
			fmt.Println("  get <key>              - Get value for key")
			fmt.Println("  set <key> <value>      - Set key to value")
			fmt.Println("  delete <key>           - Delete key")
			fmt.Println("  status                 - Show all nodes status")
			fmt.Println("  status <node_addr>     - Show specific node status")
			fmt.Println("  partition <node> <addrs> - Partition node from addresses")
			fmt.Println("  heal <node>            - Heal node's partitions")
			fmt.Println("  quit                   - Exit")

		case "quit", "exit":
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Printf("Unknown command: %s\n", cmd)
		}
	}
}

func getClient(addr string) (pb.RaftServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return pb.NewRaftServiceClient(conn), conn, nil
}

func findLeader() (string, pb.RaftServiceClient, *grpc.ClientConn) {
	for _, node := range nodes {
		client, conn, err := getClient(node)
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		status, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
		cancel()

		if err != nil {
			conn.Close()
			continue
		}

		if status.State == "leader" {
			return node, client, conn
		}

		conn.Close()
	}
	return "", nil, nil
}

func handleGet(key string) {
	addr, client, conn := findLeader()
	if client == nil {
		fmt.Println("Error: No leader available")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ClientRequest(ctx, &pb.ClientRequestMessage{
		Operation: "GET",
		Key:       key,
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if resp.Success {
		fmt.Printf("[%s] %s = %s\n", addr, key, resp.Value)
	} else {
		fmt.Printf("Error: %s\n", resp.Error)
	}
}

func handleSet(key, value string) {
	addr, client, conn := findLeader()
	if client == nil {
		fmt.Println("Error: No leader available")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.ClientRequest(ctx, &pb.ClientRequestMessage{
		Operation: "SET",
		Key:       key,
		Value:     value,
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if resp.Success {
		fmt.Printf("[%s] SET %s = %s (committed)\n", addr, key, value)
	} else {
		fmt.Printf("Error: %s\n", resp.Error)
	}
}

func handleDelete(key string) {
	addr, client, conn := findLeader()
	if client == nil {
		fmt.Println("Error: No leader available")
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.ClientRequest(ctx, &pb.ClientRequestMessage{
		Operation: "DELETE",
		Key:       key,
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if resp.Success {
		fmt.Printf("[%s] DELETE %s (committed)\n", addr, key)
	} else {
		fmt.Printf("Error: %s\n", resp.Error)
	}
}

func handleStatusAll() {
	fmt.Println("\n=== Cluster Status ===")
	for _, node := range nodes {
		handleStatus(node)
	}
	fmt.Println()
}

func handleStatus(addr string) {
	client, conn, err := getClient(addr)
	if err != nil {
		fmt.Printf("[%s] OFFLINE - connection failed\n", addr)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status, err := client.GetStatus(ctx, &pb.GetStatusRequest{})
	if err != nil {
		fmt.Printf("[%s] OFFLINE - %v\n", addr, err)
		return
	}

	stateMarker := " "
	if status.State == "leader" {
		stateMarker = "*"
	}

	fmt.Printf("[%s] %s%-9s term=%d commit=%d log=%d",
		status.NodeId, stateMarker, status.State,
		status.CurrentTerm, status.CommitIndex, status.LogLength)

	if len(status.Partitioned) > 0 {
		fmt.Printf(" partitioned=[%s]", strings.Join(status.Partitioned, ","))
	}
	fmt.Println()
}

func handlePartition(nodeAddr, addrs string) {
	client, conn, err := getClient(nodeAddr)
	if err != nil {
		fmt.Printf("Error connecting to %s: %v\n", nodeAddr, err)
		return
	}
	defer conn.Close()

	addresses := strings.Split(addrs, ",")
	for i := range addresses {
		addresses[i] = strings.TrimSpace(addresses[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Partition(ctx, &pb.PartitionRequest{
		Addresses: addresses,
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if resp.Success {
		fmt.Printf("Partitioned %s from %v\n", nodeAddr, addresses)
	} else {
		fmt.Printf("Error: %s\n", resp.Message)
	}
}

func handleHeal(nodeAddr string) {
	client, conn, err := getClient(nodeAddr)
	if err != nil {
		fmt.Printf("Error connecting to %s: %v\n", nodeAddr, err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Heal(ctx, &pb.HealRequest{})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if resp.Success {
		fmt.Printf("Healed all partitions for %s\n", nodeAddr)
	} else {
		fmt.Printf("Error: %s\n", resp.Message)
	}
}
