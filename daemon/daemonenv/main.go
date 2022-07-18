package daemonenv

import (
	"path/filepath"

	"opensvc.com/opensvc/core/rawconfig"
)

var (
	// TODO use values from config
	RawPort  = "1214"
	HttpPort = "1215"

	PathUxRaw  = filepath.Join(rawconfig.Paths.Var, "lsnr", "lsnr.sock")
	PathUxHttp = filepath.Join(rawconfig.Paths.Var, "lsnr", "h2.sock")

	UrlUxRaw    = "raw://" + PathUxRaw
	UrlUxHttp   = "http://" + PathUxHttp
	UrlInetHttp = "https://localhost:" + HttpPort
	UrlInetRaw  = "raw://localhost:" + RawPort

	CertFile = filepath.Join(rawconfig.Paths.Certs, "certificate_chain")
	KeyFile  = filepath.Join(rawconfig.Paths.Certs, "private_key")

	HeaderNode        = "o-node"
	HeaderMultiplexed = "o-multiplexed"
)
