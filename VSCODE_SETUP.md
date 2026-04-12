# VS Code Setup Guide for Antigravity Trading Engine

## Overview
This guide shows you how to configure VS Code for development on the Antigravity project with proper extensions, tasks, and debugging setup.

---

## 1. Essential Extensions to Install

### In VS Code, open Extensions (Ctrl+Shift+X) and search for:

#### Go Development
```
Name: Go
Publisher: Go Team at Google
ID: golang.Go
Latest Version
```
Command: `code --install-extension golang.Go`

#### Python Development
```
Name: Python
Publisher: Microsoft
ID: ms-python.python
Latest Version
```
Command: `code --install-extension ms-python.python`

#### Pylance (Python Language Server)
```
Name: Pylance
Publisher: Microsoft
ID: ms-python.vscode-pylance
Latest Version
```
Command: `code --install-extension ms-python.vscode-pylance`

#### JavaScript/TypeScript & React
```
Name: ES7+ React/Redux/React-Native snippets
Publisher: dsznajder
ID: dsznajder.es7-react-js-snippets
```
Command: `code --install-extension dsznajder.es7-react-js-snippets`

#### TypeScript
```
Name: TypeScript Vue Plugin (Volar)
Publisher: Vue
ID: Vue.vscode-typescript-vue-plugin
```
Command: `code --install-extension Vue.vscode-typescript-vue-plugin`

#### Docker Support
```
Name: Docker
Publisher: Microsoft
ID: ms-azuretools.vscode-docker
```
Command: `code --install-extension ms-azuretools.vscode-docker`

#### Database Tools
```
Name: PostgreSQL
Publisher: Chris Kolkman
ID: ckolkman.vscode-postgres
```
Command: `code --install-extension ckolkman.vscode-postgres`

#### Git & Version Control
```
Name: GitLens
Publisher: GitKraken
ID: eamodio.gitlens
```
Command: `code --install-extension eamodio.gitlens`

#### REST API Testing
```
Name: REST Client
Publisher: Huachao Mao
ID: humao.rest-client
```
Command: `code --install-extension humao.rest-client`

#### Linting & Formatting
```
Name: ESLint
Publisher: Microsoft
ID: dbaeumer.vscode-eslint
```
Command: `code --install-extension dbaeumer.vscode-eslint`

```
Name: Prettier
Publisher: Prettier
ID: esbenp.prettier-vscode
```
Command: `code --install-extension esbenp.prettier-vscode`

#### Environment Variables
```
Name: .env
Publisher: mikestead
ID: mikestead.dotenv
```
Command: `code --install-extension mikestead.dotenv`

---

## 2. Install All Extensions at Once

```bash
code --install-extension golang.Go && \
code --install-extension ms-python.python && \
code --install-extension ms-python.vscode-pylance && \
code --install-extension dsznajder.es7-react-js-snippets && \
code --install-extension Vue.vscode-typescript-vue-plugin && \
code --install-extension ms-azuretools.vscode-docker && \
code --install-extension ckolkman.vscode-postgres && \
code --install-extension eamodio.gitlens && \
code --install-extension humao.rest-client && \
code --install-extension dbaeumer.vscode-eslint && \
code --install-extension esbenp.prettier-vscode && \
code --install-extension mikestead.dotenv
```

---

## 3. VS Code Settings Configuration

### Edit Settings (Ctrl+Shift+P → "Preferences: Open Settings (JSON)")

Update `.vscode/settings.json`:

```json
{
  // General Settings
  "editor.formatOnSave": true,
  "editor.defaultFormatter": "esbenp.prettier-vscode",
  "editor.wordWrap": "on",
  "editor.fontSize": 13,
  "files.trimTrailingWhitespace": true,
  "files.insertFinalNewline": true,

  // Go Settings
  "go.lintOnSave": "package",
  "go.lintTool": "golangci-lint",
  "go.lintArgs": ["--fast"],
  "go.formatTool": "goimports",
  "go.useLanguageServer": true,
  "[go]": {
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
      "source.organizeImports": "explicit"
    }
  },

  // Python Settings
  "python.defaultInterpreterPath": "${workspaceFolder}\\infrastructure\\ai\\.venv\\Scripts\\python.exe",
  "python.languageServer": "Pylance",
  "python.analysis.typeCheckingMode": "basic",
  "python.analysis.autoSearchPaths": true,
  "python.testing.unittestEnabled": true,
  "python.testing.pytestEnabled": true,
  "python.testing.pytestArgs": ["tests"],
  "python.formatting.provider": "black",
  "[python]": {
    "editor.defaultFormatter": "ms-python.python",
    "editor.formatOnSave": true
  },

  // JavaScript/TypeScript Settings
  "javascript.format.enable": true,
  "typescript.tsdk": "node_modules/typescript/lib",
  "[javascript]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode",
    "editor.formatOnSave": true
  },
  "[typescript]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode",
    "editor.formatOnSave": true
  },
  "[json]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode",
    "editor.formatOnSave": true
  },

  // ESLint Settings
  "eslint.enable": true,
  "eslint.alwaysShowStatus": true,
  "eslint.format.enable": true,

  // Docker Settings
  "docker.showExplorer": true,

  // Terminal Settings
  "terminal.integrated.shell.windows": "C:\\Windows\\System32\\cmd.exe",
  "terminal.integrated.defaultProfile.windows": "Command Prompt"
}
```

---

## 4. VS Code Tasks Configuration

Update `.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Go: Build Engine",
      "type": "shell",
      "command": "go",
      "args": ["build", "-o", "../bin/antigravity.exe", "cmd/antigravity/main.go"],
      "options": {
        "cwd": "${workspaceFolder}/engine"
      },
      "problemMatcher": ["$go"],
      "group": {
        "kind": "build",
        "isDefault": true
      }
    },
    {
      "label": "Go: Test All",
      "type": "shell",
      "command": "go",
      "args": ["test", "./..."],
      "options": {
        "cwd": "${workspaceFolder}/engine"
      },
      "problemMatcher": ["$go"]
    },
    {
      "label": "Go: Run Engine (Debug)",
      "type": "shell",
      "command": "go",
      "args": ["run", "cmd/antigravity/main.go"],
      "options": {
        "cwd": "${workspaceFolder}/engine"
      },
      "problemMatcher": ["$go"]
    },
    {
      "label": "Next.js: Build Client",
      "type": "shell",
      "command": "npm",
      "args": ["run", "build"],
      "options": {
        "cwd": "${workspaceFolder}/client"
      }
    },
    {
      "label": "Next.js: Dev Server",
      "type": "shell",
      "command": "npm",
      "args": ["run", "dev"],
      "options": {
        "cwd": "${workspaceFolder}/client"
      },
      "problemMatcher": []
    },
    {
      "label": "npm: Install (client)",
      "type": "shell",
      "command": "npm",
      "args": ["install"],
      "options": {
        "cwd": "${workspaceFolder}/client"
      }
    },
    {
      "label": "npm: Lint (client)",
      "type": "shell",
      "command": "npm",
      "args": ["run", "lint"],
      "options": {
        "cwd": "${workspaceFolder}/client"
      }
    },
    {
      "label": "Docker: Start Infrastructure",
      "type": "shell",
      "command": "docker",
      "args": ["compose", "-f", "docker-compose.prod.yml", "up", "-d"]
    },
    {
      "label": "Docker: Stop Infrastructure",
      "type": "shell",
      "command": "docker",
      "args": ["compose", "-f", "docker-compose.prod.yml", "down"]
    },
    {
      "label": "Docker: View Logs",
      "type": "shell",
      "command": "docker",
      "args": ["compose", "-f", "docker-compose.prod.yml", "logs", "-f"]
    },
    {
      "label": "Python: Create venv",
      "type": "shell",
      "command": "python",
      "args": ["-m", "venv", "infrastructure/ai/.venv"],
      "problemMatcher": []
    },
    {
      "label": "Python: Install Requirements",
      "type": "shell",
      "command": "pip",
      "args": ["install", "-r", "infrastructure/ai/requirements.txt"],
      "problemMatcher": []
    }
  ]
}
```

**How to Run Tasks**: Ctrl+Shift+P → "Tasks: Run Task" → Select the task

---

## 5. Debugging Configuration

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Go: Debug Antigravity",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/engine/cmd/antigravity",
      "env": {
        "DATABASE_URL": "postgres://antigravity:password123@localhost:5432/antigravity",
        "REDIS_URL": "redis://localhost:6379"
      },
      "args": [],
      "showLog": true
    },
    {
      "name": "Go: Debug Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}/engine",
      "args": ["-test.v"]
    },
    {
      "name": "Python: Debug AI Service",
      "type": "python",
      "request": "launch",
      "program": "${workspaceFolder}/infrastructure/ai/strategy_service/api.py",
      "console": "integratedTerminal",
      "justMyCode": true,
      "env": {
        "PYTHONPATH": "${workspaceFolder}/infrastructure/ai"
      }
    },
    {
      "name": "Node.js: Debug Bridge",
      "type": "node",
      "request": "launch",
      "program": "${workspaceFolder}/bridge/bridge.js",
      "console": "integratedTerminal"
    }
  ]
}
```

**How to Debug**: 
- Click on line number to set breakpoint
- Press F5 or go to Run → Start Debugging
- Select configuration from dropdown

---

## 6. Useful Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+Shift+P` | Command Palette |
| `Ctrl+Shift+X` | Extensions |
| `Ctrl+Shift+D` | Debug View |
| `Ctrl+Shift+T` | Test Explorer |
| `Ctrl+`` | Toggle Terminal |
| `F5` | Start Debug |
| `Ctrl+K Ctrl+0` | Fold All |
| `Ctrl+K Ctrl+J` | Unfold All |
| `Ctrl+G` | Go to Line |
| `Ctrl+F` | Find |
| `Ctrl+H` | Find & Replace |
| `Ctrl+Shift+F` | Search in Files |
| `Alt+↑↓` | Move Line Up/Down |
| `Shift+Alt+↑↓` | Duplicate Line |
| `Ctrl+/` | Toggle Comment |
| `Ctrl+Shift+[` | Fold Region |
| `Ctrl+Shift+]` | Unfold Region |

---

## 7. Quick Command Reference

### From VS Code Command Palette (Ctrl+Shift+P):

```
Go: Test Package
Go: Test All Packages
Go: Format Code
Go: Organize Imports
Python: Select Interpreter
Python: Run Python File
Python: Lint Folder
TypeScript: Restart TS Server
ESLint: Fix All Auto-Fixable Problems
Docker: Show Overview
Tasks: Run Task
Tasks: Terminate Task
Debug: Start Debugging
Debug: Start Without Debugging
Source Control: Commit
Source Control: Push
```

---

## 8. Project Structure in VS Code

```
Trading application/
├── .vscode/
│   ├── settings.json          ← Workspace settings
│   ├── tasks.json             ← Build & debug tasks
│   └── launch.json            ← Debugging configs
├── engine/                    ← Go trading engine
│   ├── cmd/
│   │   ├── antigravity/       ← Main engine
│   │   ├── backtest/          ← Backtester
│   │   └── seed_db/           ← Database seeder
│   └── internal/
│       ├── trading/
│       ├── strategy/
│       ├── execution/
│       └── ...
├── client/                    ← Next.js dashboard
│   ├── src/
│   │   └── app/
│   └── package.json
├── bridge/                    ← ChatGPT bridge
│   ├── bridge.js
│   └── package.json
├── infrastructure/            ← Databases & AI
│   ├── ai/                    ← Python AI service
│   │   └── strategy_service/
│   └── docker-compose.yml
├── .env                       ← Environment variables
├── docker-compose.prod.yml
├── UPGRADE_COMMANDS.md        ← Upgrade guide
└── VSCODE_SETUP.md           ← This file
```

---

## 9. Opening Antigravity in VS Code

### Method 1: From Terminal
```bash
cd "C:\Trading apllication"
code .
```

### Method 2: File Menu
- File → Open Folder
- Select `C:\Trading apllication`

### Method 3: Already Open
- If VS Code is already open, File → Open Folder → Select path

---

## 10. Setting Up Go Development

### Install Go Tools
```bash
# In terminal
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-delve/delve/cmd/dlv@latest
```

### Verify Setup
```bash
go version
golangci-lint version
gosec --version
```

---

## 11. Setting Up Python Development

### Create Virtual Environment (if not exists)
```bash
cd infrastructure/ai
python -m venv .venv

# Activate (Windows)
.venv\Scripts\activate

# Activate (Linux/Mac)
source .venv/bin/activate
```

### Install Requirements
```bash
pip install -r requirements.txt
pip install -r requirements-dev.txt
```

### Select Interpreter in VS Code
- Ctrl+Shift+P → "Python: Select Interpreter"
- Choose `.venv` virtual environment

---

## 12. Setting Up Node.js Development

### Install Node.js Extensions
```bash
npm install -g npm-check-updates
npm install -g typescript
npm install -g prettier
npm install -g eslint
```

### Open Integrated Terminal
- Ctrl+` or View → Terminal
- Navigate to project and run:
```bash
cd client
npm install
npm run dev
```

---

## 13. Useful Extensions Commands

### GitLens
- `Ctrl+Shift+P` → "GitLens: Show Repository Graph"
- `Ctrl+Shift+P` → "GitLens: Show File History"

### REST Client
Create `.http` or `.rest` files to test APIs:
```http
### Get Engine Health
GET http://localhost:8080/health

### Get Trade History
GET http://localhost:8080/api/trades?limit=20

### Get Dashboard
GET http://localhost:3000

### Test AI Insights
GET http://localhost:8080/api/ai/insights
```

Then click "Send Request" above each block.

---

## 14. VS Code Extensions to Avoid Conflicts

Remove these if you have them (can conflict):
- Competing formatters (keep only Prettier)
- Multiple Python extensions (keep only Microsoft's)
- Multiple Go extensions (keep only Google's)

Check with:
- Ctrl+Shift+X → Search for potential conflicts
- Look for duplicate functionality

---

## 15. Performance Tips

### For Large Project
```json
{
  "files.watcherExclude": {
    "**/.git/objects/**": true,
    "**/.git/subtree-cache/**": true,
    "**/node_modules/**": true,
    "**/.venv/**": true,
    "**/bin/**": true
  }
}
```

### Disable Expensive Features (if slow)
```json
{
  "python.analysis.typeCheckingMode": "off",
  "go.lintOnSave": false
}
```

---

## 16. Troubleshooting

### Go Extension Not Working
```bash
# In VS Code terminal
go env
go mod download
go mod tidy
```

### Python Intellisense Not Working
```bash
# Check interpreter
python --version

# Reinstall Pylance
code --install-extension ms-python.vscode-pylance --force
```

### ESLint Not Formatting
```bash
cd client
npm install
npm run lint
```

### Debugging Not Working
- Install Go Delve: `go install github.com/go-delve/delve/cmd/dlv@latest`
- Check launch configuration in `.vscode/launch.json`

---

## 17. Quick Start Workflow

1. **Open Project**:
   ```bash
   cd "C:\Trading apllication"
   code .
   ```

2. **Install All Extensions** (Ctrl+Shift+X):
   - Search and install all extensions listed in Section 1

3. **Open Integrated Terminal** (Ctrl+`):
   ```bash
   cd engine
   go mod tidy
   go run cmd/antigravity/main.go
   ```

4. **In New Terminal Tab** (Ctrl+Shift+`):
   ```bash
   cd client
   npm install
   npm run dev
   ```

5. **Start Docker**:
   - Ctrl+Shift+P → "Tasks: Run Task" → "Docker: Start Infrastructure"

6. **Set Breakpoints and Debug**:
   - F5 → Select "Go: Debug Antigravity" or "Python: Debug AI Service"

---

**Last Updated**: April 12, 2026
**Version**: 1.0
