@echo off
echo.
echo ===========================================
echo    Amazon Aurora Node Starter (GO)
echo ===========================================
echo.
set /p id="Enter Node ID for this laptop (e.g., 1, 2, 3, or 4): "
echo Starting Node %id%...
go run node.go %id%
pause
