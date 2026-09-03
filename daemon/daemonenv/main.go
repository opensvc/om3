package daemonenv

import (
	"fmt"
	"os/user"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// The listeners are named the same wherever they are named: the audit
// subsystem a client selects with "om daemon audit --sub", the label the
// listener subscribes to daemon control messages with, and the name the
// api takes in the path of the listener actions. They used to differ,
// so an action addressed to the name the audit taxonomy uses was
// accepted, queued, and matched by no subscriber.
const (
	// ListenerNameUX is the listener serving the unix socket.
	ListenerNameUX = "api.ux"

	// ListenerNameInet is the listener serving the tcp port.
	ListenerNameInet = "api.inet"

	// ListenerNameFamily is the parent of both, for a client selecting
	// the whole api rather than one of its listeners.
	ListenerNameFamily = "api"
)

// ListenerNames are the listeners an api action may be addressed to.
var ListenerNames = []string{ListenerNameUX, ListenerNameInet}

// LifecycleListenerNames are the listeners a start, stop or restart may be
// addressed to.
//
// The unix socket listener is not one of them: it lives as long as the daemon
// does, and the request asking for its restart travels through it. It used to
// accept the three actions, log that it ignored them, and answer the caller
// that they were queued.
var LifecycleListenerNames = []string{ListenerNameInet}

var (
	HTTPPort = 1215

	DrainChanDuration = 40 * time.Millisecond

	// ImonDelayDuration is the minimum for new imon publication
	ImonDelayDuration = 250 * time.Millisecond

	// ReadyDuration define the minimum time to wait during the startup of an instance object by imon
	// The ready duration impacts the durations involved during daemon cluster split analyse (see nmon spit
	// detection details).
	ReadyDuration = 5 * time.Second

	// Username is the current username, or "root" if user.Current has error
	Username string

	// Groupname is the current group name from user.Current, or "root" if user.LookupGroupId has error
	Groupname string

	// HTTPUnixFileBasename is the basename of http listener unix socket
	HTTPUnixFileBasename = "http.sock"

	// HeartbeatStatusRefreshMaximumInterval is the maximum interval for refreshing daemon heartbeat statistics
	HeartbeatStatusRefreshMaximumInterval = 60 * time.Second
)

var (
	// SubQSSmall is the daemon subscription small queue size
	SubQSSmall uint64 = 200
	// SubQSSmall is the daemon subscription medium queue size
	SubQSMedium uint64 = 1000
	// SubQSSmall is the daemon subscription large queue size
	SubQSLarge uint64 = 20000
	// SubQSHuge is the daemon subscription huge queue size
	SubQSHuge uint64 = 40000
)

func CAKeyFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "ca_private_key")
}

func CACertChainFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "ca_certificate_chain")
}

func CAsCertFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "ca_certificates")
}

func CertChainFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "certificate_chain")
}

func CertFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "certificate")
}

func KeyFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "private_key")
}

func ProfileUnixFile() string {
	return filepath.Join(rawconfig.Paths.Lsnr, "profile.sock")
}

func HTTPUnixFile() string {
	return filepath.Join(rawconfig.Paths.Lsnr, HTTPUnixFileBasename)
}

func HTTPLocalURL() string {
	return fmt.Sprintf("https://localhost:%d", HTTPPort)
}

func HTTPNodeURL(node string) string {
	return fmt.Sprintf("https://%s:%d", node, HTTPPort)
}

func HTTPNodeAndPortURL(node, port string) string {
	return fmt.Sprintf("https://%s:%s", node, port)
}

func HTTPUnixURL() string {
	return "http://" + HTTPUnixFile()
}

func init() {
	if currentUser, err := user.Current(); err != nil {
		Username = "root"
		Groupname = "root"
		return
	} else {
		Username = currentUser.Username
		if grp, err := user.LookupGroupId(currentUser.Gid); err != nil {
			Groupname = "root"
			return
		} else {
			Groupname = grp.Name
		}
	}
}
