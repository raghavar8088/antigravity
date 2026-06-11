const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');
const { getConfig } = require('./lib/env');
const { pickMessage } = require('./lib/message');
const { createLogger } = require('./lib/logger');
const { openChat, sendMessage, captureReply, isLoginPage } = require('./lib/claudePage');

const ROOT = __dirname;

function acquireLock(lockFile) {
  if (fs.existsSync(lockFile)) {
    const existing = fs.readFileSync(lockFile, 'utf8').trim();
    throw new Error(`Another run appears active (lock: ${existing}). Remove ${lockFile} if stale.`);
  }
  fs.writeFileSync(lockFile, `${process.pid} ${new Date().toISOString()}`);
}

function releaseLock(lockFile) {
  try {
    if (fs.existsSync(lockFile)) fs.unlinkSync(lockFile);
  } catch {
    // ignore
  }
}

async function saveFailureScreenshot(page, screenshotDir, log) {
  try {
    fs.mkdirSync(screenshotDir, { recursive: true });
    const file = path.join(
      screenshotDir,
      `failure-${new Date().toISOString().replace(/[:.]/g, '-')}.png`,
    );
    await page.screenshot({ path: file, fullPage: true });
    log('ERROR', `Saved failure screenshot: ${file}`);
  } catch (err) {
    log('ERROR', `Could not save screenshot: ${err.message}`);
  }
}

(async () => {
  const config = getConfig(ROOT);
  const log = createLogger(config.logFile);

  if (!fs.existsSync(config.authFile)) {
    throw new Error(
      `Missing ${config.authFile}. Run "npm run save-auth" on your laptop, then upload auth.json to Lightsail.`,
    );
  }

  acquireLock(config.lockFile);
  log('INFO', `Starting claude-bot (mode=${config.mode})`);

  const message = pickMessage(config);
  log('INFO', `Message preview: ${message.slice(0, 120)}${message.length > 120 ? '...' : ''}`);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ storageState: config.authFile });
  const page = await context.newPage();

  try {
    await openChat(page, config, log);

    if (await isLoginPage(page)) {
      throw new Error(
        'Session expired or not logged in. Re-run save-auth on your laptop and upload a fresh auth.json.',
      );
    }

    await sendMessage(page, message, log);

    if (config.mode === 'new') {
      const reply = await captureReply(page, config.replyTimeoutMs, log);
      fs.writeFileSync(config.replyFile, `${reply}\n`);
      log('INFO', `Saved reply to ${config.replyFile}`);
    }

    log('INFO', 'Run completed successfully');
  } catch (err) {
    log('ERROR', err.message);
    await saveFailureScreenshot(page, config.screenshotDir, log);
    process.exitCode = 1;
  } finally {
    await browser.close();
    releaseLock(config.lockFile);
  }
})();
