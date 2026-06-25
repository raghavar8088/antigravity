# AWS Lightsail — Go Engine Redeploy Reference

**Server:** `ubuntu@13.233.8.80` (ap-south-1a / Mumbai)  
**SSH Key:** `.application-credentials/LightsailDefaultKey-ap-south-1.pem`  
**Remote repo:** `/home/ubuntu/antigravity`

---

## One-Command Redeploy (from local machine)

**Windows (PowerShell):**
```powershell
powershell -ExecutionPolicy Bypass -File scripts\deploy-aws.ps1
```

**Linux / Git Bash:**
```bash
bash scripts/deploy-aws.sh
```

Both scripts read `scripts/aws-deploy.env` and SSH into the server to run `git pull + docker-compose rebuild`.

---

## From Inside the Lightsail Browser Terminal

Already logged in at `ubuntu@ip-172-26-5-93:~/antigravity$`? Run this:

```bash
bash scripts/update-aws-engine.sh
```

---

## Manual SSH Commands

### 1. Open SSH session
```bash
ssh -i ".application-credentials/LightsailDefaultKey-ap-south-1.pem" -o StrictHostKeyChecking=accept-new ubuntu@13.233.8.80
```

### 2. Full redeploy (git pull + rebuild + restart)
```bash
cd /home/ubuntu/antigravity && bash scripts/update-aws-engine.sh
```

### 3. Individual steps (if you need finer control)

```bash
# Pull latest code
cd /home/ubuntu/antigravity
git pull origin main

# Stop old container
docker rm -f antigravity_engine 2>/dev/null || true
docker-compose -f docker-compose.prod.yml down --remove-orphans

# Rebuild and start
docker-compose -f docker-compose.prod.yml build --pull engine
docker-compose -f docker-compose.prod.yml up -d --force-recreate --remove-orphans

# Check health
curl -sS http://127.0.0.1/health

# Check container status
docker-compose -f docker-compose.prod.yml ps

# Check which commit is running
git log -1 --oneline
```

---

## Useful Remote Commands

```bash
# Live engine logs (follow)
docker-compose -f docker-compose.prod.yml logs -f engine

# Last 100 log lines
docker-compose -f docker-compose.prod.yml logs --tail=100 engine

# Restart engine without rebuilding
docker-compose -f docker-compose.prod.yml restart engine

# Check running containers
docker ps

# Disk usage
df -h

# Engine process memory
docker stats --no-stream antigravity_engine
```

---

## Edit secrets on server

```bash
ssh -i ".application-credentials/LightsailDefaultKey-ap-south-1.pem" ubuntu@13.233.8.80
nano /home/ubuntu/antigravity/.env
# then restart:
cd /home/ubuntu/antigravity && docker-compose -f docker-compose.prod.yml restart engine
```

---

## SSH key permissions fix (Windows — run once if SSH refuses the key)

```powershell
$key = ".application-credentials\LightsailDefaultKey-ap-south-1.pem"
icacls $key /inheritance:r
icacls $key /remove "NT AUTHORITY\Authenticated Users" "BUILTIN\Users" "Everyone"
icacls $key /grant:r "${env:USERDOMAIN}\${env:USERNAME}:R"
```
