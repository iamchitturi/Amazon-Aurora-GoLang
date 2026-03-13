package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

// Message defines the structure for all inter-node and client-node communication
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
	myID               int
	nodes              map[int]NodeInfo
	leaderID           int = -1
	storage            map[string]string
	storageLock        sync.RWMutex
	electionInProgress bool
	electionLock       sync.Mutex
)

func loadConfig() {
	file, err := os.Open("../cluster_config.json")
	if err != nil {
		fmt.Printf(ColorRed + "Error: cluster_config.json not found in parent directory." + ColorReset + "\n")
		return
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		fmt.Printf(ColorRed + "Error decoding config: %v" + ColorReset + "\n", err)
		os.Exit(1)
	}

	nodes = make(map[int]NodeInfo)
	for idStr, info := range config.Nodes {
		id, _ := strconv.Atoi(idStr)
		nodes[id] = info
	}
}

func saveStorage() {
	storageLock.RLock()
	defer storageLock.RUnlock()
	data, _ := json.Marshal(storage)
	filename := fmt.Sprintf("storage_node_%d.json", myID)
	os.WriteFile(filename, data, 0644)
}

func loadStorage() {
	filename := fmt.Sprintf("storage_node_%d.json", myID)
	data, err := os.ReadFile(filename)
	if err == nil {
		json.Unmarshal(data, &storage)
		fmt.Printf(ColorGreen + "[RECOVERY] Loaded %d keys from persistent log-structured storage." + ColorReset + "\n", len(storage))
	}
}

func sendMessage(targetID int, msg Message, timeout time.Duration) (*Message, error) {
	target, ok := nodes[targetID]
	if !ok {
		return nil, fmt.Errorf("node %d not found in config", targetID)
	}

	address := fmt.Sprintf("%s:%d", target.IP, target.Port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(timeout))
	decoder := json.NewDecoder(conn)
	var response Message
	if err := decoder.Decode(&response); err != nil {
		if err == io.EOF {
			return nil, nil // Some messages don't expect a response
		}
		return nil, err
	}

	return &response, nil
}

func startElection() {
	electionLock.Lock()
	if electionInProgress {
		electionLock.Unlock()
		return
	}
	electionInProgress = true
	electionLock.Unlock()

	fmt.Printf("\n" + ColorYellow + "[ELECTION] Node %d starting Bully Election..." + ColorReset + "\n", myID)
	
	higherNodes := []int{}
	for id := range nodes {
		if id > myID {
			higherNodes = append(higherNodes, id)
		}
	}

	responses := 0
	for _, id := range higherNodes {
		fmt.Printf(ColorYellow + "[ELECTION] Sending ELECTION message to Node %d..." + ColorReset + "\n", id)
		res, err := sendMessage(id, Message{Type: "ELECTION", ID: myID}, time.Second)
		if err == nil && res != nil && res.Status == "OK" {
			responses++
		}
	}

	if responses == 0 {
		fmt.Printf(ColorGreen + "[ELECTION] I AM THE NEW LEADER!" + ColorReset + "\n")
		leaderID = myID
		for id := range nodes {
			if id != myID {
				sendMessage(id, Message{Type: "COORDINATOR", LeaderID: myID}, time.Second)
			}
		}
	} else {
		fmt.Printf(ColorYellow + "[ELECTION] Higher node responded. Waiting for Coordinator announcement..." + ColorReset + "\n")
	}

	electionLock.Lock()
	electionInProgress = false
	electionLock.Unlock()
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var msg Message
	if err := decoder.Decode(&msg); err != nil {
		return
	}

	switch msg.Type {
	case "ELECTION":
		encoder.Encode(Message{Status: "OK"})
		go startElection()

	case "COORDINATOR":
		leaderID = msg.LeaderID
		fmt.Printf("\n" + ColorCyan + "[LEADER] Node %d is now the Primary Coordinator." + ColorReset + "\n", leaderID)
		encoder.Encode(Message{Status: "OK"})

	case "REPLICATE":
		storageLock.Lock()
		storage[msg.Key] = msg.Value
		storageLock.Unlock()
		saveStorage()
		fmt.Printf(ColorBlue + "[AURORA-STORAGE] Log Segment Replicated: %s -> %s" + ColorReset + "\n", msg.Key, msg.Value)
		encoder.Encode(Message{Status: "OK"})

	case "WRITE":
		if leaderID != myID {
			encoder.Encode(Message{Status: "ERROR", Msg: fmt.Sprintf("Not leader. Leader is %d", leaderID)})
			return
		}

		fmt.Printf("\n" + ColorPurple + "[WRITE] Client Request: %s=%s. Initiating Quorum (W=3, N=4)..." + ColorReset + "\n", msg.Key, msg.Value)
		
		storageLock.Lock()
		storage[msg.Key] = msg.Value
		storageLock.Unlock()
		saveStorage()

		ackCount := 1 // Self
		var wg sync.WaitGroup
		var mu sync.Mutex

		for id := range nodes {
			if id != myID {
				wg.Add(1)
				go func(targetID int) {
					defer wg.Done()
					fmt.Printf("[QUORUM] Replicating log to Storage Node %d...\n", targetID)
					res, err := sendMessage(targetID, Message{Type: "REPLICATE", Key: msg.Key, Value: msg.Value}, 2*time.Second)
					if err == nil && res != nil && res.Status == "OK" {
						mu.Lock()
						ackCount++
						mu.Unlock()
						fmt.Printf(ColorGreen + "[QUORUM] Node %d ACK received." + ColorReset + "\n", targetID)
					} else {
						fmt.Printf(ColorRed + "[QUORUM] Node %d is unreachable." + ColorReset + "\n", targetID)
					}
				}(id)
			}
		}
		
		wg.Wait()

		requiredQuorum := (len(nodes) / 2) + 1
		if ackCount >= requiredQuorum {
			fmt.Printf(ColorGreen + "[QUORUM] WRITE SUCCESS! Quorum achieved (%d/%d)." + ColorReset + "\n", ackCount, len(nodes))
			encoder.Encode(Message{Status: "SUCCESS", Msg: fmt.Sprintf("Write successful. Quorum %d/%d achieved.", ackCount, len(nodes))})
		} else {
			fmt.Printf(ColorRed + "[QUORUM] WRITE FAILED! Quorum NOT achieved (%d/%d)." + ColorReset + "\n", ackCount, len(nodes))
			encoder.Encode(Message{Status: "FAILED", Msg: fmt.Sprintf("Write failed. Only %d/%d achieved.", ackCount, len(nodes))})
		}

	case "READ":
		storageLock.RLock()
		val, exists := storage[msg.Key]
		storageLock.RUnlock()
		if exists {
			fmt.Printf(ColorCyan + "[READ] Serving Data for Key: %s" + ColorReset + "\n", msg.Key)
			encoder.Encode(Message{Status: "SUCCESS", Value: val})
		} else {
			encoder.Encode(Message{Status: "ERROR", Msg: "Key not found"})
		}

	case "PING":
		encoder.Encode(Message{Status: "OK"})
	}
}

func startServer(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		fmt.Printf(ColorRed + "Error starting server: %v" + ColorReset + "\n", err)
		os.Exit(1)
	}
	fmt.Printf(ColorGreen + "--- Amazon Aurora Simulation: Node %d Started ---" + ColorReset + "\n", myID)
	fmt.Printf("Status: Listening on port %d...\n", port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}

func checkLeader() {
	for {
		time.Sleep(3 * time.Second)
		if leaderID != -1 && leaderID != myID {
			res, err := sendMessage(leaderID, Message{Type: "PING"}, 1*time.Second)
			if err != nil || res == nil || res.Status != "OK" {
				fmt.Printf("\n" + ColorRed + "[FAILURE] Leader Node %d is unresponsive!" + ColorReset + "\n", leaderID)
				leaderID = -1
				startElection()
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: run_node.bat (and enter ID)")
		return
	}

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Invalid node ID")
		return
	}
	myID = id
	storage = make(map[string]string)

	loadConfig()
	loadStorage()
	
	info, ok := nodes[myID]
	if !ok {
		fmt.Printf("Node %d not in config\n", myID)
		return
	}

	go startServer(info.Port)
	
	time.Sleep(1 * time.Second)
	if leaderID == -1 {
		startElection()
	}

	checkLeader()
}
