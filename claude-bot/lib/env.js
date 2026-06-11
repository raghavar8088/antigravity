const fs = require('fs');
const path = require('path');

function loadDotEnv(filePath) {
  const resolved = path.resolve(filePath);
  if (!fs.existsSync(resolved)) return;

  const lines = fs.readFileSync(resolved, 'utf8').split('\n');
  for (const raw of lines) {
    const line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    const eq = line.indexOf('=');
    if (eq === -1) continue;
    const key = line.slice(0, eq).trim();
    let value = line.slice(eq + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (process.env[key] === undefined) {
      process.env[key] = value;
    }
  }
}

function getConfig(rootDir) {
  loadDotEnv(path.join(rootDir, '.env'));

  const mode = (process.env.MODE || 'fixed').toLowerCase();
  if (mode !== 'fixed' && mode !== 'new') {
    throw new Error('MODE must be "fixed" or "new"');
  }

  return {
    mode,
    chatUrl: process.env.CLAUDE_CHAT_URL || '',
    messageOverride: (process.env.MESSAGE || '').trim(),
    messagesFile: process.env.MESSAGES_FILE || path.join(rootDir, 'messages.json'),
    replyTimeoutMs: Number(process.env.REPLY_TIMEOUT_MS || 90000),
    authFile: path.join(rootDir, 'auth.json'),
    logFile: path.join(rootDir, 'log.txt'),
    replyFile: path.join(rootDir, 'last-reply.txt'),
    lockFile: path.join(rootDir, '.send.lock'),
    screenshotDir: path.join(rootDir, 'screenshots'),
  };
}

module.exports = { loadDotEnv, getConfig };
