import { test as base, expect } from "./marketData";
import { type Page } from "@playwright/test";
import { AUTH_STORAGE_STATE } from "../global-setup";

// Reuses the session captured once in global-setup.ts instead of driving the
// /login form per spec — keeps protected-page specs fast and avoids the
// login rate limiter (5 attempts / 15 min).
export const test = base.extend({
  storageState: AUTH_STORAGE_STATE,
});

export { expect, type Page };
