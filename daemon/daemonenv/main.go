package daemonenv

import (
	"fmt"
	"path/filepath"

	"opensvc.com/opensvc/core/rawconfig"
)

var (
	// TODO use values from config
	RawPort  = 1214
	HttpPort = 1215

	HeaderNode        = "o-node"
	HeaderMultiplexed = "o-multiplexed"
)

func CertFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "certificate_chain")
}

func KeyFile() string {
	return filepath.Join(rawconfig.Paths.Certs, "private_key")
}

func UrlInetRaw() string {
	return fmt.Sprintf("raw://localhost:%d", RawPort)
}

func UrlInetHttp() string {
	return fmt.Sprintf("https://localhost:%d", HttpPort)
}

func UrlUxRaw() string {
	return "raw://" + PathUxRaw()
}

func UrlUxHttp() string {
	return "http://" + PathUxHttp()
}

func PathUxRaw() string {
	return filepath.Join(rawconfig.Paths.Var, "lsnr", "lsnr.sock")
}

func PathUxHttp() string {
	return filepath.Join(rawconfig.Paths.Var, "lsnr", "h2.sock")
}
