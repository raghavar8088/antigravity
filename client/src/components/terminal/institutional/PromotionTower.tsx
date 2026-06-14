"use client";

/** @deprecated Promotion pipeline removed. */

import type { StrategyStatus } from "@/lib/strategyAuthority/types";

export interface TowerLayerData {
  status: StrategyStatus;
  count: number;
}

export function PromotionTower({
  layers,
  retiredCount = 0,
  onSelectStatus,
  selectedStatus,
  totalStrategies = 305,
}: {
  layers: TowerLayerData[];
  retiredCount?: number;
  onSelectStatus?: (status: StrategyStatus | null) => void;
  selectedStatus?: StrategyStatus | null;
  totalStrategies?: number;
}) {
  void layers;
  void retiredCount;
  void onSelectStatus;
  void selectedStatus;
  void totalStrategies;
  return null;
}
