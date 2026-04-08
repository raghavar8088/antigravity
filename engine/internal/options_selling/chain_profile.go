package options_selling

import (
	"math"
	"time"
)

func buildChainForProfile(spot float64, expiry time.Time, baseIV float64, profile MarketProfile) []ChainRow {
	cfg := profile.ChainConfig
	atmStrike := roundToIncrement(spot, cfg.StrikeIncrement)
	dte := int(math.Max(1, math.Ceil(time.Until(expiry).Hours()/24)))

	numStrikes := cfg.NumStrikes
	if numStrikes <= 0 {
		numStrikes = 20
	}
	increment := cfg.StrikeIncrement
	if increment <= 0 {
		increment = 500
	}

	rows := make([]ChainRow, 0, numStrikes*2+1)
	for i := -numStrikes; i <= numStrikes; i++ {
		strike := atmStrike + float64(i)*increment
		if strike <= 0 {
			continue
		}

		moneynessPC := (strike - spot) / spot * 100
		callIV := smileIVForProfile(baseIV, spot, strike, Call, profile)
		putIV := smileIVForProfile(baseIV, spot, strike, Put, profile)

		callRes := PriceOption(spot, strike, expiry, callIV, Call)
		putRes := PriceOption(spot, strike, expiry, putIV, Put)

		logMoneyness := math.Log(strike / spot)
		callBid, callAsk := bidAskForConfig(callRes.Premium, logMoneyness, cfg)
		putBid, putAsk := bidAskForConfig(putRes.Premium, logMoneyness, cfg)

		callOI := simulateOIForConfig(strike, spot, dte, cfg)
		putOI := simulateOIForConfig(strike+1, spot, dte, cfg)

		rows = append(rows, ChainRow{
			Strike:      strike,
			IsATM:       i == 0,
			MoneynessPC: moneynessPC,
			Call: ChainLeg{
				IV:     callIV * 100,
				Delta:  callRes.Delta,
				Gamma:  callRes.Gamma,
				Theta:  callRes.Theta,
				Vega:   callRes.Vega,
				Mark:   callRes.Premium,
				Bid:    callBid,
				Ask:    callAsk,
				OI:     callOI,
				Volume: simulateVolumeForConfig(callOI, strike, cfg),
				IsITM:  strike < spot,
			},
			Put: ChainLeg{
				IV:     putIV * 100,
				Delta:  putRes.Delta,
				Gamma:  putRes.Gamma,
				Theta:  putRes.Theta,
				Vega:   putRes.Vega,
				Mark:   putRes.Premium,
				Bid:    putBid,
				Ask:    putAsk,
				OI:     putOI,
				Volume: simulateVolumeForConfig(putOI, strike+0.5, cfg),
				IsITM:  strike > spot,
			},
		})
	}

	return rows
}
