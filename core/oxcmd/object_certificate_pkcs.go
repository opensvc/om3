package oxcmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectCertificatePKCS struct {
		OptsGlobal
	}
)

func (t *CmdObjectCertificatePKCS) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	ctx := context.Background()
	c, err := client.New()
	if err != nil {
		return err
	}
	paths, err := objectselector.New(
		mergedSelector,
		objectselector.WithClient(c),
	).Expand()
	if err != nil {
		return err
	}
	if len(paths) != 1 {
		return fmt.Errorf("the pkcs command must be executed on a single object, %d selected", len(paths))
	}

	path := paths[0]

	password, err := commoncmd.ReadPasswordFromStdinOrPrompt("Password: ")
	if err != nil {
		return err
	}

	privateKeyBytes, err := decode(ctx, c, path, "private_key")
	if err != nil {
		return err
	}

	certificateChainBytes, err := decode(ctx, c, path, "certificate_chain")
	if err != nil {
		return err
	}

	b, err := object.PKCS(privateKeyBytes, certificateChainBytes, password)
	_, err = io.Copy(os.Stdout, bytes.NewReader(b))
	return err
}

func decode(ctx context.Context, c *client.T, path naming.Path, key string) ([]byte, error) {
	params := api.GetObjectDataKeyParams{
		Name: key,
	}
	response, err := c.GetObjectDataKeyWithResponse(ctx, path.Namespace, path.Kind, path.Name, &params)
	if err != nil {
		return nil, err
	}
	switch response.StatusCode() {
	case http.StatusOK:
		return response.Body, err
	case http.StatusBadRequest:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON400)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON401)
	case http.StatusForbidden:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON403)
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON500)
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON404)
	default:
		return nil, fmt.Errorf("%s: unexpected response: %s", path, response.Status())
	}
}
