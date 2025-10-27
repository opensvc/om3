package oxcmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdObjectEnter struct {
		ObjectSelector string
		RID            string
		NodeSelector   string
	}
)

func (t *CmdObjectEnter) Run(kind string) error {
	if t.NodeSelector == "" {
		if clientcontext.IsSet() {
			return fmt.Errorf("--node must be set")
		}
		t.NodeSelector = hostname.Hostname()
	}
	path, err := naming.ParsePath(t.ObjectSelector)
	if err != nil {
		return err
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	for _, nodename := range nodenames {
		params := api.PostInstanceResourceConsoleParams{}
		if t.RID != "" {
			params.Rid = &t.RID
		}
		response, err := c.PostInstanceResourceConsoleWithResponse(ctx, nodename, path.Namespace, path.Kind, path.Name, &params)
		if err != nil {
			return err
		}
		if response.StatusCode() != http.StatusCreated {
			switch response.StatusCode() {
			case 400:
				return fmt.Errorf("%s: node %s: %s", path, nodename, *response.JSON400)
			case 401:
				return fmt.Errorf("%s: node %s: %s", path, nodename, *response.JSON401)
			case 403:
				return fmt.Errorf("%s: node %s: %s", path, nodename, *response.JSON403)
			case 404:
				return fmt.Errorf("%s: node %s: %s", path, nodename, *response.JSON404)
			case 500:
				return fmt.Errorf("%s: node %s: %s", path, nodename, *response.JSON500)
			default:
				return fmt.Errorf("%s: node %s: unexpected status code %d", path, nodename, response.StatusCode())
			}
		}
		url := response.HTTPResponse.Header.Get("Location")
		if err := t.runTtyShare(url, false); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectEnter) runTtyShare(url string, insecure bool) error {
	var args []string
	if insecure {
		args = append(args, "-k")
	}
	args = append(args, url)
	cmd := exec.Command("tty-share", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			if code == 2 && !insecure {
				fmt.Print("Invalid certificate, proceed anyway ? (y/n) : ")
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					r, err := regexp.Compile("^[oOyY]$")
					if err != nil {
						return err
					}
					if !r.MatchString(scanner.Text()) {
						return fmt.Errorf("aborted by user")
					}
					return t.runTtyShare(url, true)
				}
				return scanner.Err()
			}
		}
	}
	return nil
}
