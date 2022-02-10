package daemonenv

var (
	HttpPort = "1225"

	PathUxRaw  = "/tmp/lsnr_ux"
	PathUxHttp = "/tmp/lsnr_ux_h2"

	UrlUxRaw    = "raw://" + PathUxRaw
	UrlUxHttp   = "http://" + PathUxHttp
	UrlInetHttp = "https://localhost:" + HttpPort

	CertFile = "/tmp/certificate_chain"
	KeyFile  = "/tmp/private_key"
)
