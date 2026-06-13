# systemd Service — BTC Trading Engine

## Installation

```bash
# 1. Copy service unit file
sudo cp btc-engine.service /etc/systemd/system/

# 2. Install the pre-start health check script
sudo mkdir -p /home/ubuntu/btc-pilot/scripts
sudo cp health-check-pre-start.sh /home/ubuntu/btc-pilot/scripts/
sudo chmod +x /home/ubuntu/btc-pilot/scripts/health-check-pre-start.sh

# 3. Reload systemd and enable on boot
sudo systemctl daemon-reload
sudo systemctl enable btc-engine

# 4. Start the service
sudo systemctl start btc-engine
```

## Verification

```bash
# Check service status
sudo systemctl status btc-engine

# Follow live logs
sudo journalctl -u btc-engine -f

# View last 100 lines of logs
sudo journalctl -u btc-engine -n 100 --no-pager
```

## Restart Behaviour

| Exit Code | Meaning | Restarted? |
|-----------|---------|-----------|
| 0 | Clean / intentional shutdown | No |
| 2 | Kill switch activated | No |
| 1 | Crash / unexpected error | Yes (after 5s) |
| any other | Crash | Yes (after 5s) |

The engine sends `SIGTERM` on `systemctl stop`. The 30-second `TimeoutStopSec`
gives the engine time to close open positions gracefully before `SIGKILL` fires.

## Manual Commands

```bash
# Stop (graceful — sends SIGTERM first)
sudo systemctl stop btc-engine

# Force-kill immediately
sudo systemctl kill -s SIGKILL btc-engine

# Restart
sudo systemctl restart btc-engine

# Disable auto-start on boot
sudo systemctl disable btc-engine
```
