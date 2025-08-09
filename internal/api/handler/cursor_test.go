package handler

import (
	"testing"
	"time"
)

func TestEncodeDecodeCursor(t *testing.T) {
	ts := time.Date(2025, 8, 7, 16, 9, 34, 0, time.UTC)
	id := int64(172)
	cur := encodeCursor(ts, id)
	gotTs, gotID, err := decodeCursor(cur)
	if err != nil {
		t.Fatalf("decodeCursor error = %v", err)
	}
	if !gotTs.Equal(ts) {
		t.Errorf("timestamp mismatch: got %v want %v", gotTs, ts)
	}
	if gotID != id {
		t.Errorf("id mismatch: got %d want %d", gotID, id)
	}
}

func TestDecodeCursorBadFormat(t *testing.T) {
	if _, _, err := decodeCursor("bad"); err == nil {
		t.Fatal("expected error for bad cursor")
	}
}
