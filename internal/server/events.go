package server

import (
	"database/sql"
	"os"

	"github.com/faciam-dev/gcfm/internal/events"
	"github.com/faciam-dev/gcfm/internal/logger"
	"github.com/faciam-dev/goquent/orm/driver"
)

// initEvents initializes the global events dispatcher.
func initEvents(db *sql.DB, dialect driver.Dialect, tablePrefix string) {
	evtConf, err := events.LoadConfig(os.Getenv("CF_EVENTS_CONFIG"))
	if err != nil {
		logger.L.Error("Failed to load events configuration", "err", err)
		os.Exit(1)
	}
	var sinks []events.Sink
	if wh := events.NewWebhookSink(evtConf.Sinks.Webhook); wh != nil {
		sinks = append(sinks, wh)
	}
	if rs, err := events.NewRedisSink(evtConf.Sinks.Redis); err == nil && rs != nil {
		sinks = append(sinks, rs)
	} else if err != nil {
		logger.L.Error("redis sink", "err", err)
	}
	if ks, err := events.NewKafkaSink(evtConf.Sinks.Kafka); err == nil && ks != nil {
		sinks = append(sinks, ks)
	} else if err != nil {
		logger.L.Error("kafka sink", "err", err)
	}
	events.Default = events.NewDispatcher(evtConf, &events.SQLDLQ{DB: db, Dialect: dialect, TablePrefix: tablePrefix}, sinks...)
}
