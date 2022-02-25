package daemonenv

var (
	RawPort  = "1224"
	HttpPort = "1225"

	PathUxRaw  = "/tmp/lsnr_ux"
	PathUxHttp = "/tmp/lsnr_ux_h2"

	UrlUxRaw    = "raw://" + PathUxRaw
	UrlUxHttp   = "http://" + PathUxHttp
	UrlInetHttp = "https://localhost:" + HttpPort
	UrlInetRaw  = "raw://localhost:" + RawPort

	CertFile = "/tmp/certificate_chain"
	KeyFile  = "/tmp/private_key"

	HeaderNode = "o-node"
)
