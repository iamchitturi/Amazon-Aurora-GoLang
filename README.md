# Amazon Aurora Distributed System Case Study

**Topic:** Amazon Aurora Distributed Database Simulation
**Algorithms Implemented:**
1. **Bully Election Algorithm** (Unit 2: Leader Election)
2. **Quorum Based Replication Voting Protocol** (Unit 3: Fault Tolerance / Replica Management)

## System Architecture
This simulation creates a decentralized cluster of 4 nodes running on your local machine using socket programming (Unit 1: Message Passing Systems). 

* **Node 4** acts as the initial Primary/Coordinator engine.
* **Nodes 1, 2, and 3** act as Replicas across different simulated Availability Zones.
* **Client Simulator** connects to the Coordinator to write data.

## How to Run the Simulation
1. Double-click the `run_cluster.bat` file.
2. Five Command Prompt windows will open:
   * 4 Node Windows (Ports 5001 - 5004)
   * 1 Client Window
3. Wait a few seconds for Node 4 to run the Bully Algorithm and establish itself as the Leader.

## How to Test the Algorithms

### 1. Test Quorum Replication (Normal Operation)
In the **Aurora Client Window**, enter a write command:
```bash
write username amrita
```
* **What happens:** The Client sends this request to the Leader (Node 4). Node 4 replicates this data to all other nodes. The write is only confirmed as "SUCCESS" to the client once at least 3 out of 4 nodes (a Quorum) reply with an ACK.

### 2. Test Fault Tolerance (Leader Crashes)
1. Forcefully close the **Node 4** window (Click the 'X').
* **What happens:** The background ping threads in the other nodes will detect the timeout. Nodes will immediately initiate the **Bully Election Algorithm**. Node 3 (having the highest remaining ID) will win the election and become the new Leader.

### 3. Test Client Auto-Discovery
In the **Aurora Client Window**, enter another write command:
```bash
write password secure123
```
* **What happens:** The Client will realize Node 4 is dead. It will automatically scan the cluster, discover that Node 3 is the new Leader, redirect the request, and successfully achieve Quorum replication.
