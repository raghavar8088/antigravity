async function findChatInput(page) {
  const candidates = [
    page.locator('div[contenteditable="true"][data-testid="chat-input"]').last(),
    page.locator('div.ProseMirror[contenteditable="true"]').last(),
    page.locator('div[contenteditable="true"]').last(),
    page.locator('textarea').last(),
  ];

  for (const locator of candidates) {
    try {
      await locator.waitFor({ state: 'visible', timeout: 5000 });
      return locator;
    } catch {
      // try next selector
    }
  }

  throw new Error('Could not find Claude chat input. UI may have changed or session expired.');
}

async function isLoginPage(page) {
  const url = page.url();
  if (url.includes('/login') || url.includes('/auth')) return true;

  const loginHints = [
    page.getByRole('button', { name: /log in|sign in/i }),
    page.getByText(/continue with google|sign in to claude/i),
  ];

  for (const hint of loginHints) {
    if (await hint.count()) {
      try {
        if (await hint.first().isVisible({ timeout: 1000 })) return true;
      } catch {
        // ignore
      }
    }
  }

  return false;
}

async function isCloudflareChallenge(page) {
  const hints = [
    page.getByText(/verify you are human/i),
    page.getByText(/performing security verification/i),
    page.getByText(/security service to protect against malicious bots/i),
  ];

  for (const hint of hints) {
    try {
      if (await hint.first().isVisible({ timeout: 2000 })) return true;
    } catch {
      // ignore
    }
  }

  return false;
}

async function openChat(page, config, log) {
  if (config.mode === 'new') {
    log('INFO', 'Opening new Claude chat');
    await page.goto('https://claude.ai/new', {
      waitUntil: 'domcontentloaded',
      timeout: 90000,
    });
    await page.waitForTimeout(3000);

    if (await isCloudflareChallenge(page)) {
      throw new Error(
        'Cloudflare blocked this server IP. Run save-auth-server.js on Lightsail and capture auth with capture-auth-cdp.js (see README).',
      );
    }
    return;
  }

  if (!config.chatUrl) {
    throw new Error('CLAUDE_CHAT_URL is required when MODE=fixed');
  }

  log('INFO', `Opening fixed chat: ${config.chatUrl}`);
  await page.goto(config.chatUrl, {
    waitUntil: 'domcontentloaded',
    timeout: 90000,
  });
}

async function sendMessage(page, message, log) {
  const input = await findChatInput(page);
  await input.click();
  await input.fill(message);
  await page.keyboard.press('Enter');
  log('INFO', 'Message submitted');
}

async function captureReply(page, timeoutMs, log) {
  log('INFO', `Waiting up to ${timeoutMs}ms for assistant reply`);

  const replyLocator = page.locator(
    '[data-testid="assistant-message"], [data-is-streaming="false"] .font-claude-message, .font-claude-message',
  );

  const deadline = Date.now() + timeoutMs;
  let lastText = '';

  while (Date.now() < deadline) {
    const count = await replyLocator.count();
    if (count > 0) {
      const texts = await replyLocator.allTextContents();
      const combined = texts.map((t) => t.trim()).filter(Boolean).join('\n\n');
      if (combined && combined === lastText) {
        return combined;
      }
      lastText = combined;
    }
    await page.waitForTimeout(2000);
  }

  if (lastText) {
    log('WARN', 'Reply still streaming or incomplete; saving latest captured text');
    return lastText;
  }

  throw new Error('Timed out waiting for assistant reply');
}

module.exports = {
  openChat,
  sendMessage,
  captureReply,
  isLoginPage,
};
