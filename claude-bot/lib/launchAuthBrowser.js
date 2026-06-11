const path = require('path');
const { chromium } = require('playwright');

const STEALTH_ARGS = [
  '--disable-blink-features=AutomationControlled',
];

async function launchAuthBrowser(rootDir) {
  const profileDir = path.join(rootDir, '.chrome-profile');
  const baseOptions = {
    headless: false,
    args: STEALTH_ARGS,
    ignoreDefaultArgs: ['--enable-automation'],
    viewport: null,
  };

  for (const channel of ['chrome', 'msedge']) {
    try {
      const context = await chromium.launchPersistentContext(profileDir, {
        ...baseOptions,
        channel,
      });
      console.log(`Using installed ${channel === 'chrome' ? 'Google Chrome' : 'Microsoft Edge'}.`);
      return context;
    } catch (err) {
      console.warn(`Could not launch ${channel}: ${err.message}`);
    }
  }

  console.warn(
    'Installed Chrome/Edge not found. Falling back to Playwright Chromium — Google login may be blocked.',
  );
  return chromium.launchPersistentContext(profileDir, baseOptions);
}

module.exports = { launchAuthBrowser };
