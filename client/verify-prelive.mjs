import { chromium } from 'playwright';

const BASE = 'https://antigravity-five-pink.vercel.app';
const TARGET = `${BASE}/terminal/pre-live-engine`;
const USERNAME = 'raghava';

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();

  await page.goto(`${BASE}/sign-in`, { waitUntil: 'networkidle', timeout: 20000 }).catch(() => {});
  const hasUser = await page.locator('input[type="text"]').count();
  console.log('Login inputs found:', hasUser);
  if (hasUser > 0) {
    await page.fill('input[type="text"]', USERNAME);
    await page.fill('input[type="password"]', USERNAME);
    await page.click('button[type="submit"]').catch(() => {});
    await page.waitForTimeout(3000);
  }
  console.log('After login URL:', page.url());
  await page.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/after_login.png' });

  await page.goto(TARGET, { waitUntil: 'networkidle', timeout: 25000 }).catch(() => {});
  await page.waitForTimeout(5000);
  await page.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/desktop_viewport.png', fullPage: false });
  await page.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/desktop_full.png', fullPage: true });
  console.log('Pre-live URL:', page.url());

  const info = await page.evaluate(() => {
    const gp = document.querySelector('.google-page');
    return {
      googlePageGap: gp ? window.getComputedStyle(gp).gap : 'missing',
      children: gp ? [...gp.children].map(c => ({ tag: c.tagName, cls: c.className?.toString().slice(0,40), h: Math.round(c.getBoundingClientRect().height), top: Math.round(c.getBoundingClientRect().top) })) : [],
      scrollWrappers: [...document.querySelectorAll('*')].filter(el => {
        const ox = window.getComputedStyle(el).overflowX;
        return ox === 'scroll' || ox === 'auto';
      }).slice(0,15).map(el => ({ ox: window.getComputedStyle(el).overflowX, sw: el.scrollWidth, cw: el.clientWidth, cls: el.className?.toString().slice(0,40) })),
      emptyStates: [...document.querySelectorAll('[class*="empty"]')].map(el => ({ cls: el.className?.toString().slice(0,50), h: Math.round(el.getBoundingClientRect().height), minH: window.getComputedStyle(el).minHeight })),
    };
  });
  console.log('PAGE INFO:', JSON.stringify(info, null, 2));

  // Mobile
  const cookies = await ctx.cookies();
  const mCtx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true });
  await mCtx.addCookies(cookies);
  const mp = await mCtx.newPage();
  await mp.goto(TARGET, { waitUntil: 'networkidle', timeout: 20000 }).catch(() => {});
  await mp.waitForTimeout(3000);
  await mp.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/mobile_full.png', fullPage: true });
  const mob = await mp.evaluate(() => ({
    url: location.href,
    sidebar: (() => { const s = document.querySelector('.terminal-sidebar'); return s ? { w: Math.round(s.getBoundingClientRect().width), display: window.getComputedStyle(s).display } : null; })(),
    mainMargin: (() => { const m = document.querySelector('.terminal-main'); return m ? window.getComputedStyle(m).marginLeft : null; })(),
    bodyH: document.body.scrollHeight,
    vw: window.innerWidth,
  }));
  console.log('MOBILE:', JSON.stringify(mob, null, 2));

  await browser.close();
  console.log('Done.');
})();
