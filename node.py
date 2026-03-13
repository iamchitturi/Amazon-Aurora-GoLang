import socket
import threading
import sys
import time
import json

# Load cluster configuration
config_file = "cluster_config.json"
try:
    with open(config_file, 'r') as f:
        config = json.load(f)
        NODES = config["nodes"]
except FileNotFoundError:
    print(f"Error: {config_file} not found. Please create it with node mappings.")
    sys.exit(1)

my_id = int(sys.argv[1])
if str(my_id) not in NODES:
    print(f"Error: Node ID {my_id} not defined in {config_file}")
    sys.exit(1)

my_ip = NODES[str(my_id)]["ip"]
my_port = NODES[str(my_id)]["port"]

leader_id = None
storage = {}
election_in_progress = False

def send_message(node_id, message, timeout=2):
    try:
        target_ip = NODES[str(node_id)]["ip"]
        target_port = NODES[str(node_id)]["port"]
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(timeout)
        s.connect((target_ip, target_port))
        s.sendall(json.dumps(message).encode('utf-8'))
        response = s.recv(1024)
        s.close()
        if response:
            return json.loads(response.decode('utf-8'))
    except:
        return None

def election():
    global leader_id, election_in_progress
    if election_in_progress:
        return
    election_in_progress = True
    print(f"\n[ELECTION] Node {my_id} starting election (Bully Algorithm)...")
    higher_nodes = [int(n) for n in NODES if int(n) > my_id]
    
    responses = 0
    for node in higher_nodes:
        print(f"[ELECTION] Sending ELECTION message to Node {node}...")
        res = send_message(node, {"type": "ELECTION"}, timeout=1)
        if res and res.get("status") == "OK":
            responses += 1
            
    if responses == 0:
        print(f"[ELECTION] No response from higher nodes. I am the new LEADER!")
        leader_id = my_id
        for node in NODES:
            if int(node) != my_id:
                send_message(int(node), {"type": "COORDINATOR", "leader_id": my_id})
    else:
        print(f"[ELECTION] Higher node responded. Waiting for COORDINATOR message...")
    
    election_in_progress = False

def handle_client(conn, addr):
    global leader_id, storage
    try:
        data = conn.recv(1024)
        if not data:
            return
        msg = json.loads(data.decode('utf-8'))
        
        if msg["type"] == "ELECTION":
            conn.sendall(json.dumps({"status": "OK"}).encode('utf-8'))
            # If someone with lower id starts election, we should also start election
            threading.Thread(target=election).start()
            
        elif msg["type"] == "COORDINATOR":
            leader_id = msg["leader_id"]
            print(f"\n[LEADER] Node {leader_id} is the new leader.")
            conn.sendall(json.dumps({"status": "OK"}).encode('utf-8'))
            
        elif msg["type"] == "REPLICATE":
            key = msg["key"]
            value = msg["value"]
            storage[key] = value
            print(f"\n[STORAGE] Replicated data from Leader: {key}={value}")
            conn.sendall(json.dumps({"status": "OK"}).encode('utf-8'))
            
        elif msg["type"] == "WRITE":
            if leader_id != my_id:
                conn.sendall(json.dumps({"status": "ERROR", "msg": f"Not leader. Leader is {leader_id}"}).encode('utf-8'))
                return
                
            key = msg["key"]
            value = msg["value"]
            print(f"\n[WRITE] Client write request received: {key}={value}. Initiating QUORUM Replication...")
            storage[key] = value
            ack_count = 1 # Counts self
            
            for node in NODES:
                if int(node) != my_id:
                    print(f"[QUORUM] Replicating to Node {node}...")
                    res = send_message(int(node), {"type": "REPLICATE", "key": key, "value": value})
                    if res and res.get("status") == "OK":
                        ack_count += 1
                        
            # Quorum requires majority: (N // 2) + 1
            required_quorum = (len(NODES) // 2) + 1
            if ack_count >= required_quorum:
                print(f"[QUORUM] Write SUCCESS! Quorum achieved ({ack_count}/{len(NODES)} ACKs)")
                conn.sendall(json.dumps({"status": "SUCCESS", "message": f"Write successful. Quorum {ack_count}/{len(NODES)} achieved."}).encode('utf-8'))
            else:
                print(f"[QUORUM] Write FAILED! Quorum NOT achieved ({ack_count}/{len(NODES)} ACKs)")
                conn.sendall(json.dumps({"status": "FAILED", "message": f"Write failed. Only {ack_count}/{len(NODES)} achieved."}).encode('utf-8'))

        elif msg["type"] == "PING":
            conn.sendall(json.dumps({"status": "OK"}).encode('utf-8'))

    except Exception as e:
        print(f"Error handling request: {e}")
    finally:
        conn.close()


def start_server():
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    # Bind to 0.0.0.0 to accept connections from other laptops on the network
    server.bind(('0.0.0.0', my_port))
    server.listen(5)
    print(f"--- Node {my_id} Database Engine Started on {my_ip}:{my_port} ---")
    
    while True:
        try:
            conn, addr = server.accept()
            threading.Thread(target=handle_client, args=(conn, addr)).start()
        except Exception as e:
            print(f"Server error: {e}")

def check_leader():
    global leader_id
    while True:
        time.sleep(3)
        current_leader = leader_id
        if current_leader is not None and current_leader != my_id:
            res = send_message(current_leader, {"type": "PING"})
            # Check if leader_id hasn't changed to someone else (or ourselves) while pinging
            if not res and leader_id == current_leader:
                print(f"\n[FAILURE] Leader Node {current_leader} did not respond! Assuming failed.")
                leader_id = None
                election()

if __name__ == "__main__":
    # Start the server to listen for messages
    threading.Thread(target=start_server, daemon=True).start()
    
    # Wait for other nodes to potentially start
    time.sleep(2)
    
    # Initiate election if no leader was announced
    if leader_id is None:
        election()
        
    # Start thread to ping leader and check failure
    check_leader()
