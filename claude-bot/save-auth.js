const fs = require('fs');
const path = require('path');
const readline = require('readline');
const { launchAuthBrowser } = require('./lib/launchAuthBrowser');

const ROOT = __dirname;
const AUTH_FILE = path.join(ROOT, 'auth.json');

function waitForEnter(prompt) {
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  return new Promise((resolve) => {
    rl.question(prompt, () => {
      rl.close();
      resolve();
    });
  });
}

(async () => {
  console.log('Opening Claude in your real Chrome/Edge browser...');
  console.log('');
  console.log('IMPORTANT — avoid Google login block:');
  console.log('  • Prefer "Continue with email" on Claude (not "Continue with Google")');
  console.log('  • If Google blocks sign-in, close popup and use email login instead');
  console.log('');
  console.log('Steps:');
  console.log('  1) Log in to https://claude.ai');
  console.log('  2) Confirm you can see the chat UI');
  console.log('  3) Return here and press Enter to save auth.json');
  console.log('');

  const context = await launchAuthBrowser(ROOT);
  const page = context.pages()[0] || await context.newPage();

  await page.goto('https://claude.ai', {
    waitUntil: 'domcontentloaded',
    timeout: 90000,
  });

  await waitForEnter('Press Enter after login is complete... ');

  await context.storageState({ path: AUTH_FILE });
  await context.close();

  fs.chmodSync(AUTH_FILE, 0o600);
  console.log(`\nSaved session to ${AUTH_FILE}`);
  console.log('Upload this file to Lightsail:');
  console.log('  scp auth.json ubuntu@YOUR_LIGHTSAIL_IP:/home/ubuntu/claude-bot/auth.json');
})();
