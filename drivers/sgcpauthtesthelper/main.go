package sgcpauthtesthelper

import "github.com/opensvc/om3/v3/util/sgcp"

type (
	// MockGetAuthInfoProvider is a mock implementation of the GetAuthInfoProvider interface for testing purposes.
	MockGetAuthInfoProvider struct {
		sgcp.AuthInfo
	}

	getAuthInfoProvider interface {
		GetAuthInfo(string) (*sgcp.AuthInfo, error)
	}
)

var (
	// _ ensures MockGetAuthInfoProvider statically implements the getAuthInfoProvider interface.
	_ getAuthInfoProvider = &MockGetAuthInfoProvider{}
)

// NewMockGetAuthInfoProvider creates a new mock instance of MockGetAuthInfoProvider populated with test AuthInfo data.
func NewMockGetAuthInfoProvider(s string) *MockGetAuthInfoProvider {
	var a sgcp.AuthInfo
	switch s {
	case "id1":
		a = sgcp.AuthInfo{
			AccountID:    "account_1",
			ClientID:     "client_id_1",
			ClientSecret: "client_secret_1",
			Signature:    "1",
		}
	default:
		a = sgcp.AuthInfo{
			AccountID:    "account_default",
			ClientID:     "client_id_efault",
			ClientSecret: "client_secret_default",
			Signature:    "default",
		}
	}
	return &MockGetAuthInfoProvider{AuthInfo: a}
}

// GetAuthInfo retrieves the authentication details of the MockGetAuthInfoProvider as a pointer to sgcp.AuthInfo.
func (a *MockGetAuthInfoProvider) GetAuthInfo(string) (*sgcp.AuthInfo, error) {
	return &a.AuthInfo, nil
}
