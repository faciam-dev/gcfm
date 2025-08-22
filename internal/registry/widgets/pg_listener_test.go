package widgets

import (
	"context"
	"database/sql"
	"testing"
	"time"

	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
)

type stubRepo struct {
	row widgetsrepo.Row
	err error
}

func (s stubRepo) List(context.Context, widgetsrepo.Filter) ([]widgetsrepo.Row, int, error) {
	return nil, 0, nil
}
func (s stubRepo) GetETagAndLastMod(context.Context, widgetsrepo.Filter) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (s stubRepo) Upsert(context.Context, widgetsrepo.Row) error { return nil }
func (s stubRepo) Remove(context.Context, string) error          { return nil }
func (s stubRepo) GetByID(ctx context.Context, id string) (widgetsrepo.Row, error) {
	return s.row, s.err
}

type stubReg struct {
	upserted []Widget
	removed  []string
}

func (s *stubReg) List(context.Context, Options) ([]Widget, int, string, time.Time, error) {
	return nil, 0, "", time.Time{}, nil
}
func (s *stubReg) Upsert(ctx context.Context, w Widget) error {
	s.upserted = append(s.upserted, w)
	return nil
}
func (s *stubReg) Remove(ctx context.Context, id string) error {
	s.removed = append(s.removed, id)
	return nil
}
func (s *stubReg) ApplyDiff(context.Context, []Widget, []string) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (s *stubReg) Subscribe() (<-chan Event, func()) { ch := make(chan Event); return ch, func() {} }

func TestPGListenerApplyUpsert(t *testing.T) {
	repo := stubRepo{row: widgetsrepo.Row{ID: "a", Name: "A", Version: "1", Type: "widget", Scopes: []string{"system"}, UpdatedAt: time.Now()}}
	reg := &stubReg{}
	l := PGListener{Repo: repo, Reg: reg}
	l.apply(context.Background(), "a")
	if len(reg.upserted) != 1 || reg.upserted[0].ID != "a" {
		t.Fatalf("upsert not called: %+v", reg.upserted)
	}
}

func TestPGListenerApplyRemove(t *testing.T) {
	repo := stubRepo{err: sql.ErrNoRows}
	reg := &stubReg{}
	l := PGListener{Repo: repo, Reg: reg}
	l.apply(context.Background(), "b")
	if len(reg.removed) != 1 || reg.removed[0] != "b" {
		t.Fatalf("remove not called: %+v", reg.removed)
	}
}
