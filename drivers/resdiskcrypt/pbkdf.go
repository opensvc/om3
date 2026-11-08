//go:build linux

package resdiskcrypt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/converters"
)

const (
	// pbkdfManaged is the key derivation of a volume formatted with the
	// passphrase om generates: 256 random characters, which a memory-hard
	// derivation makes no harder to guess, but which costs the node about
	// a gigabyte and seconds of cpu at each format and each open.
	pbkdfManaged = "pbkdf2"

	// iterTimeManaged is the time, in milliseconds, the derivation of a
	// passphrase om generates takes on the node formatting the volume.
	iterTimeManaged = 100
)

// pbkdfArgs returns the key derivation options of the luksFormat command.
//
// The passphrase om generates is derived with pbkdf2 for 100ms, unless a
// keyword asks for another derivation or another time. A passphrase the user
// supplies keeps the derivation cryptsetup chooses, unless a keyword asks for
// another. The options apply to the format only: an open reads them from the
// header.
func (t *T) pbkdfArgs() ([]string, error) {
	pbkdf := t.Pbkdf
	iterTime := t.IterTime
	if pbkdf == pbkdfManaged {
		var set []string
		if t.PbkdfMemory != "" {
			set = append(set, "pbkdf_memory")
		}
		if t.PbkdfParallel != 0 {
			set = append(set, "pbkdf_parallel")
		}
		if len(set) > 0 {
			return nil, fmt.Errorf("%s apply to the argon2 key derivations, not to pbkdf = %s", strings.Join(set, " and "), pbkdf)
		}
	}
	if t.ManagePassphrase && pbkdf == "" && t.PbkdfMemory == "" && t.PbkdfParallel == 0 {
		// The memory and the threads of an argon2 derivation set with no
		// pbkdf ask for the argon2 cryptsetup chooses, so the pbkdf2 of a
		// generated passphrase goes with none of them.
		pbkdf = pbkdfManaged
	}
	if t.ManagePassphrase && pbkdf == pbkdfManaged && iterTime == 0 {
		iterTime = iterTimeManaged
	}
	var args []string
	if pbkdf != "" {
		args = append(args, "--pbkdf", pbkdf)
	}
	if t.PbkdfMemory != "" {
		kib, err := parsePbkdfMemory(t.PbkdfMemory)
		if err != nil {
			return nil, err
		}
		args = append(args, "--pbkdf-memory", strconv.FormatInt(kib, 10))
	}
	if t.PbkdfParallel != 0 {
		args = append(args, "--pbkdf-parallel", strconv.Itoa(t.PbkdfParallel))
	}
	if iterTime != 0 {
		args = append(args, "--iter-time", strconv.Itoa(iterTime))
	}
	return args, nil
}

// parsePbkdfMemory returns the KiB the pbkdf_memory value s says: a number
// is a count of KiB, as cryptsetup reads it, and a size with a unit is
// converted to KiB.
func parsePbkdfMemory(s string) (int64, error) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("pbkdf_memory %s is not a positive count of KiB", s)
		}
		return n, nil
	}
	v, err := converters.Size.Convert(s)
	if err != nil {
		return 0, fmt.Errorf("pbkdf_memory %s is neither a count of KiB nor a size: %w", s, err)
	}
	n := *v.(*int64) / 1024
	if n <= 0 {
		return 0, fmt.Errorf("pbkdf_memory %s is less than 1 KiB", s)
	}
	return n, nil
}

func validatePbkdfMemory(s string) error {
	_, err := parsePbkdfMemory(s)
	return err
}

// validatePositive says whether s is a count above zero, as the threads of
// a derivation and its time in milliseconds are.
func validatePositive(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("%s is not a positive integer", s)
	}
	return nil
}
