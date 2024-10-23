package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
)

func createTempRemoteConfig(p naming.Path, c *client.T) (string, error) {
	var (
		err  error
		buff []byte
		f    *os.File
	)
	if buff, err = fetchConfig(p, c); err != nil {
		return "", err
	}
	if f, err = os.CreateTemp("", ".opensvc.remote.config.*"); err != nil {
		return "", err
	}
	filename := f.Name()
	if _, err = f.Write(buff); err != nil {
		os.Remove(filename)
		return "", err
	}
	return filename, nil
}

func fetchConfig(p naming.Path, c *client.T) ([]byte, error) {
	resp, err := c.GetObjectConfigFileWithResponse(context.Background(), p.Namespace, p.Kind, p.Name)
	if err != nil {
		return nil, err
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get object %s file from %s: %s", p, c.URL(), resp.Status())
	}
	return resp.Body, nil
}

func putConfig(p naming.Path, fName string, c *client.T) (err error) {
	file, err := os.Open(fName)
	if err != nil {
		return err
	}
	defer file.Close()
	resp, err := c.PutObjectConfigFileWithBodyWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, "application/octet-stream", file)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("put object %s file from %s: %s", p, c.URL(), resp.Status()+string(resp.Body))
	}
}
