/**
 * Consolidated into @/components/ui/StatusChip's Badge export, which now
 * accepts this file's old variant/size/dot/children API as an alias. Kept
 * as a re-export shim so existing call sites (OrderBlotter, PositionManager,
 * ReconciliationStatus, LivePnlFeed, GrafanaEmbed) don't need edits.
 */
export { Badge, type BadgeVariant } from "./StatusChip";

export type BadgeSize = "sm" | "md";
