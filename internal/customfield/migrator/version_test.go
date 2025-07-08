package migrator

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

func TestIsSyntaxErrMySQL(t *testing.T) {
	err := &mysql.MySQLError{Number: 1064, Message: "syntax"}
	if !isSyntaxErr(err) {
		t.Fatalf("expected true for mysql syntax error")
	}
}

func TestIsSyntaxErrPostgres(t *testing.T) {
	err := &pq.Error{Code: "42601"}
	if !isSyntaxErr(err) {
		t.Fatalf("expected true for postgres syntax error")
	}
}

func TestIsSyntaxErrOther(t *testing.T) {
	if isSyntaxErr(errors.New("syntax error")) {
		t.Fatalf("expected false for generic error")
	}
}
