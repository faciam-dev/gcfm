package unit_test

import (
	"testing"
	"time"

	"github.com/faciam-dev/gcfm/internal/api/handler"
)

func TestParseAuditTime(t *testing.T) {
	s := "2024-01-02 15:04:05"
	want, _ := time.Parse("2006-01-02 15:04:05", s)

	got, err := handler.ParseAuditTime([]byte(s))
	if err != nil {
		t.Fatalf("parse []byte: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("byte parse mismatch: %v != %v", got, want)
	}

	got, err = handler.ParseAuditTime(s)
	if err != nil {
		t.Fatalf("parse string: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("string parse mismatch: %v != %v", got, want)
	}

	got, err = handler.ParseAuditTime(want)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("time parse mismatch: %v != %v", got, want)
	}

	micro := "2024-01-02 15:04:05.123456"
	wantMicro, _ := time.Parse("2006-01-02 15:04:05.000000", micro)
	got, err = handler.ParseAuditTime(micro)
	if err != nil {
		t.Fatalf("parse micro: %v", err)
	}
	if !got.Equal(wantMicro) {
		t.Fatalf("micro parse mismatch: %v != %v", got, wantMicro)
	}

	if _, err := handler.ParseAuditTime(nil); err != nil {
		t.Fatalf("parse nil: %v", err)
	}
}
