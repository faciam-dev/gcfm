package widgets

import (
	"context"
	"database/sql"
)

// Notifier sends notifications when widgets change.
type Notifier struct {
	DB *sql.DB
}

// NotifyWidgetChanged emits a database notification.
func (n *Notifier) NotifyWidgetChanged(ctx context.Context, id string) error {
	if n.DB == nil {
		return nil
	}
	_, err := n.DB.ExecContext(ctx, `SELECT pg_notify('widgets_changed', $1)`, id)
	return err
}
