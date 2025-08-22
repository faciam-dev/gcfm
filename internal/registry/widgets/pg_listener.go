package widgets

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	widgetsrepo "github.com/faciam-dev/gcfm/internal/repository/widgets"
	"github.com/faciam-dev/gcfm/internal/util"
	"github.com/lib/pq"
)

type PGListener struct {
	ConnString string
	Repo       widgetsrepo.Repo
	Reg        Registry
	Logger     *slog.Logger
}

func NewPGListener(conn string, repo widgetsrepo.Repo, reg Registry, logger *slog.Logger) *PGListener {
	return &PGListener{ConnString: conn, Repo: repo, Reg: reg, Logger: logger}
}

func (l *PGListener) Start(ctx context.Context) (func(), error) {
	listener := pq.NewListener(l.ConnString, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil && l.Logger != nil {
			l.Logger.Error("pg listener", "err", err)
		}
	})
	if err := listener.Listen("widgets_changed"); err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				listener.Close()
				return
			case n := <-listener.Notify:
				if n == nil {
					continue
				}
				l.apply(ctx, n.Extra)
			}
		}
	}()
	return func() { listener.Close() }, nil
}

func (l *PGListener) apply(ctx context.Context, id string) {
	row, err := l.Repo.GetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			_ = l.Reg.Remove(ctx, id)
		} else if l.Logger != nil {
			l.Logger.Warn("repo get", "id", id, "err", err)
		}
		return
	}
	w := Widget{
		ID:           row.ID,
		Name:         row.Name,
		Version:      row.Version,
		Type:         row.Type,
		Scopes:       row.Scopes,
		Enabled:      row.Enabled,
		Description:  util.Deref(row.Description),
		Capabilities: row.Capabilities,
		Homepage:     util.Deref(row.Homepage),
		Meta:         row.Meta,
		Tenants:      row.Tenants,
		UpdatedAt:    row.UpdatedAt,
	}
	if err := l.Reg.Upsert(ctx, w); err != nil && l.Logger != nil {
		l.Logger.Warn("registry upsert", "id", id, "err", err)
	}
}
