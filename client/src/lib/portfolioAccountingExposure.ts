import type { PaperPositionDoc } from "@/lib/paperDeskClient";
import { normalizePaperPositionSide } from "@/lib/paperDeskPositionMath";
import type { ExposureMetrics } from "@/lib/portfolioAccountingTypes";

export function computeExposureFromPositions(
  positions: readonly PaperPositionDoc[],
  markPrice?: number,
): ExposureMetrics {
  let longBtc = 0;
  let shortBtc = 0;
  let exposureUsd = 0;

  for (const pos of positions) {
    const size = Number(pos.size) || 0;
    if (size <= 0) continue;
    const side = normalizePaperPositionSide(pos.side);
    if (side === "LONG") longBtc += size;
    else shortBtc += size;

    const px = Number.isFinite(markPrice) && (markPrice ?? 0) > 0
      ? (markPrice as number)
      : Number(pos.entry_price) || 0;
    if (px > 0) exposureUsd += size * px;
  }

  const net = longBtc - shortBtc;
  return {
    long_exposure_btc: longBtc,
    short_exposure_btc: shortBtc,
    net_exposure_btc: net,
    gross_exposure_btc: longBtc + shortBtc,
    exposure_usd: exposureUsd,
  };
}
