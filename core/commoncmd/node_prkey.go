package commoncmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

// CheckPRKeyUniqueness reports the nodes of the cluster that announce
// the same scsi3 persistent reservation key.
//
// The key identifies a node to the storage, and two nodes holding the
// same one can each preempt the reservations of the other, which is
// what a scsireserv resource relies on to keep a peer away from a
// device it was not given. A duplicate is nearly always a node.conf
// copied to a peer without redacting the prkey.
//
// The explanation goes to stderr and the error makes the exit code non
// zero, so that the caller's stdout still holds the value it asked for.
// A key that could not be read from the daemon is not a duplicate: the
// caller is told the check did not run, and is not failed for it.
func CheckPRKeyUniqueness() error {
	byKey, err := prKeysByNode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot verify the prkey is unique in this cluster: %s\n", err)
		return nil
	}
	duplicates := make([]string, 0)
	for prKey, nodes := range byKey {
		if len(nodes) < 2 {
			continue
		}
		sort.Strings(nodes)
		duplicates = append(duplicates, fmt.Sprintf("%s: %s", prKey, strings.Join(nodes, ", ")))
	}
	if len(duplicates) == 0 {
		return nil
	}
	sort.Strings(duplicates)
	fmt.Fprintf(os.Stderr, "the following nodes share a scsi3 persistent reservation key:\n")
	for _, s := range duplicates {
		fmt.Fprintf(os.Stderr, "  %s\n", s)
	}
	fmt.Fprintf(os.Stderr, "a reservation taken by one is preemptable by the other. Set a different\n")
	fmt.Fprintf(os.Stderr, "node.prkey on all but one of them, usually because a node.conf was copied\n")
	fmt.Fprintf(os.Stderr, "to a peer without redacting it.\n")
	return fmt.Errorf("the prkey is not unique in this cluster")
}

// prKeysByNode returns the nodes of the cluster indexed by the prkey
// they announce.
func prKeysByNode() (map[string][]string, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	selector := "*"
	resp, err := c.GetNodesWithResponse(context.Background(), &api.GetNodesParams{Node: &selector})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("get nodes: unexpected status code %d", resp.StatusCode())
	}
	byKey := make(map[string][]string)
	for _, item := range resp.JSON200.Items {
		if item.Data.Config == nil || item.Data.Config.PRKey == "" {
			continue
		}
		byKey[item.Data.Config.PRKey] = append(byKey[item.Data.Config.PRKey], item.Meta.Node)
	}
	return byKey, nil
}
