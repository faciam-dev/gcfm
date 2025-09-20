package monitordb

import "testing"

func TestHasDatabaseNameMongo(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "with database", dsn: "mongodb://user:pass@localhost:27017/sample", want: true},
		{name: "srv uri", dsn: "mongodb+srv://example.com/mydb?replicaSet=rs0", want: true},
		{name: "auth source", dsn: "mongodb://localhost:27017/?authSource=admin", want: true},
		{name: "missing", dsn: "mongodb://localhost:27017/", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasDatabaseName("mongo", tt.dsn); got != tt.want {
				t.Fatalf("HasDatabaseName() = %v, want %v", got, tt.want)
			}
		})
	}
}
