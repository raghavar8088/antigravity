package omsv3

import "encoding/json"

// unmarshalSilent deserialises JSON payload into dst.
// Returns true on success, false on any error. Used by projections that must
// never panic on malformed historical events.
func unmarshalSilent(payload []byte, dst any) bool {
	if len(payload) == 0 {
		return false
	}
	return json.Unmarshal(payload, dst) == nil
}
