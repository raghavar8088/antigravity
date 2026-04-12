# Crypto Equity Diagnosis

You are diagnosing why a crypto equity dashboard may appear "not working".

Known code facts:
- The UI uses `useCryptoEquityEngine`.
- It polls `/api/crypto/markets` every 3 seconds.
- If that fails, it tries direct Binance fetch from the browser.
- If both fail, it sets diagnostics to `Unable to fetch crypto market data.`
- The UI shows quotes only when prices are greater than zero.

Question:
What are the top 3 most likely reasons this module is not working for a user, and what exact checks should they perform first?

Reply with exactly these sections:
1. Likely causes
2. First checks
3. Best fix

Keep it under 180 words.
