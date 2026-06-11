const fs = require('fs');

function dayOfYear(date) {
  const start = new Date(date.getFullYear(), 0, 0);
  const diff = date - start;
  return Math.floor(diff / (1000 * 60 * 60 * 24));
}

function pickMessage(config) {
  if (config.messageOverride) {
    return config.messageOverride;
  }

  if (!fs.existsSync(config.messagesFile)) {
    throw new Error(
      `No MESSAGE set and messages file not found: ${config.messagesFile}`,
    );
  }

  const messages = JSON.parse(fs.readFileSync(config.messagesFile, 'utf8'));
  if (!Array.isArray(messages) || messages.length === 0) {
    throw new Error(`messages file must be a non-empty JSON array: ${config.messagesFile}`);
  }

  const index = dayOfYear(new Date()) % messages.length;
  return String(messages[index]).trim();
}

module.exports = { pickMessage };
