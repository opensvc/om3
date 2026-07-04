package sgcp

type (
	// AuthInfo represents authentication details required for API access.
	AuthInfo struct {
		AccountID    string
		ClientID     string
		ClientSecret string

		// Signature identifies auth info
		Signature string
	}
)
