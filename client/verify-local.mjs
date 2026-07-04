import { chromium } from 'playwright';

(async () => {
  const browser = await chromium.launch({ headless: true });

  // ── Desktop 1400×900 ─────────────────────────────────────────
  const ctx = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  const page = await ctx.newPage();

  // Login via API directly (bypass UI)
  const loginRes = await page.request.post('http://localhost:3000/api/auth/login', {
    data: { username: 'raghava', password: 'raghava' },
  });
  console.log('Login status:', loginRes.status());
  const loginBody = await loginRes.text();
  console.log('Login body:', loginBody.slice(0, 200));

  await page.goto('http://localhost:3000/terminal/pre-live-engine', { waitUntil: 'networkidle', timeout: 20000 }).catch(e => console.log('nav err:', e.message));
  await page.waitForTimeout(4000);

  const url = page.url();
  console.log('URL after nav:', url);
  await page.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/local_desktop.png', fullPage: false });
  await page.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/local_full.png', fullPage: true });

  const info = await page.evaluate(() => {
    const gp = document.querySelector('.google-page');
    const scrollEls = [...document.querySelectorAll('*')].filter(el => {
      const ox = window.getComputedStyle(el).overflowX;
      return (ox === 'scroll' || ox === 'auto') && el.scrollWidth > 0;
    }).slice(0, 12).map(el => ({
      tag: el.tagName,
      ox: window.getComputedStyle(el).overflowX,
      sw: el.scrollWidth,
      cw: el.clientWidth,
      cls: el.className?.toString().slice(0, 50),
      inlineStyle: el.getAttribute('style')?.slice(0, 80) || '',
    }));

    const emptyEls = [...document.querySelectorAll('[class*="empty"]')].map(el => ({
      cls: el.className?.toString().slice(0, 60),
      h: Math.round(el.getBoundingClientRect().height),
      minH: window.getComputedStyle(el).minHeight,
    }));

    const children = gp ? [...gp.children].map(c => ({
      tag: c.tagName,
      cls: c.className?.toString().slice(0, 40),
      h: Math.round(c.getBoundingClientRect().height),
      top: Math.round(c.getBoundingClientRect().top),
    })) : [];

    return {
      gpGap: gp ? window.getComputedStyle(gp).gap : 'NO google-page',
      gpInlineGap: gp ? (gp.getAttribute('style') || 'no inline style') : 'missing',
      children,
      scrollEls,
      emptyEls,
    };
  });
  console.log('PAGE INFO:', JSON.stringify(info, null, 2));

  // Mobile 390×844
  const mCtx = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true });
  const cookies = await ctx.cookies();
  await mCtx.addCookies(cookies);
  const mp = await mCtx.newPage();
  await mp.goto('http://localhost:3000/terminal/pre-live-engine', { waitUntil: 'networkidle', timeout: 20000 }).catch(e => console.log('mob err:', e.message));
  await mp.waitForTimeout(3000);
  await mp.screenshot({ path: 'C:/Users/ragha/AppData/Local/Temp/claude/local_mobile.png', fullPage: true });
  console.log('Mobile URL:', mp.url());
  const mob = await mp.evaluate(() => ({
    bodyH: document.body.scrollHeight,
    vw: window.innerWidth,
    sidebar: (() => { const s = document.querySelector('.terminal-sidebar'); return s ? { w: Math.round(s.getBoundingClientRect().width), display: window.getComputedStyle(s).display } : 'not found'; })(),
    main: (() => { const m = document.querySelector('.terminal-main'); return m ? { ml: window.getComputedStyle(m).marginLeft, w: Math.round(m.getBoundingClientRect().width) } : 'not found'; })(),
  }));
  console.log('MOBILE:', JSON.stringify(mob, null, 2));

  await browser.close();
  console.log('Done.');
})();
