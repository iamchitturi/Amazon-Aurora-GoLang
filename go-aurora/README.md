# Amazon Aurora Distributed Cluster Simulation (Go Version)

This is a Go implementation of the Amazon Aurora case study simulation, featuring:
1. **Bully Election Algorithm** for leader selection.
2. **Quorum-Based Replication** (Log-structured replication) for data consistency.
3. **Leader Health Monitoring** and automatic failover.

---

## Setup Instructions

### 1. Configure the Cluster
Edit `cluster_config.json` to include the IP addresses of the 4 laptops in your team.
- If testing on a single laptop, you can keep the IPs as `127.0.0.1` but ensure the ports are unique.
- If using multiple laptops, change `127.0.0.1` to the actual Local IP of each laptop (e.g., `192.168.1.5`).

### 2. How to Run (Direct Commands)

Open a terminal (CMD or PowerShell) and go to the specific sub-folder on each laptop:

**On Laptop 1:**
```bash
cd node_process
go run node.go 1
```

**On Laptop 2:**
```bash
cd node_process
go run node.go 2
```

**On Laptop 3:**
```bash
cd node_process
go run node.go 3
```

### 3. How to Run the Client
On any laptop, navigate to the client folder and start the simulator:
```bash
cd client_process
go run client.go
```

**Commands inside the client terminal:**
- `write <key> <value>`: Simulates a Quorum-based write.
- `read <key>`: Simulates a consistent read from the leader.
- `exit`: Close the simulator.

---

## Key Logic Details
- **Quorum**: Requires $(N/2)+1$ ACKs. In a 4-node cluster, 3 nodes must ACK the write for it to succeed.
- **Election**: Uses the **Bully algorithm**. If the leader fails, the node with the highest ID becomes the new leader.
- **Consistency**: High consistency (Read/Write Quorum principles).
- **Persistence**: Replicated log segments are stored in `storage_node_X.json` files for durability.
