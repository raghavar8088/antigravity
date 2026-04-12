# Claude Agent Setup for Antigravity Trading Application

## Overview
This guide shows how to create a Claude Agent that can monitor, analyze, and manage your Antigravity trading engine through the Claude API.

---

## 1. Agent Configuration

### What This Agent Can Do:
- Monitor real-time trading performance
- Execute upgrade commands on the engine
- Analyze trade history and statistics
- Manage positions and risk
- Query AI insights from the strategy service
- Execute docker commands for infrastructure
- Generate performance reports
- Trigger backtests

---

## 2. Creating the Agent via Claude Console

### Step-by-Step:

1. **Navigate to Claude Console**:
   - Go to: `https://platform.claude.com/workspaces/default/agent-quickstart`
   - Click "Create agent"

2. **Fill in Agent Details**:
   - **Agent Name**: `Antigravity Trading Manager`
   - **Description**: `Manages and monitors the Antigravity trading engine, executes upgrades, analyzes trades, and generates reports`
   - **Instructions**: See Section 3 below

3. **Configure Environment**:
   - Add environment variables (see Section 5)
   - Configure API endpoints
   - Set up authentication

4. **Add Tools**:
   - HTTP tools for API calls
   - Bash execution for commands
   - File operations for analysis

5. **Deploy**:
   - Click "Create Agent"
   - Note your Agent ID
   - Test with sample requests

---

## 3. Agent System Prompt (Instructions)

Copy this into the "Instructions" field when creating the agent:

```
You are the Antigravity Trading Engine Manager Agent. Your role is to:

1. **Monitor Trading Performance**
   - Check real-time trade statistics and P&L
   - Monitor open positions and risk levels
   - Track daily/weekly/monthly performance metrics

2. **Execute Upgrades & Maintenance**
   - Run upgrade commands on the Go engine
   - Update Node.js bridge dependencies
   - Refresh Python AI service packages
   - Pull and rebuild Docker infrastructure
   - Execute build scripts and compile binaries

3. **Analyze Trading Data**
   - Query recent trades and execution history
   - Generate performance reports
   - Calculate win rate, average profit, risk metrics
   - Identify profitable strategies and failing ones

4. **Risk Management**
   - Check current exposure and leverage
   - Monitor drawdown levels
   - Alert if risk thresholds exceeded
   - Manage position sizing

5. **Infrastructure Management**
   - Check engine, database, and cache health
   - View container logs and status
   - Restart services if needed
   - Monitor resource usage

6. **AI Strategy Analysis**
   - Query AI-generated trading insights
   - Review strategy recommendations
   - Validate strategy parameters
   - Get sentiment analysis

**Key Principles**:
- Always verify before executing critical commands
- Provide clear explanations of what actions you're taking
- Log all commands executed
- Ask for confirmation for destructive operations
- Monitor success of operations and report results

**Default Base URL**: http://localhost:8080 (or https://antigravity-x7he.onrender.com for production)

When a user asks you to perform a task:
1. Break it down into clear steps
2. Execute necessary API calls or commands
3. Report results with relevant data
4. Suggest next actions or improvements
```

---

## 4. Agent API Endpoints Configuration

### Production Endpoints:
```
Base URL: https://antigravity-x7he.onrender.com
```

### Local Development Endpoints:
```
Engine: http://localhost:8080
Dashboard: http://localhost:3000
Grafana: http://localhost:3001
Database: postgres://antigravity:password123@localhost:5432/antigravity
Redis: redis://localhost:6379
```

---

## 5. Environment Variables for Agent

When setting up the agent, configure these environment variables:

```
# API Configuration
API_BASE_URL=https://antigravity-x7he.onrender.com
LOCAL_API_URL=http://localhost:8080
DASHBOARD_URL=http://localhost:3000

# Database Configuration
DATABASE_URL=postgres://antigravity:password123@localhost:5432/antigravity
REDIS_URL=redis://localhost:6379

# AI Service
AI_GRPC_HOST=localhost:50051

# System Paths
WORKSPACE_PATH=/path/to/Trading\ apllication
ENGINE_PATH=/path/to/Trading\ apllication/engine
CLIENT_PATH=/path/to/Trading\ apllication/client

# Authentication (if needed)
API_KEY=<your-api-key>
AUTH_TOKEN=<your-auth-token>
```

---

## 6. API Endpoints the Agent Will Use

### Trading Status & Data
```
GET /api/health                    # Engine health check
GET /api/state                     # Current engine state
GET /api/trades?limit=50           # Trade history
GET /api/trades/stats              # Trade statistics
GET /api/positions                 # Open positions
GET /api/stats                     # Performance stats
```

### AI Insights
```
GET /api/ai/insights               # AI strategy recommendations
GET /api/nifty-stocks/strategies   # Available strategies
```

### Admin & Control
```
POST /api/admin/reset              # Reset engine state
GET /api/logs                      # Engine logs
POST /api/admin/killswitch         # Emergency stop
```

### Monitoring
```
GET /api/metrics                   # Prometheus metrics
GET /metrics                       # Metrics endpoint
```

---

## 7. Creating the Agent Programmatically (Python)

If you prefer to create the agent via code:

```python
import anthropic
import json

client = anthropic.Anthropic(api_key="your-api-key")

# Create the agent
response = client.agents.create(
    model="claude-opus-4-1",
    name="Antigravity Trading Manager",
    instructions="""You are the Antigravity Trading Engine Manager Agent...
    [Insert full system prompt from Section 3]
    """,
    tools=[
        {
            "name": "http_request",
            "description": "Make HTTP requests to Antigravity API",
            "input_schema": {
                "type": "object",
                "properties": {
                    "method": {"type": "string", "enum": ["GET", "POST", "PUT", "DELETE"]},
                    "url": {"type": "string"},
                    "body": {"type": "object"}
                },
                "required": ["method", "url"]
            }
        },
        {
            "name": "execute_command",
            "description": "Execute shell commands for upgrades and maintenance",
            "input_schema": {
                "type": "object",
                "properties": {
                    "command": {"type": "string"},
                    "cwd": {"type": "string"}
                },
                "required": ["command"]
            }
        }
    ]
)

print(f"Agent created with ID: {response.id}")
```

---

## 8. Creating the Agent via Claude Code (CLI)

```bash
# Install Claude Code
npm install -g @anthropic-ai/claude-code

# Create agent configuration
claude agent create \
  --name "Antigravity Trading Manager" \
  --description "Manages and monitors the Antigravity trading engine" \
  --instructions "$(cat agent-instructions.txt)" \
  --model claude-opus-4-1
```

---

## 9. Agent Configuration JSON

Save this as `agent-config.json`:

```json
{
  "name": "Antigravity Trading Manager",
  "description": "Manages and monitors the Antigravity trading engine, executes upgrades, analyzes trades, and generates reports",
  "model": "claude-opus-4-1",
  "instructions": "You are the Antigravity Trading Engine Manager Agent...",
  "tools": [
    {
      "type": "http",
      "name": "query_trades",
      "description": "Query trade history and statistics",
      "base_url": "http://localhost:8080",
      "endpoints": {
        "trades": "/api/trades",
        "stats": "/api/trades/stats",
        "positions": "/api/positions"
      }
    },
    {
      "type": "http",
      "name": "ai_insights",
      "description": "Get AI-generated trading insights",
      "base_url": "http://localhost:8080",
      "endpoints": {
        "insights": "/api/ai/insights",
        "strategies": "/api/nifty-stocks/strategies"
      }
    },
    {
      "type": "command",
      "name": "execute_upgrades",
      "description": "Execute upgrade commands",
      "allowed_commands": [
        "go mod tidy",
        "go get -u ./...",
        "go build",
        "npm update",
        "docker compose pull",
        "docker compose build"
      ]
    },
    {
      "type": "http",
      "name": "admin_control",
      "description": "Admin control endpoints",
      "base_url": "http://localhost:8080",
      "endpoints": {
        "reset": "/api/admin/reset",
        "logs": "/api/logs",
        "killswitch": "/api/admin/killswitch"
      }
    }
  ],
  "environment": {
    "API_BASE_URL": "http://localhost:8080",
    "WORKSPACE_PATH": "C:\\Trading apllication"
  }
}
```

---

## 10. Sample Agent Interactions

### Example 1: Check Trading Status
```
User: "What's the current trading status?"

Agent Response:
- Queries /api/state
- Gets /api/stats
- Retrieves /api/positions
- Summarizes: "Current balance: $50,000 | Open positions: 3 | 
  Daily P&L: +$1,240 | Win rate: 65% | Today's trades: 12"
```

### Example 2: Execute Engine Upgrade
```
User: "Upgrade the Go engine to the latest dependencies"

Agent Response:
1. Executes: cd engine && go mod tidy
2. Executes: go get -u ./...
3. Executes: go test ./...
4. Executes: go build -o ../bin/antigravity.exe cmd/antigravity/main.go
5. Reports: "Go engine upgraded successfully. All tests passed."
```

### Example 3: Generate Performance Report
```
User: "Generate a trading performance report for the last 7 days"

Agent Response:
1. Queries /api/trades with time filter
2. Calculates metrics (win rate, avg trade, max drawdown, etc.)
3. Analyzes by strategy
4. Returns formatted report with charts
```

### Example 4: Monitor Infrastructure
```
User: "Check if all services are running"

Agent Response:
1. Checks /api/health (Engine)
2. Checks http://localhost:3000 (Dashboard)
3. Checks http://localhost:3001 (Grafana)
4. Checks docker container status
5. Reports: "✅ All services operational"
```

---

## 11. Deploying the Agent

### Via Claude Console:
1. Go to `https://platform.claude.com/`
2. Navigate to "Managed Agents"
3. Click "Create Agent"
4. Fill in details from this guide
5. Click "Deploy"

### Via API:
```bash
curl -X POST https://api.anthropic.com/v1/agents \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "content-type: application/json" \
  -d @agent-config.json
```

### Via Claude Code CLI:
```bash
claude agent deploy --name "Antigravity Trading Manager" --config agent-config.json
```

---

## 12. Testing the Agent

### Test Query 1: Health Check
```
"Is the trading engine healthy?"
Expected: Agent checks /api/health and reports status
```

### Test Query 2: Recent Performance
```
"Show me the last 10 trades with profit/loss"
Expected: Agent queries /api/trades?limit=10 and formats results
```

### Test Query 3: Upgrade Request
```
"Can you upgrade the Next.js client?"
Expected: Agent asks for confirmation, then runs: cd client && npm update && npm run build
```

### Test Query 4: Risk Analysis
```
"What's my current exposure and drawdown?"
Expected: Agent queries positions and stats, calculates risk metrics
```

---

## 13. Integration with Your Application

### Adding Agent API Endpoint to Engine

In `engine/cmd/antigravity/main.go`, add:

```go
// Agent management endpoint
http.HandleFunc("/api/agent/status", func(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "agent_id": "antigravity-manager-v1",
        "status": "operational",
        "engine_status": "running",
        "last_check": time.Now(),
    }
    json.NewEncoder(w).Encode(status)
})

// Agent command execution endpoint (protected)
http.HandleFunc("/api/agent/execute", func(w http.ResponseWriter, r *http.Request) {
    // Verify agent authentication
    // Execute command safely
    // Log execution
})
```

---

## 14. Security Considerations

### Authentication
```go
// Add API key validation
func validateAgentKey(apiKey string) bool {
    return apiKey == os.Getenv("AGENT_API_KEY")
}

// Add to protected endpoints
if !validateAgentKey(r.Header.Get("X-Agent-Key")) {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

### Rate Limiting
```go
// Implement rate limiting for agent queries
limiter := rate.NewLimiter(rate.Limit(100), 10) // 100 req/sec, burst 10

if !limiter.Allow() {
    http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
}
```

### Command Whitelist
```go
// Only allow specific commands
allowedCommands := []string{
    "go mod tidy",
    "go get -u ./...",
    "npm update",
    "docker compose pull",
}

func isCommandAllowed(cmd string) bool {
    for _, allowed := range allowedCommands {
        if cmd == allowed {
            return true
        }
    }
    return false
}
```

---

## 15. Monitoring Agent Activity

### Log Agent Requests
```go
// Add agent logging
http.HandleFunc("/api/agent/logs", func(w http.ResponseWriter, r *http.Request) {
    logs := []string{
        "2026-04-12 10:30:15 | Agent queried /api/trades",
        "2026-04-12 10:30:20 | Agent executed: go mod tidy",
        "2026-04-12 10:30:45 | Agent upgraded dependencies",
        "2026-04-12 10:31:00 | Agent generated performance report",
    }
    json.NewEncoder(w).Encode(logs)
})
```

---

## 16. Advanced Features

### Scheduled Tasks
The agent can trigger scheduled actions:
```
Agent Task: "Upgrade engine every Sunday at 2 AM"
Action: Agent creates scheduled job to run upgrades
Benefit: Automatic maintenance without manual intervention
```

### Custom Analysis
```
Agent Query: "Which strategy is most profitable?"
Agent Response: Analyzes all trades by strategy, returns insights
Benefit: Data-driven strategy selection
```

### Automated Alerts
```
Agent Monitor: Track drawdown levels
Agent Action: Alert if drawdown > 10%
Agent Action: Suggest risk adjustment
```

---

## 17. Quick Start Steps

1. **Create Agent**:
   - Go to Claude Console
   - Click "Create agent"
   - Use configuration from this guide
   - Deploy

2. **Configure Endpoints**:
   - Set `API_BASE_URL` to your engine URL
   - Add environment variables

3. **Test Agent**:
   - Send test queries
   - Verify API responses
   - Check command execution

4. **Deploy to Production**:
   - Update `API_BASE_URL` to production domain
   - Set secure API keys
   - Enable authentication
   - Monitor agent activity

5. **Integrate with Dashboard**:
   - Add agent chat widget to dashboard
   - Allow users to query agent
   - Display agent recommendations

---

## 18. Troubleshooting

### Agent Can't Connect to API
```
Error: Connection refused
Solution: 
- Verify engine is running
- Check API_BASE_URL is correct
- Check firewall rules
- Test with: curl http://localhost:8080/api/health
```

### Agent Timeouts
```
Error: Request timeout
Solution:
- Increase timeout in agent config
- Optimize slow API endpoints
- Check database performance
```

### Command Execution Fails
```
Error: Command not found
Solution:
- Verify command is in whitelist
- Check working directory
- Ensure permissions are set
- Test command manually first
```

---

**Last Updated**: April 12, 2026
**Version**: 1.0
**Status**: Ready to Deploy

---

## Next Steps:

1. ✅ Review this guide
2. ✅ Go to https://platform.claude.com/workspaces/default/agent-quickstart
3. ✅ Fill in agent configuration
4. ✅ Create and test agent
5. ✅ Deploy to production
6. ✅ Monitor agent activity
