package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
)

// Authenticator provides standard REST signing payloads for crypto exchanges.
type Authenticator struct {
	APIKey    string
	APISecret string
}

func NewAuthenticator(key, secret string) *Authenticator {
	return &Authenticator{
		APIKey:    key,
		APISecret: secret,
	}
}

// SignBinanceRequest generates the required HMAC SHA256 signature and appends it to the query string.
// Binance requires queries to be sorted, combined, and then signed using the API Secret.
func (a *Authenticator) SignBinanceRequest(params url.Values) (string, error) {
	if a.APISecret == "" {
		return "", fmt.Errorf("AUTH_ERROR: Missing API Secret to sign request")
	}

	rawQuery := params.Encode()
	
	mac := hmac.New(sha256.New, []byte(a.APISecret))
	_, err := mac.Write([]byte(rawQuery))
	if err != nil {
		return "", err
	}

	signature := hex.EncodeToString(mac.Sum(nil))
	
	// Safely attach the newly minted cryptographic string to the outgoing payload
	params.Add("signature", signature)

	return params.Encode(), nil
}
