package handler

import "testing"

func TestMongoDatabaseName(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		fallback string
		want     string
		wantErr  bool
	}{
		{name: "fallback", dsn: "mongodb://localhost:27017/ignored", fallback: "manual", want: "manual"},
		{name: "path", dsn: "mongodb://localhost:27017/sample", want: "sample"},
		{name: "trim slash", dsn: "mongodb://localhost:27017//sample", want: "sample"},
		{name: "authSource", dsn: "mongodb://localhost:27017/?authSource=admin", want: "admin"},
		{name: "missing", dsn: "mongodb://localhost:27017/", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mongoDatabaseName(tt.dsn, tt.fallback)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("mongoDatabaseName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("mongoDatabaseName() = %q, want %q", got, tt.want)
			}
		})
	}
}
