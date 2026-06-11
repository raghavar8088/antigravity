const fs = require('fs');
const path = require('path');
const { chromium } = require('playwright');

const ROOT = __dirname;
const AUTH_FILE = path.join(ROOT, 'auth.json');
const CDP_URL = process.env.CDP_URL || 'http://127.0.0.1:9222';

(async () => {
  const browser = await chromium.connectOverCDP(CDP_URL);
  const context = browser.contexts()[0];

  if (!context) {
    throw new Error('No browser context found. Is save-auth-server.js still running?');
  }

  await context.storageState({ path: AUTH_FILE });
  fs.chmodSync(AUTH_FILE, 0o600);

  console.log(`Saved server-side session to ${AUTH_FILE}`);
  console.log('Test with: npm run send');
  await browser.close();
})();
