/**
 * Strategy authority catalog.
 *
 * The catalog has been emptied because strategies have been removed from the
 * application. Public exports remain for migration/status endpoints.
 */

import type { ISPAPCatalogEntry } from "./types";

export const STRATEGY_CATALOG: ISPAPCatalogEntry[] = [];

export const CATALOG_BY_ID = new Map(STRATEGY_CATALOG.map((s) => [s.id, s]));

export const TOTAL_STRATEGIES = STRATEGY_CATALOG.length;
