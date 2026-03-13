@echo off
echo =======================================================
echo     Amazon Aurora Simulation Cluster (4 Nodes)
echo =======================================================
echo Starting 4 independent systems using localhost...
echo.

start cmd /k "title Node 1 (Replica) & color 0A & python node.py 1"
timeout /t 1 /nobreak >nul
start cmd /k "title Node 2 (Replica) & color 0B & python node.py 2"
timeout /t 1 /nobreak >nul
start cmd /k "title Node 3 (Replica) & color 0C & python node.py 3"
timeout /t 1 /nobreak >nul
start cmd /k "title Node 4 (Leader / Primary) & color 0D & python node.py 4"

echo All 4 nodes are booting up!
echo Waiting a few seconds for leader election (Bully Algorithm) to complete...
timeout /t 5 /nobreak >nul

echo Starting the Client Simulator GUI...
start cmd /k "title Aurora Client & color 0F & python client.py"

echo.
echo =======================================================
echo   Cluster is running! Check the new command windows.
echo =======================================================
echo.
echo To simulate failure for the presentation:
echo 1. Open "Node 4" window and close it (this kills the Leader).
echo 2. Watch "Node 3" window start the Election and become new Leader!
echo 3. Write data from client again and see it handled by Node 3.
echo.
pause
