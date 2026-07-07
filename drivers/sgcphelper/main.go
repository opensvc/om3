package sgcphelper

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/sgcp"
)

type (
	GetAuthInfoFromDatastorePather struct{}
)

// GetAuthInfo retrieves authentication information from the specified datastore path and returns an AuthInfo struct.
func (g *GetAuthInfoFromDatastorePather) GetAuthInfo(datastorePath string) (*sgcp.AuthInfo, error) {
	var (
		ds       object.DataStore
		dsValues [][]byte
	)
	secPath, err := naming.ParsePath(datastorePath)
	if err != nil {
		return nil, fmt.Errorf("parse secret path: %w", err)
	}

	ds, err = object.NewSec(secPath, object.WithVolatile(true))
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", secPath, err)
	}

	dsValues, err = ds.DecodeKeys("account_id", "client_id", "client_secret")
	if err != nil {
		return nil, fmt.Errorf("decode auth keys from %s: %w", secPath, err)
	}
	authInfo := &sgcp.AuthInfo{
		AccountID:    string(dsValues[0]),
		ClientID:     string(dsValues[1]),
		ClientSecret: string(dsValues[2]),

		Signature: fmt.Sprintf("sgcp-authinfo-%s-%s", secPath.Namespace, secPath.Name),
	}
	return authInfo, nil
}
