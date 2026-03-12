# Case Study: Distributed Database Architecture (Amazon Aurora Simulation)

**Project Title:** Amazon Aurora Decoupled Storage & Compute Simulation
**Language:** Go (Golang)
**Concept:** Decentralized Leader Election & Quorum-Based Replication

---

## 1. Problem Understanding (Rubric 1)
Traditional relational databases (like standard MySQL) couple compute and storage. In a distributed environment, this model struggles with:
- **I/O Bottlenecks**: Writing full database pages to storage is slow.
- **Availability**: If the primary node crashes, data might be lost or unavailable during recovery.

**Amazon Aurora** solves this by:
1.  **Decoupling**: Separating the database engine (Compute) from the storage layer.
2.  **Log-Is-The-Database**: Only sending "redo log segments" over the network instead of full pages.

---

## 2. Architecture Design (Rubric 2)
Our simulation implements a 4-node cluster with the following distributed logic:

### A. Election Algorithm (Unit 2)
We use the **Bully Election Algorithm**. 
- When a node detects the Leader is down, it sends an `ELECTION` message to higher-ID nodes.
- If no higher nodes respond, it declares itself `COORDINATOR`.
- Ensures a single "Source of Truth" for compute operations.

### B. Consistency & Replication (Unit 3)
We use **Quorum-Based Replication**.
- Total Nodes (N) = 4.
- Write Quorum (W) = 3 (Majority: N/2 + 1).
- This ensures that even if one node fails, the data remains consistent and available across the majority.

---

## 3. Implementation Details (Rubric 3)
The system is built in Go for high-concurrency:
- **Goroutines**: Each network request and background health check runs in its own lightweight thread.
- **Persistence**: Every storage node writes replicated segments to a local JSON file (`storage_node_X.json`), simulating Aurora’s durable storage layer.
- **ANSI Colors**: Terminal output is color-coded for visual clarity during presentation (Green for Success, Red for Failure, Yellow for Elections).

---

## 4. Presentation & Q&A Scenarios (Rubric 5)

| Scenario | Expected Observation |
| :--- | :--- |
| **Normal Write** | Leader sends logs to 3/4 nodes. ACKs are received. Write succeed. |
| **Leader Crash** | Pings fail -> Bully Election starts -> New Leader elected -> Client auto-discovers new leader. |
| **Node Recovery** | Node restarts -> Loads JSON from disk -> Automatically rejoins the cluster data state. |
| **Network Partition** | If nodes cannot reach a majority, the system denies writes to prevent "Split-Brain" inconsistency. |

---

## 5. How to Run for the Professor
1.  **Start Nodes**: Run `run_node.bat` on each system (ID 1, 2, 3, 4).
2.  **Start Client**: Run `run_client.bat`.
3.  **Command**: `write session_id 101` -> Observe replication.
4.  **Command**: `read session_id` -> Observe consistent read.
