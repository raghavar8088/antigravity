Likely causes

/api/crypto/markets is failing or returning invalid data, so the hook falls back and may still end in Unable to fetch crypto market data.

Browser-side Binance fetch is blocked or failing due to network, CORS, VPN, ad-blocker, firewall, or exchange access restrictions.

Prices are coming through as 0, null, strings, or wrong field names, so the UI hides quotes because it only renders values greater than zero.

First checks

Open DevTools → Network and check /api/crypto/markets every 3 seconds. Confirm it returns HTTP 200 and JSON with real numeric prices.

If that fails, check the Binance request in Network/Console. Look for CORS, failed fetch, blocked request, DNS, or region/network errors.

Inspect the hook state or API response shape. Verify symbols exist and price fields are positive numbers, not missing or zero.

Best fix

Make /api/crypto/markets the primary reliable server-side source, normalize all price fields to numbers, log both primary and fallback errors clearly, and show a visible diagnostics panel with response status and parsed price values.