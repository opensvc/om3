package omcmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/sshnode"
)

type (
	CmdNodeSSHTrust struct {
		OptsGlobal
		authorizedKeys sshnode.AuthorizedKeysMap
		client         *client.T
	}
)

func (t *CmdNodeSSHTrust) doNode(node string) error {
	resp, err := t.client.GetNodeSSHKeys(context.Background(), node)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return fmt.Errorf("get ssh keys from %s: %s", node, resp.Status)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Bytes()
		if v, _ := t.authorizedKeys.Has(line); v {
			fmt.Println("node", node, "is already trusted by key", string(line))
			continue
		}
		if err := sshnode.AppendAuthorizedKeys(line); err != nil {
			return err
		} else {
			fmt.Println("node", node, "add trust by key", string(line))
		}
	}

	// Check for errors
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (t *CmdNodeSSHTrust) Run() error {
	c, err := client.New(client.WithURL(t.Server), client.WithTimeout(0))
	if err != nil {
		return err
	}
	t.client = c
	nodes, err := nodeselector.New("*", nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found")
	}
	if m, err := sshnode.GetAuthorizedKeysMap(); err != nil {
		return err
	} else {
		t.authorizedKeys = m
	}

	var errs error
	for _, node := range nodes {
		if node == hostname.Hostname() {
			continue
		}
		if err := t.doNode(node); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
