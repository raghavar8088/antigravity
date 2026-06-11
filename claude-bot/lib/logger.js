const fs = require('fs');

function createLogger(logFile) {
  return function log(level, message) {
    const line = `[${new Date().toISOString()}] [${level}] ${message}`;
    console.log(line);
    fs.appendFileSync(logFile, `${line}\n`);
  };
}

module.exports = { createLogger };
