package daemonenv

var (
	// TODO use values from config
	RawPort  = "1214"
	HttpPort = "1215"

	PathUxRaw  = "/var/lib/opensvc/lsnr/lsnr.sock"
	PathUxHttp = "/var/lib/opensvc/lsnr/h2.sock"

	UrlUxRaw    = "raw://" + PathUxRaw
	UrlUxHttp   = "http://" + PathUxHttp
	UrlInetHttp = "https://localhost:" + HttpPort
	UrlInetRaw  = "raw://localhost:" + RawPort

	CertFile = "/tmp/certificate_chain"
	KeyFile  = "/tmp/private_key"

	HeaderNode        = "o-node"
	HeaderMultiplexed = "o-multiplexed"
)
