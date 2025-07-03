package events

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	dbcmd "github.com/faciam-dev/gcfm/cmd/fieldctl/db"
	notify "github.com/faciam-dev/gcfm/internal/events"
	"github.com/spf13/cobra"
)

// NewCmd creates the events command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "events"}
	cmd.AddCommand(newListFailedCmd())
	cmd.AddCommand(newRetryCmd())
	cmd.AddCommand(newTailCmd())
	return cmd
}

func openDB(f dbcmd.DBFlags) (*sql.DB, error) {
	if f.Driver == "" {
		d, err := dbcmd.DetectDriver(f.DSN)
		if err != nil {
			return nil, err
		}
		f.Driver = d
	}
	return sql.Open(f.Driver, f.DSN)
}

func newListFailedCmd() *cobra.Command {
	var flags dbcmd.DBFlags
	cmd := &cobra.Command{
		Use:   "ls-failed",
		Short: "List failed events",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.QueryContext(cmd.Context(), "SELECT id,name,payload,attempts,last_error FROM gcfm_events_failed ORDER BY id")
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name, payload, last string
				var attempts int
				if err := rows.Scan(&id, &name, &payload, &attempts, &last); err != nil {
					return err
				}
				cmd.Printf("%d\t%s\t%d\t%s\n", id, name, attempts, last)
			}
			return rows.Err()
		},
	}
	flags.AddFlags(cmd)
	cmd.MarkFlagRequired("db")
	return cmd
}

func newRetryCmd() *cobra.Command {
	var flags dbcmd.DBFlags
	var id int64
	var redisDSN, channel string
	cmd := &cobra.Command{
		Use:   "retry",
		Short: "Retry failed event by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			var payload string
			err = db.QueryRowContext(cmd.Context(), "SELECT payload FROM gcfm_events_failed WHERE id=?", id).Scan(&payload)
			if err != nil {
				return err
			}
			var evt notify.Event
			if err := json.Unmarshal([]byte(payload), &evt); err != nil {
				return err
			}
			opt, err := redis.ParseURL(redisDSN)
			if err != nil {
				return err
			}
			cli := redis.NewClient(opt)
			sink := &notify.RedisSink{Client: cli, Channel: channel}
			if err := sink.Emit(cmd.Context(), evt); err != nil {
				return err
			}
			if _, err := db.ExecContext(cmd.Context(), "DELETE FROM gcfm_events_failed WHERE id=?", id); err != nil {
				return err
			}
			cmd.Println("re-dispatched", id)
			return nil
		},
	}
	flags.AddFlags(cmd)
	cmd.Flags().Int64Var(&id, "id", 0, "event id")
	cmd.Flags().StringVar(&redisDSN, "redis", "", "redis DSN")
	cmd.Flags().StringVar(&channel, "channel", "cf-events", "channel name")
	cmd.MarkFlagRequired("db")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("redis")
	return cmd
}

func newTailCmd() *cobra.Command {
	var dsn, channel string
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail events from redis",
		RunE: func(cmd *cobra.Command, args []string) error {
			opt, err := redis.ParseURL(dsn)
			if err != nil {
				return err
			}
			client := redis.NewClient(opt)
			sub := client.Subscribe(context.Background(), channel)
			if _, err := sub.Receive(cmd.Context()); err != nil {
				return err
			}
			for msg := range sub.Channel() {
				cmd.Println(msg.Payload)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dsn, "dsn", "", "redis DSN")
	cmd.Flags().StringVar(&channel, "channel", "cf-events", "channel name")
	cmd.MarkFlagRequired("dsn")
	return cmd
}
