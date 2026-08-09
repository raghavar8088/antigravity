# Live Demo Engine — deploy

A second scalp_prelive container against Delta's demo venue, port 8095.
Separate process and separate credentials from the live engine, so the two
wallets cannot be confused. The live container on 8094 is untouched.

## Required env (`/home/ubuntu/.scalp_demo_env`)

```
DELTA_TESTNET=true
DELTA_API_KEY=<demo key with TRADING permission>
DELTA_API_SECRET=<demo secret>
SCALP_DEMO_API_TOKEN=<any shared token, must match Vercel>
SCALP_LIVE_STREAMS=ANTI_D20_MACD_Cross:BTCUSD,ANTI_D20_MACD_Cross:BTCUSD,ANTI_D20_HeikinAshi_Flip:XRPUSD,ANTI_D20_VWAP_Reversion:BTCUSD,ANTI_M1_RSI2_5_95_T50_Long:XRPUSD,ANTI_M1_RSI2_10_90_T20_Short:XRPUSD,ANTI_M1_RSI2_10_90_T20_Short:BTCUSD,ANTI_M1_RSI_Div_Long:BTCUSD,ANTI_Recurrence_Quantification_Signal:DOGEUSD,ANTI_Recurrence_Quantification_Signal:BTCUSD,ANTI_Recurrence_Quantification_Signal:ETHUSD,ANTI_Ornstein_Uhlenbeck_Reversion:DOGEUSD,ANTI_Ornstein_Uhlenbeck_Reversion:ONDOUSD,ANTI_M1_InsideBar_V20_Long:ETHUSD,ANTI_M1_InsideBar_V12_Long:ETHUSD,ANTI_M1_InsideBar_V20_Short:SOLUSD,ANTI_M1_InsideBar_V12_Short:SOLUSD,ANTI_M1_NR7_Expand_T20_Long:ETHUSD,ANTI_M1_VWAP_Rev_40bp_Short:BTCUSD,ANTI_M1_VWAP_Rev_70bp_Short:BTCUSD,ANTI_M1_VWAP_Rev_40bp_Short:ADAUSD,ANTI_M1_VWAP_Rev_70bp_Short:ONDOUSD,ANTI_M1_VWAP_Rev_40bp_Long:1000SHIBUSD,ANTI_M1_VWAP_Rev_70bp_Long:1000SHIBUSD,ANTI_M1_VWAP_Rev_40bp_Long:BTCUSD,ANTI_M1_VWAP_Rev_70bp_Long:BTCUSD,ANTI_M1X_VWAP_TrendPull_Long:BTCUSD,ANTI_M1_HMA34_Flip_Long:SOLUSD,ANTI_M1_HMA21_Flip_Short:ADAUSD,ANTI_M1_Break_D60_T50_Long:SOLUSD,ANTI_M1_Break_D30_T20_Long:SOLUSD
```

## Why the streams differ from live

The demo venue lists 16 perpetuals and none of the ten the live engine
trades, so an exact roster would place zero orders forever. The 31 streams
keep their strategy logic and pairing structure, re-pointed by liquidity
rank onto demo majors:

```
COOKIEUSD -> BTCUSD        MUBARAKUSD -> DOGEUSD
LABUSD    -> ETHUSD        SKYAIUSD   -> ADAUSD
AVAAIUSD  -> SOLUSD        SOLVUSD    -> ONDOUSD
TSTUSD    -> XRPUSD        SAGAUSD    -> 1000SHIBUSD
BANKUSD   -> BTCUSD        BLESSUSD   -> ETHUSD
```

Tokenised equities (NVDAX/QQQX/SPYX/SLVON/SNDKB/USOON) and metals
(PAXG/XAUT) are excluded — they are not crypto perps and do not trade the
same way, which would make the rehearsal misleading rather than different.

## What this does and does not establish

It rehearses the plumbing: arming, sizing, bracket placement, fee
accounting, reconciliation. It does not test the edge. Demo fills come from
a different book with different liquidity on different symbols, so a
profitable demo record is not evidence the same streams profit live.
