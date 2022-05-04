package object

import (
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/logging"
)

// ConfigureLog configures the zerolog logger with console writer and lumberjack rotating file writer.
func ConfigureLog() *logging.Logger {
	return logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             rawconfig.Node.Paths.Log,
		Filename:              "node",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	})
}
