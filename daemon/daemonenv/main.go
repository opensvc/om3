package daemonenv

import (
	"fmt"
	"os/user"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/rawconfig"
)

var (
	HttpPort = 1215

	DrainChanDuration = 40 * time.Millisecond

	// ReadyDuration define the minimum time to wait during the startup of an instance object by imon
	// The ready duration impacts the durations involved during daemon cluster split analyse (see nmon spit
	// detection details).
	ReadyDuration = 5 * time.Second

	// Username is the current username, or "root" if user.Current has error
	Username string

	// Groupname is the current group name from user.Current, or "root" if user.LookupGroupId has error
	Groupname string

	// BaseHttpSock is the basename of http listener unix socket
	BaseHttpSock = "http.sock"
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

func UrlInetHttp() string {
	return fmt.Sprintf("https://localhost:%d", HttpPort)
}

func UrlHttpNode(node string) string {
	return fmt.Sprintf("https://%s:%d", node, HttpPort)
}

func UrlUxHttp() string {
	return "http://" + PathUxHttp()
}

func PathUxProfile() string {
	return filepath.Join(rawconfig.Paths.Lsnr, "profile.sock")
}

func PathUxHttp() string {
	return filepath.Join(rawconfig.Paths.Lsnr, BaseHttpSock)
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
