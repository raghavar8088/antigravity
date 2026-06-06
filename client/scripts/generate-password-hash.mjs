#!/usr/bin/env node
/**
 * Generate the scrypt hash for ADMIN_PASSWORD_HASH.
 *
 * Usage:
 *   node scripts/generate-password-hash.mjs <password>
 *
 * Example:
 *   node scripts/generate-password-hash.mjs "MySecureP@ssw0rd"
 *
 * Copy the output into your .env.local:
 *   ADMIN_PASSWORD_HASH=scrypt:<salt>:<hash>
 */

import crypto from "node:crypto";
import { promisify } from "node:util";
import readline from "node:readline";

const scrypt = promisify(crypto.scrypt);

async function hashPassword(password) {
  const salt = crypto.randomBytes(16).toString("hex");
  const derived = await scrypt(password, salt, 64);
  return `scrypt:${salt}:${derived.toString("hex")}`;
}

async function main() {
  let password = process.argv[2];

  if (!password) {
    // Interactive prompt so password doesn't appear in shell history
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    password = await new Promise((resolve) => {
      process.stdout.write("Enter password: ");
      // Hide input
      if (process.stdin.isTTY) process.stdin.setRawMode(true);
      let input = "";
      process.stdin.on("data", (char) => {
        const c = char.toString();
        if (c === "\r" || c === "\n") {
          if (process.stdin.isTTY) process.stdin.setRawMode(false);
          process.stdout.write("\n");
          rl.close();
          resolve(input);
        } else if (c === "") {
          process.exit(1); // Ctrl+C
        } else {
          input += c;
        }
      });
    });
  }

  if (!password || password.length < 8) {
    console.error("Error: password must be at least 8 characters");
    process.exit(1);
  }

  console.log("\nHashing…");
  const hash = await hashPassword(password);

  console.log("\n✓ Add this to your .env.local:\n");
  console.log(`ADMIN_PASSWORD_HASH=${hash}`);
  console.log(`\n✓ Also set:\n`);
  console.log(`ADMIN_USERNAME=<your_chosen_username>`);
  console.log(`AUTH_JWT_SECRET=<64-char hex secret>\n`);
  console.log("To generate AUTH_JWT_SECRET:");
  console.log('  node -e "console.log(require(\'crypto\').randomBytes(32).toString(\'hex\'))"');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
