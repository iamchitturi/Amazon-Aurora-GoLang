package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	Type     string `json:"type"`
	ID       int    `json:"id,omitempty"`
	LeaderID int    `json:"leader_id,omitempty"`
	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
	Status   string `json:"status,omitempty"`
	Msg      string `json:"msg,omitempty"`
}

type NodeInfo struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type Config struct {
	Nodes map[string]NodeInfo `json:"nodes"`
}

var (
	nodes    map[int]NodeInfo
	leaderID int = -1
)

func loadConfig() {
	file, err := os.Open("../cluster_config.json")
	if err != nil {
		fmt.Printf("Error: cluster_config.json not found in parent directory.\n")
		os.Exit(1)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		fmt.Printf("Error decoding config: %v\n", err)
		os.Exit(1)
	}

	nodes = make(map[int]NodeInfo)
	for idStr, info := range config.Nodes {
		id, _ := strconv.Atoi(idStr)
		nodes[id] = info
	}

	// Set initial leader to highest ID (Bully assumption)
	maxID := -1
	for id := range nodes {
		if id > maxID {
			maxID = id
		}
	}
	leaderID = maxID
}

func sendRequest(targetID int, msg Message) (*Message, error) {
	target, ok := nodes[targetID]
	if !ok {
		return nil, fmt.Errorf("node %d not found", targetID)
	}

	address := fmt.Sprintf("%s:%d", target.IP, target.Port)
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	decoder := json.NewDecoder(conn)
	var response Message
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func main() {
	loadConfig()

	fmt.Println("=========================================================")
	fmt.Println("      Amazon Aurora Client Simulator Interface (GO)")
	fmt.Println("=========================================================\n")
	fmt.Println("Commands:")
	fmt.Println("  write <key> <value>   - Send log-structured write to leader")
	fmt.Println("  read <key>            - Read data from the leader")
	fmt.Println("  exit                  - Close client\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("[Client -> Leader Node %d]> ", leaderID)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]

		if cmd == "exit" {
			break
		}

		if cmd == "write" || cmd == "read" {
			if cmd == "write" && len(parts) != 3 {
				fmt.Println("Error: Invalid syntax. Usage: write <key> <value>")
				continue
			}
			if cmd == "read" && len(parts) != 2 {
				fmt.Println("Error: Invalid syntax. Usage: read <key>")
				continue
			}

			key := parts[1]
			var value string
			var msgType string
			if cmd == "write" {
				value = parts[2]
				msgType = "WRITE"
				fmt.Printf("Action: Sending WRITE request [%s=%s] to Leader Node %d...\n", key, value, leaderID)
			} else {
				msgType = "READ"
				fmt.Printf("Action: Sending READ request for [%s] to Leader Node %d...\n", key, leaderID)
			}
			
			res, err := sendRequest(leaderID, Message{Type: msgType, Key: key, Value: value})
			
			// Handle leader change
			if err == nil && res != nil && res.Status == "ERROR" && strings.Contains(res.Msg, "Leader is") {
				msgParts := strings.Fields(res.Msg)
				newLeaderID, _ := strconv.Atoi(msgParts[len(msgParts)-1])
				fmt.Printf("Info: Redirecting to NEW Leader Node %d...\n", newLeaderID)
				leaderID = newLeaderID
				res, err = sendRequest(leaderID, Message{Type: msgType, Key: key, Value: value})
			}

			// Handle node failure / Lead discovery
			if err != nil {
				fmt.Printf("Error: Leader Node %d is DOWN. Attempting Auto-Discovery...\n", leaderID)
				
				discovered := false
				nodeIDs := []int{}
				for id := range nodes {
					nodeIDs = append(nodeIDs, id)
				}
				sort.Sort(sort.Reverse(sort.IntSlice(nodeIDs)))

				for _, id := range nodeIDs {
					if id == leaderID {
						continue
					}
					fmt.Printf("Checking Node %d...\n", id)
					checkRes, checkErr := sendRequest(id, Message{Type: msgType, Key: key, Value: value})
					if checkErr == nil && checkRes != nil {
						if checkRes.Status == "ERROR" && strings.Contains(checkRes.Msg, "Leader is") {
							msgParts := strings.Fields(checkRes.Msg)
							newID, _ := strconv.Atoi(msgParts[len(msgParts)-1])
							leaderID = newID
							fmt.Printf("Success: Discovered Leader Node %d. Routing request...\n", leaderID)
							res, _ = sendRequest(leaderID, Message{Type: msgType, Key: key, Value: value})
							discovered = true
							break
						} else if checkRes.Status == "SUCCESS" {
							fmt.Printf("Success: Node %d is the new leader.\n", id)
							leaderID = id
							res = checkRes
							discovered = true
							break
						}
					}
				}

				if !discovered {
					fmt.Println("Critical Error: Cluster unavailable.")
					continue
				}
			}

			if res != nil {
				if res.Status == "SUCCESS" {
					if cmd == "read" {
						fmt.Printf("Result: [FOUND] %s = %s\n", key, res.Value)
					} else {
						fmt.Printf("Result: [SUCCESS] %s\n", res.Msg)
					}
				} else {
					fmt.Printf("Result: [FAILED] %s\n", res.Msg)
				}
			}
		} else {
			fmt.Println("Unknown command. Supported: 'write <key> <value>', 'read <key>' or 'exit'.")
		}
	}

	fmt.Println("Exiting Client Simulator...")
}
