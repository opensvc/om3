//go:build linux

package network

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/google/nftables"
)

// The accepts of the forward chain of om's table are not enough on a node
// running docker. nftables runs every base chain registered on a hook, and a
// packet one of them drops is dropped, whatever the others say: the forward
// chain docker has iptables add to the filter table drops what docker did not
// accept, its policy set to drop, so the traffic of the om networks forwarded
// between the nodes never leaves the node.
//
// Docker leaves the DOCKER-USER chain to the administrator, which its forward
// chain runs first. om fills a chain of its own with the accepts, and has
// DOCKER-USER jump to it, through iptables, which owns the filter table: a
// chain iptables adds keeps the table readable to the other firewall drivers
// going through it, as docker does.
const (
	fwDockerUserChain = "DOCKER-USER"
	fwDockerChain     = "OSVC-FORWARD"
)

type (
	// iptablesFamily is the iptables command of an address family.
	iptablesFamily struct {
		Family  nftables.TableFamily
		Command string
	}

	// iptablesRunner runs an iptables command, with stdin as its input when
	// it is not empty, and returns its output.
	iptablesRunner func(name, stdin string, args ...string) ([]byte, error)
)

var (
	iptablesFamilies = []iptablesFamily{
		{Family: nftables.TableFamilyIPv4, Command: "iptables"},
		{Family: nftables.TableFamilyIPv6, Command: "ip6tables"},
	}

	// runIPTables runs the iptables commands, replaced by the tests.
	runIPTables iptablesRunner = func(name, stdin string, args ...string) ([]byte, error) {
		cmd := exec.Command(name, args...)
		if stdin != "" {
			cmd.Stdin = strings.NewReader(stdin)
		}
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b
		err := cmd.Run()
		return b.Bytes(), err
	}

	// hasIPTables says the iptables command of a family is installed,
	// replaced by the tests.
	hasIPTables = func(name string) bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
)

// fwDockerDevs returns the devices of the networks of a family, whose
// forwarded traffic the chain accepts.
func fwDockerDevs(family nftables.TableFamily, networks []fwNetwork) []string {
	l := make([]string, 0)
	for _, nw := range networks {
		ip, _, err := net.ParseCIDR(nw.CIDR)
		if err != nil || ipFamily(ip) != family || !isDevNameValid(nw.Dev) {
			continue
		}
		l = append(l, nw.Dev)
	}
	return l
}

// fwDockerRestore renders the iptables-restore document filling the chain.
//
// Declaring the chain flushes it, and the document is one transaction, so
// the accepts are never half there, as the ones of om's table are not.
func fwDockerRestore(devs []string) string {
	var sb strings.Builder
	sb.WriteString("*filter\n")
	fmt.Fprintf(&sb, ":%s - [0:0]\n", fwDockerChain)
	for _, dev := range devs {
		fmt.Fprintf(&sb, "-A %s -i %s -j ACCEPT\n", fwDockerChain, dev)
		fmt.Fprintf(&sb, "-A %s -o %s -j ACCEPT\n", fwDockerChain, dev)
	}
	sb.WriteString("COMMIT\n")
	return sb.String()
}

// setupDockerFW has DOCKER-USER accept the traffic of the om networks, on the
// nodes running docker, and removes what om added there before from the ones
// no longer running it or having no network of the family.
func setupDockerFW(n logger, networks []fwNetwork) error {
	errs := make([]error, 0)
	for _, f := range iptablesFamilies {
		if !hasIPTables(f.Command) {
			continue
		}
		if err := setupDockerFWFamily(n, f.Command, fwDockerDevs(f.Family, networks)); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Command, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("docker forward chain: %s", errs)
	}
	return nil
}

func setupDockerFWFamily(n logger, cmd string, devs []string) error {
	hasChain := func(chain string) bool {
		_, err := runIPTables(cmd, "", "-w", "-S", chain)
		return err == nil
	}
	isJumped := func() bool {
		_, err := runIPTables(cmd, "", "-w", "-C", fwDockerUserChain, "-j", fwDockerChain)
		return err == nil
	}
	run := func(args ...string) error {
		n.Log().Infof("%s %s", cmd, strings.Join(args, " "))
		if b, err := runIPTables(cmd, "", append([]string{"-w"}, args...)...); err != nil {
			return fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
		}
		return nil
	}
	hasDocker := hasChain(fwDockerUserChain)
	if !hasDocker || len(devs) == 0 {
		if hasDocker {
			for isJumped() {
				if err := run("-D", fwDockerUserChain, "-j", fwDockerChain); err != nil {
					return err
				}
			}
		}
		if hasChain(fwDockerChain) {
			if err := run("-F", fwDockerChain); err != nil {
				return err
			}
			if err := run("-X", fwDockerChain); err != nil {
				return err
			}
		}
		return nil
	}
	doc := fwDockerRestore(devs)
	n.Log().Attr("ruleset", doc).Infof("apply the %s chain of the om networks", fwDockerChain)
	if b, err := runIPTables(cmd+"-restore", doc, "-w", "--noflush"); err != nil {
		return fmt.Errorf("%s-restore: %w: %s", cmd, err, strings.TrimSpace(string(b)))
	}
	if isJumped() {
		return nil
	}
	return run("-I", fwDockerUserChain, "1", "-j", fwDockerChain)
}
