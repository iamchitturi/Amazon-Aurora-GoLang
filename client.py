import socket
import json
import sys

# Load cluster configuration
config_file = "cluster_config.json"
try:
    with open(config_file, 'r') as f:
        config = json.load(f)
        NODES = config["nodes"]
except FileNotFoundError:
    print(f"Error: {config_file} not found. Please create it first.")
    sys.exit(1)

def send_request(node_id, message):
    try:
        if str(node_id) not in NODES:
            return None
        target_ip = NODES[str(node_id)]["ip"]
        target_port = NODES[str(node_id)]["port"]
        
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(2)
        s.connect((target_ip, target_port))
        s.sendall(json.dumps(message).encode('utf-8'))
        response = s.recv(1024)
        s.close()
        if response:
            return json.loads(response.decode('utf-8'))
    except Exception as e:
        return None

if __name__ == "__main__":
    print("===================================================")
    print("      Amazon Aurora Client Simulator Interface")
    print("===================================================\n")
    leader_id = max([int(n) for n in NODES]) # Bully algorithm default winner
    
    print("Commands:")
    print("  write <key> <value>   - Send write request to the leader (Simulates Quorum write)")
    print("  exit                  - Close client\n")
    
    while True:
        try:
            cmd_input_raw = input(f"[Client -> Leader Node {leader_id}]> ").strip()
            if not cmd_input_raw:
                continue
                
            cmd_input = cmd_input_raw.split()
            if cmd_input[0] == "exit":
                break
            
            if cmd_input[0] == "write":
                if len(cmd_input) != 3:
                    print("Error: Invalid syntax. Usage: write <key> <value>")
                    continue
                    
                key, value = cmd_input[1], cmd_input[2]
                
                print(f"Action: Sending WRITE request [{key}={value}] to Leader Node {leader_id}...")
                res = send_request(leader_id, {"type": "WRITE", "key": key, "value": value})
                
                # Check if leader has changed
                if res and res.get("status") == "ERROR" and "Leader is" in str(res.get("msg", "")):
                    new_id_str = str(res["msg"]).split()[-1]
                    if new_id_str.isdigit():
                        leader_id = int(new_id_str)
                        print(f"Info: Server responded that leader has changed to Node {leader_id}.")
                        print(f"Action: Redirecting WRITE request to new Leader Node {leader_id}...")
                        res = send_request(leader_id, {"type": "WRITE", "key": key, "value": value})
                
                # If leader crashed, we auto-discover to keep system fault-tolerant
                if not res:
                    print(f"Error: No response. Cluster Coordinator (Leader Node {leader_id}) seems to have crashed.")
                    print("Action: Attempting to auto-discover new leader...")
                    
                    for i in [int(n) for n in sorted(NODES.keys(), reverse=True)]:
                        if i == leader_id:
                            continue
                        
                        check_res = send_request(i, {"type": "WRITE", "key": key, "value": value})
                        
                        if check_res:
                            if check_res.get("status") == "ERROR" and "Leader is" in str(check_res.get("msg", "")):
                                new_id = str(check_res["msg"]).split()[-1]
                                if new_id.isdigit():
                                    leader_id = int(new_id)
                                    print(f"Success: Discovered new leader Node {leader_id}. Resending request...")
                                    res = send_request(leader_id, {"type": "WRITE", "key": key, "value": value})
                                    break
                            elif check_res.get("status") == "SUCCESS":
                                print(f"Success: Randomly found new leader Node {i} and executed command.")
                                leader_id = i
                                res = check_res
                                break
                                
                    if not res:
                        print("Critical Error: Failed to find a new leader. Cluster might be entirely down.")
                        
                if res:
                    print(f"Cluster Response: {json.dumps(res, indent=2)}")
                    
            else:
                print("Unknown command. Supported: 'write <key> <value>' or 'exit'.")
        except KeyboardInterrupt:
            break
        except Exception as e:
            print(f"Unexpected Client Error: {e}")
            
    print("Exiting Client Simulator...")
