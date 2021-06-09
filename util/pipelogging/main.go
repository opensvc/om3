// Package pipelogging provides functions to log io.Reader entries
package pipelogging

import (
	"bufio"
	"github.com/rs/zerolog"
	"io"
)

// LogWithPrefix scan 'reader' and log entries with zerolog level 'level'
// add 'prefix' to log message
// done <- true when all read is done
func LogWithPrefix(reader io.Reader, done chan bool, prefix string, log *zerolog.Logger, level zerolog.Level) {
	s := bufio.NewScanner(reader)
	for s.Scan() {
		log.WithLevel(level).Msgf("%v%v", prefix, s.Text())
	}
	done <- true
}
