package hbcfg

import (
	"fmt"
	"sync"

	"github.com/opensvc/om3/v3/util/plog"
)

var (
	warnedMu sync.Mutex
	warned   = make(map[string]string)
)

// warnedMax bounds the heartbeats remembered: beyond, they are forgotten,
// their warnings logged once again.
const warnedMax = 256

// WarnOnce logs a warning about the configuration of the heartbeat name the
// first time it is the one of the heartbeat, at the debug level after: the
// daemon configures the heartbeats again at each retry of the failed ones,
// every 30s, and a warning about a configuration nobody changed repeated
// that often is noise. A changed warning is logged again.
func WarnOnce(log *plog.Logger, name, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	warnedMu.Lock()
	if _, ok := warned[name]; !ok && len(warned) >= warnedMax {
		clear(warned)
	}
	seen := warned[name] == msg
	warned[name] = msg
	warnedMu.Unlock()
	if seen {
		log.Debugf("%s", msg)
	} else {
		log.Warnf("%s", msg)
	}
}
