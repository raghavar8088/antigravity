package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A list endpoint must serialise as [] when empty, never null.
//
// d.recent is a nil slice until the first trade closes, and Go marshals nil as
// `null`. Every client treats /scalp/trades as an array and calls .map on it,
// so `null` threw a TypeError and took the entire Scalp Desk page down —
// "This page couldn't load", with no clue that an API had returned the wrong
// shape.
//
// The bug was latent from the first day and only surfaced when the trade
// history was cleared, because until then the desk had always closed at least
// one trade. Every empty-state path is like this: it is exercised for the first
// time long after the code is written, usually by an operator doing something
// legitimate.
func TestTradesPayload_EmptyIsAnArrayNotNull(t *testing.T) {
	var nilSlice []closedTrade

	// What the endpoint used to do.
	raw, err := json.Marshal(nilSlice)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != "null" {
		t.Fatalf("a nil slice marshalled as %s; this test's premise is wrong", raw)
	}

	// What it must do now.
	out := nilSlice
	if out == nil {
		out = []closedTrade{}
	}
	raw, err = json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != "[]" {
		t.Errorf("empty trades marshalled as %s, want []", raw)
	}
}

// The guard has to survive a rewrite of the handler, so it is asserted against
// the source: any `writeJSON` of a bare slice expression is the shape that
// caused this.
func TestTradesHandler_NormalisesBeforeWriting(t *testing.T) {
	src := readSourceFile(t, "main.go")
	i := strings.Index(src, `http.HandleFunc("/scalp/trades"`)
	if i < 0 {
		t.Fatal("/scalp/trades handler not found")
	}
	body := src[i:]
	if j := strings.Index(body, "http.HandleFunc("); j > 0 {
		body = body[:j]
	}
	if !strings.Contains(body, "[]closedTrade{}") {
		t.Error("the /scalp/trades handler no longer normalises a nil slice to an empty one; " +
			"clients call .map on this payload and null crashes the page")
	}
}
