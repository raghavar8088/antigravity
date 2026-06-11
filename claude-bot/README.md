# Claude Bot — Scheduled Web Chat on AWS Lightsail

Automates sending a daily message to Claude web chat at a fixed time (default **4:00 PM IST**). Runs on your Lightsail instance — your laptop can stay off.

## Architecture

```
AWS Lightsail (Ubuntu)
  → cron (daily 4:00 PM)
  → send-message.js (Playwright)
  → claude.ai (fixed chat or new chat)
  → log.txt (+ last-reply.txt in new-chat mode)
```

## Quick start (30 minutes)

### Part A — On your Windows laptop (one-time login)

```powershell
cd "d:\Trading apllication\claude-bot"
npm install
npx playwright install chromium
npm run save-auth
```

1. Your **real Chrome or Edge** opens (not "Chrome for Testing").
2. Log in to Claude manually.
3. **Use email login** — avoid "Continue with Google" (Google blocks automated browsers).
4. Press Enter in the terminal when done.
5. `auth.json` is created locally (do not commit it).

### Cloudflare on Lightsail (important)

`auth.json` saved on your **laptop** often fails on **AWS Lightsail** because Cloudflare ties sessions to the server IP.

If the bot shows *"Performing security verification"* or *"Verify you are human"*, create auth **on the server**:

**Where to run each command:**

| Step | Where | Command |
|------|-------|---------|
| 1 | **Lightsail browser SSH** | `cd ~/claude-bot && npm run save-auth-server` |
| 2 | **Your laptop PowerShell** | `ssh -i "D:\Trading apllication\LightsailDefaultKey-ap-south-1.pem" -L 9222:localhost:9222 ubuntu@13.233.8.80` |
| 3 | **Your laptop Chrome** | `chrome://inspect` → Configure `localhost:9222` → Inspect → login |
| 4 | **2nd Lightsail SSH tab** | `cd ~/claude-bot && npm run capture-auth && npm run send` |

Do **not** run the `ssh -L` command inside the Lightsail browser terminal — that only works from your Windows PC.

### Google login blocked?

If you see *"This browser or app may not be secure"*:

1. Close the Google popup.
2. On Claude, choose **Continue with email** instead of Google.
3. Re-run `npm run save-auth` if needed — the script now uses your installed Chrome/Edge.

Copy `.env.example` to `.env` and set your chat URL:

```powershell
copy .env.example .env
notepad .env
```

Set at minimum:

```env
MODE=fixed
CLAUDE_CHAT_URL=https://claude.ai/chat/YOUR_CHAT_ID
```

Get `YOUR_CHAT_ID` from the browser address bar when you open the conversation you want to post into daily.

### Part B — Deploy to Lightsail

Replace `YOUR_LIGHTSAIL_IP` with your instance IP.

```powershell
cd "d:\Trading apllication\claude-bot"
.\deploy-to-lightsail.ps1 -LightsailIp YOUR_LIGHTSAIL_IP
```

### Part C — On Lightsail (SSH)

```bash
cd ~/claude-bot
chmod +x setup-lightsail.sh install-cron.sh
./setup-lightsail.sh
npm run send
```

Check Claude — your test message should appear. Then schedule daily 4 PM:

```bash
./install-cron.sh
```

Default cron: **16:00 Asia/Kolkata** (4:00 PM IST).

To use a different time:

```bash
CRON_HOUR=9 CRON_MINUTE=30 TIMEZONE=Asia/Kolkata ./install-cron.sh
```

## Modes

| MODE | Behavior |
|------|----------|
| `fixed` | Opens one existing chat URL and posts the message (recommended) |
| `new` | Opens a fresh chat, posts the message, waits for reply, saves to `last-reply.txt` |

## Messages

- Set a fixed message in `.env`: `MESSAGE=Your daily prompt`
- Or edit `messages.json` — the bot rotates through messages by day-of-year

## Files

| File | Purpose |
|------|---------|
| `save-auth.js` | Run on laptop to capture Claude login session |
| `send-message.js` | Main bot — run manually or via cron |
| `auth.json` | Session cookies (secret — never commit) |
| `log.txt` | Run history on Lightsail |
| `install-cron.sh` | Installs daily 4 PM cron job |
| `setup-lightsail.sh` | Installs Node, Playwright, dependencies |

## Maintenance

Sessions expire. When messages stop appearing:

1. On laptop: `npm run save-auth`
2. Redeploy: `.\deploy-to-lightsail.ps1 -LightsailIp YOUR_IP`
3. On Lightsail: `npm run send` to verify

Check logs:

```bash
tail -f ~/claude-bot/log.txt
```

Failure screenshots are saved under `~/claude-bot/screenshots/`.

## Risks

- Claude may log you out, show CAPTCHA, or change UI selectors.
- Browser automation can break after Claude UI updates.
- For maximum reliability (no web UI), use the **Claude API** instead of Playwright.

## Manual cron entry

If you prefer editing crontab yourself:

```bash
crontab -e
```

Add:

```cron
0 16 * * * cd /home/ubuntu/claude-bot && /usr/bin/node /home/ubuntu/claude-bot/send-message.js >> /home/ubuntu/claude-bot/log.txt 2>&1
```

Ensure server timezone matches your intended local time:

```bash
timedatectl
sudo timedatectl set-timezone Asia/Kolkata
```
