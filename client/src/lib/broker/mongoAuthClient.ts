/**
 * MongoDB auth user store.
 * Collection: `auth_users` in the same DB as paper trades.
 * Each document: { _id: UUID, email: string, created_at: ISO, last_seen: ISO }
 */

import crypto from "node:crypto";
import { getDb } from "@/lib/broker/mongoTradesClient";

const USERS_COLLECTION = "auth_users";

type AuthUserDoc = {
  _id: string;
  email: string;
  created_at: string;
  last_seen: string;
};

let indexEnsured = false;

async function getUsersCollection() {
  const db = await getDb();
  const col = db.collection<AuthUserDoc>(USERS_COLLECTION);
  if (!indexEnsured) {
    await col.createIndex({ email: 1 }, { unique: true, name: "uniq_email" }).catch(() => {});
    indexEnsured = true;
  }
  return col;
}

/**
 * Find existing user by email or create a new one.
 * Returns a stable UUID used as the account_key for paper trades.
 */
export async function findOrCreateAuthUser(
  rawEmail: string,
): Promise<{ userId: string; email: string }> {
  const col = await getUsersCollection();
  const email = rawEmail.trim().toLowerCase();
  const now = new Date().toISOString();

  const existing = await col.findOne({ email });
  if (existing) {
    await col.updateOne({ _id: existing._id }, { $set: { last_seen: now } }).catch(() => {});
    return { userId: existing._id, email: existing.email };
  }

  const userId = crypto.randomUUID();
  await col.insertOne({ _id: userId, email, created_at: now, last_seen: now });
  return { userId, email };
}
