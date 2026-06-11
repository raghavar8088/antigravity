const fs = require('fs');
const path = require('path');
const { getConfig } = require('./lib/env');
const { pickMessage } = require('./lib/message');
const { createLogger } = require('./lib/logger');

const ROOT = __dirname;

(async () => {
  const config = getConfig(ROOT);
  const log = createLogger(config.logFile);
  const apiKey = process.env.ANTHROPIC_API_KEY;

  if (!apiKey) {
    throw new Error('Set ANTHROPIC_API_KEY in .env (get one at console.anthropic.com)');
  }

  const message = pickMessage(config);
  const model = process.env.CLAUDE_MODEL || 'claude-sonnet-4-20250514';

  log('INFO', `Sending via Claude API (model=${model})`);
  log('INFO', `Prompt preview: ${message.slice(0, 120)}${message.length > 120 ? '...' : ''}`);

  const response = await fetch('https://api.anthropic.com/v1/messages', {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      'x-api-key': apiKey,
      'anthropic-version': '2023-06-01',
    },
    body: JSON.stringify({
      model,
      max_tokens: Number(process.env.MAX_TOKENS || 4096),
      messages: [{ role: 'user', content: message }],
    }),
  });

  const data = await response.json();

  if (!response.ok) {
    throw new Error(data.error?.message || `API error ${response.status}`);
  }

  const reply = data.content
    .filter((block) => block.type === 'text')
    .map((block) => block.text)
    .join('\n');

  const output = [
    `--- ${new Date().toISOString()} ---`,
    `PROMPT: ${message}`,
    '',
    `REPLY:`,
    reply,
    '',
  ].join('\n');

  fs.appendFileSync(config.replyFile, output);
  log('INFO', `Reply saved to ${config.replyFile}`);
  log('INFO', 'API send completed successfully');
})();
