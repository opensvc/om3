package claim

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

const (
	// TypeCPU is a claim on the cpu the objects of a namespace are capped
	// to, counted in thousandths of a cpu.
	TypeCPU = "cpu"

	// TypeMemory is a claim on the memory the objects of a namespace are
	// capped to, counted in bytes.
	TypeMemory = "memory"

	// Unbounded is the claim of an object running a process capped by
	// nothing, which takes what it finds.
	Unbounded = int64(-1)
)

// ComputeTypes are the claims counted from the caps of the objects.
var ComputeTypes = []string{TypeCPU, TypeMemory}

// ParseCPU reads a cpu cap in the pg_cpu_quota notation, a percentage of the
// cpus named after the @, one when none is, and answers it in thousandths of
// a cpu:
//
//	50%     => 500, half of one
//	100%@4  => 4000, four of them
//	10%@2   => 200, a tenth of two
//
// @all names the cpus of the node, which are allCPUs of them. A claim is a
// quantity of the cluster, where no node is the one to count them on, so a
// claim limit is read with allCPUs at zero, which refuses @all.
func ParseCPU(s string, allCPUs int) (int64, error) {
	pctString, cpusString, found := strings.Cut(s, "@")
	pct, err := strconv.ParseFloat(strings.TrimSuffix(pctString, "%"), 64)
	if err != nil || pct < 0 {
		return 0, fmt.Errorf("invalid cpu cap %s (accepted expressions: 50%%, 100%%@4)", s)
	}
	cpus := 1
	switch {
	case !found:
	case cpusString == "all" && allCPUs > 0:
		cpus = allCPUs
	case cpusString == "all":
		return 0, fmt.Errorf("invalid cpu cap %s: @all names the cpus of a node, and a claim is on those of the cluster", s)
	default:
		cpus, err = strconv.Atoi(cpusString)
		if err != nil || cpus < 1 {
			return 0, fmt.Errorf("invalid cpu cap %s (accepted expressions: 50%%, 100%%@4)", s)
		}
	}
	return int64(pct * float64(cpus) * 10), nil
}

// ParseMemory reads a memory cap in bytes.
func ParseMemory(s string) (int64, error) {
	return sizeconv.FromSize(s)
}

// Parse reads a cap of a compute claim type.
func Parse(claimType, s string, allCPUs int) (int64, error) {
	switch claimType {
	case TypeCPU:
		return ParseCPU(s, allCPUs)
	case TypeMemory:
		return ParseMemory(s)
	default:
		return 0, fmt.Errorf("%s is not a compute claim", claimType)
	}
}

// Format says a quantity of a compute claim type in the words of its unit.
func Format(claimType string, v int64) string {
	switch {
	case v == Unbounded:
		return "unbounded"
	case claimType == TypeCPU:
		return strconv.FormatFloat(float64(v)/1000, 'f', -1, 64) + " cpu"
	default:
		return sizeconv.BSizeCompact(float64(v))
	}
}

// ComputeLimit is the most a namespace may claim of a compute type, and
// whether it is capped on it at all.
func ComputeLimit(namespace, claimType string) (int64, bool, error) {
	s, capped, err := Limit(namespace, claimType, "")
	if err != nil || !capped {
		return 0, false, err
	}
	v, err := Parse(claimType, s, 0)
	if err != nil {
		return 0, false, fmt.Errorf("%s claim of the %s namespace: %w", claimType, namespace, err)
	}
	return v, true, nil
}

// ComputeFits says whether a namespace may have an object claim what it
// asks of each compute type, and why not when it may not.
//
// The claims are what the object is to claim, not the increase: what it
// claims today is already counted in what the namespace holds.
//
// A namespace claiming none of the types asked about is not capped on them,
// and is answered from the local configuration alone. A capped one is
// brokered by the node speaking for the cluster, as a pool claim is.
//
// Failing to reach the daemon leaves the claim unchecked rather than refused:
// where there is no daemon to ask there is nothing brokering.
func ComputeFits(ctx context.Context, namespace, path string, claims map[string]int64) (bool, string, error) {
	capped := false
	for claimType := range claims {
		if _, ok, err := ComputeLimit(namespace, claimType); err != nil {
			return false, "", err
		} else if ok {
			capped = true
		}
	}
	if !capped {
		return true, "", nil
	}
	c, err := client.New()
	if err != nil {
		return true, "", nil
	}
	resp, err := c.PostComputeClaimWithResponse(ctx, api.PostComputeClaim{
		Namespace: namespace,
		Path:      path,
		Claims:    claims,
	})
	if err != nil || resp.JSON200 == nil {
		return true, "", nil
	}
	if resp.JSON200.Granted {
		return true, "", nil
	}
	var why string
	if resp.JSON200.Reason != nil {
		why = *resp.JSON200.Reason
	}
	return false, why, nil
}
