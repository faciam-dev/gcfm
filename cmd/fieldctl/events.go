package main

import (
	eventscmd "github.com/faciam-dev/gcfm/cmd/fieldctl/events"
	"github.com/spf13/cobra"
)

func newEventsCmd() *cobra.Command {
	return eventscmd.NewCmd()
}
