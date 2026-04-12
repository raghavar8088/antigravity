# Step-by-Step: Create Agent in Claude Console

## Access Claude Console

1. Open your browser and navigate to:
   ```
   https://platform.claude.com/workspaces/default/agent-quickstart
   ```

2. You should see:
   - Left sidebar with "Quickstart", "Agents", "Sessions", etc.
   - Center panel showing "Create agent" button
   - "Browse templates" section
   - Form: "What do you want to build?"

---

## Step 1: Click "Create agent"

In the center of the page, you'll see a button or section labeled **"Create agent"**.

Click it to start creating your agent.

---

## Step 2: Fill in Agent Name

**Field**: Agent Name (or Name)

**Value**:
```
Antigravity Trading Manager
```

This is what your agent will be called.

---

## Step 3: Add Agent Description

**Field**: Description

**Value**:
```
Manages and monitors the Antigravity trading engine, executes upgrades, analyzes trades, and generates reports. Can query trading performance, execute system commands, manage infrastructure, and provide AI-powered trading insights.
```

This describes what the agent does.

---

## Step 4: Select Model

**Field**: Model

**Options** (select the latest):
- `claude-opus-4-1` (recommended - most capable)
- `claude-sonnet-4-1` (faster, cheaper)
- `claude-haiku-4-1` (fastest, cheapest)

**Recommendation**: Select `claude-opus-4-1` for best performance on complex trading tasks.

---

## Step 5: Add Agent Instructions (System Prompt)

**Field**: Instructions (or System Prompt)

**Copy and paste the entire text below**:

```
You are the Antigravity Trading Engine Manager Agent. Your primary role is to manage, monitor, and optimize the Antigravity trading application.

CORE RESPONSIBILITIES:

1. **Trading Performance Monitoring**
   - Query and report on current trading state (balance, open positions, P&L)
   - Fetch recent trade history and statistics
   - Calculate and analyze performance metrics (win rate, average profit, Sharpe ratio, max drawdown)
   - Identify trends and patterns in trading performance
   - Alert on significant changes or anomalies

2. **System Upgrades & Maintenance**
   - Execute Go engine upgrades (go mod tidy, go get -u, go build)
   - Update Next.js client dependencies (npm update, npm build)
   - Refresh Node.js bridge packages
   - Upgrade Python AI service requirements
   - Pull and rebuild Docker containers
   - Run full system builds with scripts/build.bat

3. **Data Analysis & Reporting**
   - Analyze trade data and generate performance reports
   - Break down performance by strategy, timeframe, instrument
   - Calculate risk metrics and exposure levels
   - Create visual summaries of trading results
   - Identify profitable vs unprofitable strategies

4. **Risk Management**
   - Monitor current exposure and leverage
   - Track drawdown levels and alert on threshold breaches
   - Query position sizes and risk parameters
   - Recommend position sizing adjustments
   - Flag excessive concentration in single instruments

5. **Infrastructure & Health Checks**
   - Check engine health and status
   - Verify database and cache connectivity
   - Monitor Docker container status
   - View and analyze error logs
   - Report on system resource usage

6. **AI & Strategy Insights**
   - Query AI-generated trading recommendations
   - Fetch available strategy parameters
   - Analyze strategy performance
   - Get sentiment analysis from market data
   - Validate strategy selections

OPERATIONAL PRINCIPLES:

✓ Be proactive: Anticipate issues and provide insights without being asked
✓ Be thorough: Provide complete context and supporting data with answers
✓ Be clear: Explain what you're doing and why in plain language
✓ Be cautious: Always ask for confirmation before executing critical commands (upgrades, resets, killswitch)
✓ Be professional: Maintain professional tone, provide structured reports
✓ Be logged: Track all operations and report execution success/failure
✓ Be helpful: Suggest improvements and next steps based on data

COMMAND EXECUTION GUIDELINES:

- When asked to execute a command, break it into steps
- Execute upgrade commands only after user confirmation
- Report on build success/failure with relevant error messages
- Suggest rollback or troubleshooting if commands fail
- Never execute destructive commands without explicit user approval

API ENDPOINTS AVAILABLE:

Health & Status:
  GET /api/health                    # Engine health
  GET /api/state                     # Engine state
  GET /api/logs                      # Engine logs

Trading Data:
  GET /api/trades?limit=N            # Trade history
  GET /api/trades/stats              # Trade statistics
  GET /api/positions                 # Open positions
  GET /api/stats                     # Performance stats

AI & Strategy:
  GET /api/ai/insights               # AI recommendations
  GET /api/nifty-stocks/strategies   # Available strategies

Admin:
  POST /api/admin/reset              # Reset engine (requires confirmation)
  POST /api/admin/killswitch         # Emergency stop (requires confirmation)

RESPONSE FORMATTING:

- For data queries: Present results in clear, organized format
- For reports: Use structured sections with headings
- For metrics: Show numbers with context and interpretation
- For alerts: Highlight important findings clearly
- For recommendations: Provide actionable next steps

EXAMPLE INTERACTIONS:

User: "Check the current trading performance"
→ Query /api/state, /api/stats, /api/positions
→ Report: Current P&L, open trades, risk metrics

User: "Upgrade the Go engine"
→ Ask for confirmation
→ Execute: go mod tidy, go get -u, go build
→ Report: Build success, run tests, verify binary

User: "Generate a daily performance report"
→ Query /api/trades/stats for today
→ Calculate win rate, average trade, max loss
→ Create formatted report with charts/summaries

User: "What's the risk level?"
→ Query /api/positions and /api/stats
→ Calculate current drawdown, exposure, leverage
→ Alert if any thresholds breached
→ Recommend adjustments if needed

Default base URL: http://localhost:8080 (development) or https://antigravity-x7he.onrender.com (production)

You are empowered to help the user manage their trading engine effectively. Use all available tools and data to provide the best insights and assistance.
```

---

## Step 6: Configure Tools

In the "Tools" section (if available), configure the following:

### Tool 1: HTTP Requests (API Queries)
- **Name**: `query_trading_api`
- **Type**: HTTP/REST
- **Base URL**: `http://localhost:8080`
- **Description**: Query trading data and engine status

### Tool 2: System Commands (Upgrades)
- **Name**: `execute_upgrades`
- **Type**: Bash/Command
- **Allowed Commands**:
  ```
  go mod tidy
  go get -u ./...
  go test ./...
  go build
  npm update
  npm run build
  docker compose pull
  docker compose build
  ```
- **Description**: Execute upgrade and build commands

### Tool 3: Admin Operations
- **Name**: `admin_control`
- **Type**: HTTP/REST
- **Base URL**: `http://localhost:8080`
- **Endpoints**:
  ```
  POST /api/admin/reset
  POST /api/admin/killswitch
  GET /api/logs
  ```
- **Description**: Administrative control endpoints

---

## Step 7: Set Environment Variables

In the "Environment" or "Settings" section:

```json
{
  "API_BASE_URL": "http://localhost:8080",
  "PRODUCTION_API_URL": "https://antigravity-x7he.onrender.com",
  "DATABASE_URL": "postgres://antigravity:password123@localhost:5432/antigravity",
  "REDIS_URL": "redis://localhost:6379",
  "WORKSPACE_PATH": "C:\\Trading apllication",
  "LOG_LEVEL": "info"
}
```

---

## Step 8: Review Configuration

Before creating, review:

- ✅ Agent Name: "Antigravity Trading Manager"
- ✅ Model: Selected (claude-opus-4-1 recommended)
- ✅ Instructions: Pasted and complete
- ✅ Tools: Configured for API and commands
- ✅ Environment: Variables set

---

## Step 9: Create Agent

Click the **"Create Agent"** or **"Deploy"** button.

The system will:
1. Validate configuration
2. Create the agent
3. Assign an Agent ID
4. Display success message

**Save your Agent ID** for later use.

---

## Step 10: Test Your Agent

Once created, you'll see a test interface. Try these sample queries:

### Test 1: Check Status
```
Query: "What's the current trading status?"
Expected: Agent queries /api/state and /api/stats, returns summary
```

### Test 2: Get Recent Trades
```
Query: "Show me the last 10 trades"
Expected: Agent queries /api/trades?limit=10, displays trade details
```

### Test 3: Performance Report
```
Query: "Generate a performance report for today"
Expected: Agent calculates metrics and returns formatted report
```

### Test 4: Infrastructure Check
```
Query: "Is the infrastructure healthy?"
Expected: Agent checks health endpoints and reports status
```

### Test 5: Upgrade (with confirmation)
```
Query: "Can you upgrade the Go engine dependencies?"
Expected: Agent asks for confirmation, then executes upgrade sequence
```

---

## Step 11: Access Your Agent

After creation, access it via:

1. **Console Interface**:
   ```
   https://platform.claude.com/workspaces/default/agents
   ```
   Look for "Antigravity Trading Manager" in the list

2. **API Endpoint**:
   ```
   GET https://api.anthropic.com/v1/agents/{agent_id}
   ```

3. **Web Chat**:
   ```
   https://platform.claude.com/agents/{agent_id}
   ```

---

## Step 12: Get Your Agent ID

Once created, you'll see:

```
Agent Created Successfully
Agent ID: agent_abc123def456...
Workspace: default
Model: claude-opus-4-1
Status: Active
```

Save this Agent ID.

---

## Step 13: Integrate with Dashboard

To add the agent to your Antigravity dashboard:

### Option A: Add Agent Chat Widget
```html
<div id="agent-chat">
  <!-- Add agent chat widget code here -->
  <script>
    const AGENT_ID = "agent_abc123def456...";
    const API_KEY = "sk-..."; // Your Anthropic API key
    
    // Initialize agent chat
    initializeAgent(AGENT_ID, API_KEY);
  </script>
</div>
```

### Option B: Create Dedicated Agent Page
```
Dashboard Routes:
- /dashboard/home (existing)
- /dashboard/trades (existing)
- /dashboard/agent (NEW) - Agent chat interface
- /dashboard/settings (existing)
```

---

## Step 14: Deploy to Production

1. Create another agent with:
   - **Name**: "Antigravity Trading Manager (Production)"
   - **API_BASE_URL**: `https://antigravity-x7he.onrender.com`

2. Update environment:
   ```json
   {
     "API_BASE_URL": "https://antigravity-x7he.onrender.com",
     "ENVIRONMENT": "production"
   }
   ```

3. Deploy and test

---

## Step 15: Monitor Agent Activity

Check agent performance:
- View conversation logs
- Monitor API response times
- Track command execution history
- Review any errors or failures

In Console:
```
Managed Agents → Antigravity Trading Manager → Activity/Logs
```

---

## Troubleshooting

### Agent Can't Connect to API
**Problem**: "Connection refused" or "Cannot reach endpoint"

**Solution**:
1. Verify engine is running: `curl http://localhost:8080/api/health`
2. Check API_BASE_URL is correct
3. Check firewall rules
4. Verify DNS resolution

### Agent Timeouts
**Problem**: "Request timeout" messages

**Solution**:
1. Increase timeout in agent configuration
2. Optimize slow API endpoints
3. Check database performance
4. Review server logs

### Command Execution Fails
**Problem**: "Command not found" or "Permission denied"

**Solution**:
1. Verify command is in whitelist
2. Check working directory
3. Test command manually first
4. Check file permissions

---

## Quick Reference

| Step | Action | Value |
|------|--------|-------|
| 1 | Agent Name | Antigravity Trading Manager |
| 2 | Model | claude-opus-4-1 |
| 3 | Base URL | http://localhost:8080 |
| 4 | Instructions | See Step 5 |
| 5 | Tools | HTTP + Commands |
| 6 | Create | Click Deploy Button |
| 7 | Test | Use sample queries |
| 8 | Integrate | Add to dashboard |
| 9 | Monitor | View activity logs |

---

## Next Steps

1. ✅ Create agent in Claude Console
2. ✅ Test with sample queries
3. ✅ Verify API connectivity
4. ✅ Add to dashboard
5. ✅ Deploy to production
6. ✅ Monitor performance
7. ✅ Gather user feedback
8. ✅ Iterate and improve

---

**Status**: Ready to Create
**Last Updated**: April 12, 2026
**Version**: 1.0
