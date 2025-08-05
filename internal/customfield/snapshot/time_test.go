package snapshot

import (
	"testing"
	"time"
)

func TestParseDBTime(t *testing.T) {
	s := "2024-01-02 15:04:05"
	want, _ := time.Parse("2006-01-02 15:04:05", s)

	got, err := parseDBTime([]byte(s))
	if err != nil {
		t.Fatalf("parse []byte: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("byte parse mismatch: %v != %v", got, want)
	}

	got, err = parseDBTime(s)
	if err != nil {
		t.Fatalf("parse string: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("string parse mismatch: %v != %v", got, want)
	}

	got, err = parseDBTime(want)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("time parse mismatch: %v != %v", got, want)
	}
}
