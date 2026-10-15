package commoncmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/uri"
)

func DataFromConfigHttp(u uri.T) ([]byte, error) {
	fpath, err := u.HttpFetch()
	if err != nil {
		return nil, err
	}
	defer os.Remove(fpath)
	return DataFromConfigFile(fpath)
}

func DataFromConfigFile(fpath string) ([]byte, error) {
	b, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	return b, err
}

func DataFromStdin() ([]byte, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return b, err
}

func DataFromTemplate(template string) ([]byte, error) {
	return nil, fmt.Errorf("todo: collector requester")
}

// CreateObjectConfig creates the object at p from the configuration b,
// through the api, which checks it as any configuration it writes, and
// answers once the daemon knows the object, so the command run next finds
// it.
//
// The daemon leaves behind what a configuration copied from another object
// recorded of itself, unless restore, and applies the keyword operations
// after, so a keyword they name is kept. With force, an object that exists
// already has its configuration replaced, with the same treatment.
func CreateObjectConfig(ctx context.Context, c *client.T, p naming.Path, b []byte, keywords []string, restore, force bool) error {
	waitLocal := true
	params := api.PostObjectConfigFileParams{WaitLocal: &waitLocal}
	if restore {
		params.Restore = &restore
	}
	if len(keywords) > 0 {
		params.Set = &keywords
	}
	resp, err := c.PostObjectConfigFileWithBodyWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params, "application/octet-stream", bytes.NewReader(b))
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusNoContent:
		fmt.Printf("%s: created\n", p)
		return nil
	case http.StatusConflict:
		if !force {
			return fmt.Errorf("%s already exists", p)
		}
		return replaceObjectConfig(ctx, c, p, b, keywords, restore)
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusRequestTimeout:
		// Created, and not known to the daemon yet: the command run next
		// may not find it.
		return fmt.Errorf("%s: %s", p, resp.JSON408.Detail)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
}

// replaceObjectConfig replaces the configuration of the object at p by b,
// as a create with force does: what b recorded of the thing it was made for
// is left behind unless restore, as on a creation.
func replaceObjectConfig(ctx context.Context, c *client.T, p naming.Path, b []byte, keywords []string, restore bool) error {
	_, b, err := object.CreatedConfig(p, b, restore, keyop.ParseOps(keywords)...)
	if err != nil {
		return err
	}
	resp, err := c.PutObjectConfigFileWithBodyWithResponse(ctx, p.Namespace, p.Kind, p.Name, nil, "application/octet-stream", bytes.NewReader(b))
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusNoContent:
		fmt.Printf("%s: replaced\n", p)
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %s", p, *resp.JSON404)
	case http.StatusConflict:
		return fmt.Errorf("%s: %s", p, *resp.JSON409)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
}

// CreatedConfigData returns the configurer reading the configuration b of
// an object created at p, as the api writes it, to tell what its validation
// finds before it is sent.
func CreatedConfigData(p naming.Path, b []byte, keywords []string, restore bool) (object.Configurer, error) {
	oc, _, err := object.CreatedConfig(p, b, restore, keyop.ParseOps(keywords)...)
	return oc, err
}
