"""Plain-English names and rules for lab variant ids."""

SESSION = {"US": ("US-open", "the NYSE open (13:30 UTC in US summer time, 14:30 in winter), weekdays only",
                  "the NYSE close", "3 h 15 min after the open"),
           "LDN": ("London-open", "08:00 London time (07:00 UTC in British summer time, 08:00 in winter), weekdays only",
                   "16:30 London time", "4 h 15 min after the open"),
           "UTC": ("UTC-day", "00:00 UTC, every day", "00:00 UTC the next day", "12:00 UTC")}
FILTER = {"none": "", "trend": " Only in the direction of the last 10 days' move.",
          "nr4": " Only after a day whose range was the narrowest of the last four.",
          "tight": " Only when the opening range is narrower than its 20-day median."}


def describe(vid):
    p = vid.split("|")
    fam = p[0]
    if fam == "A":
        s, rmin, entry, stop, tgt, filt = p[1:]
        name, start, close, until = SESSION[s]
        title = f"{name} {rmin}-min range breakout"
        ent = {"close": "Enter at market after a 1-minute close outside the range",
               "stop": "Rest buy-stop and sell-stop orders at the two range edges; the first one touched is the trade",
               "retest": "After a 1-minute close outside the range, rest a limit order back at the broken edge (maker fill)"}[entry]
        st = "the far side of the range" if stop == "far" else "the middle of the range"
        tg = "no target" if tgt == "none" else f"a limit target at {tgt}x the risk"
        rules = (f"Opening range = the first {rmin} minutes from {start}. {ent}, until {until}. "
                 f"Stop at {st}, {tg}; exit at {close}.{FILTER[filt]}")
        return title, rules
    if fam == "B":
        chk, k, filt, stop = p[1:]
        when = "1 h after the NYSE open (weekdays)" if chk == "US1h" else f"{chk}:00 UTC"
        title = f"Trend-day follow at {when.split(' (')[0]}"
        rules = (f"At {when}, if price has moved at least {k} x the 14-day ATR from the 00:00 UTC open, follow the move at "
                 f"market until 00:00 UTC. Stop at {'the day open' if stop == 'open' else 'halfway back to the day open'}."
                 f"{FILTER[filt]}")
        return title, rules
    if fam == "C":
        x, basis, mode, legs = p[1:]
        title = f"Cross-coin {'momentum' if mode == 'mom' else 'reversal'} at {x}:00 UTC"
        what = "since 00:00 UTC" if basis == "day" else "over the last 24 hours"
        rules = (f"At {x}:00 UTC rank BTC, ETH, SOL, XRP, DOGE and BNB by return {what}. "
                 f"{'Buy' if mode == 'mom' else 'Sell'} the top {legs} and {'sell' if mode == 'mom' else 'buy'} the bottom {legs}, "
                 "$10,000 each, and close everything at 00:00 UTC. No stops.")
        return title, rules
    if fam == "F":
        k, entry, filt, stop = p[1:]
        title = f"Volatility breakout k={k}"
        ent = "after a 1-minute close beyond the line" if entry == "close" else "with a stop order resting at the line"
        rules = (f"Lines = 00:00 UTC open +/- {k} x yesterday's range. Trade the first line crossed, {ent}; exit at 00:00 UTC. "
                 f"{'Stop at the day open.' if stop == 'open' else 'No stop.'}{FILTER[filt]}")
        return title, rules
    if fam == "G":
        hh, d, side = p[1:]
        end = int(hh) + int(d)
        title = f"{'Long' if side == 'long' else 'Short'} {hh}:00-{end:02d}:00 UTC"
        rules = f"Every day, {'buy' if side == 'long' else 'sell'} at {hh}:00 UTC and close at {end:02d}:00 UTC. No stop."
        return title, rules
    return vid, ""
