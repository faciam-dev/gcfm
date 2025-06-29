package main

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	var mode string
	var dsn string
	var channel string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run notifier daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if mode != "redis" {
				return errors.New("only redis mode supported")
			}
			opt, err := redis.ParseURL(dsn)
			if err != nil {
				return err
			}
			client := redis.NewClient(opt)
			sub := client.Subscribe(context.Background(), channel)
			for msg := range sub.Channel() {
				cmd.Printf("event: %s\n", msg.Payload)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "redis", "broker mode")
	cmd.Flags().StringVar(&dsn, "dsn", "", "redis DSN")
	cmd.Flags().StringVar(&channel, "channel", "cf-events", "channel name")
	cmd.MarkFlagRequired("dsn")
	return cmd
}
