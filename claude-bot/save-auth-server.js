const { chromium } = require('playwright');

const CDP_PORT = Number(process.env.CDP_PORT || 9222);

(async () => {
  console.log('=== STEP 1: SERVER (this Lightsail terminal) ===');
  console.log('Browser starting in headless mode with remote debugging...');
  console.log('');
  console.log('=== STEP 2: YOUR LAPTOP (Windows PowerShell — NOT this terminal) ===');
  console.log('Open PowerShell on your PC and run:');
  console.log('');
  console.log('  ssh -i "D:\\Trading apllication\\LightsailDefaultKey-ap-south-1.pem" -L 9222:localhost:9222 ubuntu@13.233.8.80');
  console.log('');
  console.log('=== STEP 3: YOUR LAPTOP (Chrome browser) ===');
  console.log('  1) Open chrome://inspect');
  console.log('  2) Configure → add localhost:9222');
  console.log('  3) Click "inspect" under Remote Target');
  console.log('  4) Log in to Claude + pass Cloudflare');
  console.log('');
  console.log('=== STEP 4: SERVER (open 2nd Lightsail SSH tab) ===');
  console.log('  cd ~/claude-bot && npm run capture-auth && npm run send');
  console.log('');
  console.log(`CDP listening on port ${CDP_PORT}. Keep this terminal open. (Ctrl+C to stop)`);

  const browser = await chromium.launch({
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      `--remote-debugging-port=${CDP_PORT}`,
    ],
  });

  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto('https://claude.ai/new', {
    waitUntil: 'domcontentloaded',
    timeout: 120000,
  });

  await new Promise(() => {});
})();
