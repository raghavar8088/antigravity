import { request, type FullConfig } from "@playwright/test";
import path from "path";
import {
  E2E_ADMIN_USERNAME,
  E2E_ADMIN_PASSWORD,
} from "../playwright.config";

export const AUTH_STORAGE_STATE = path.join(__dirname, ".auth", "owner.json");

// Logs in once via the API (not the UI) so every authenticated spec starts
// with a ready-made session instead of re-running the login form and its
// 5-attempts/15-min rate limiter (src/app/api/auth/login/route.ts).
export default async function globalSetup(config: FullConfig): Promise<void> {
  const baseURL = config.projects[0]?.use?.baseURL ?? "http://localhost:3000";
  const requestContext = await request.newContext({ baseURL });

  const res = await requestContext.post("/api/auth/login", {
    data: { username: E2E_ADMIN_USERNAME, password: E2E_ADMIN_PASSWORD },
  });
  if (!res.ok()) {
    throw new Error(
      `global-setup: login failed (${res.status()}) — ${await res.text()}`,
    );
  }

  await requestContext.storageState({ path: AUTH_STORAGE_STATE });
  await requestContext.dispose();
}
