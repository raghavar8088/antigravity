type FormatUSDOptions = {
  signed?: boolean;
};

const nearZeroEpsilon = 1e-9;

export function formatUSD(value: number, options?: FormatUSDOptions): string {
  if (!Number.isFinite(value)) {
    return "$0.00";
  }

  const normalized = Math.abs(value) < nearZeroEpsilon ? 0 : value;
  const abs = Math.abs(normalized);

  let digits = 2;
  if (abs > 0 && abs < 1) {
    digits = 4;
  }
  if (abs > 0 && abs < 0.01) {
    digits = 6;
  }

  const amount = abs.toLocaleString(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });

  if (normalized < 0) {
    return `-$${amount}`;
  }
  if (options?.signed && normalized > 0) {
    return `+$${amount}`;
  }
  return `$${amount}`;
}
