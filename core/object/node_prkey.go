package object

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/util/key"
)

func validatePRKey(s string) error {
	var errs error
	minLength := 1
	maxLength := 16
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	length := len(s)
	if rest := length % 2; rest != 0 {
		s = "0" + s
		length++
	}
	if length < minLength {
		err := fmt.Errorf("prkey %s is too short: %d < %d chars", s, length, minLength)
		errs = errors.Join(errs, err)
	}
	if length > maxLength {
		err := fmt.Errorf("prkey %s is too long: %d > %d chars", s, length, maxLength)
		errs = errors.Join(errs, err)
	}
	if _, err := hex.DecodeString(s); err != nil {
		err = fmt.Errorf("prkey %s is not parseable as hexa, %s", s, err)
		errs = errors.Join(errs, err)
	}
	return errs
}

func newPRKey() string {
	s := uuid.New().String()
	return "0x" + s[:8] + s[9:13] + s[14:18]
}

// PRKey returns the SCSI3-PR key stored as node.prkey in the node config.
// It sets a new key if not found.
func (t Node) PRKey() (string, error) {
	prkeyKey := key.New("node", "prkey")
	prkey := t.config.GetString(prkeyKey)
	switch {
	case prkey == "":
		prkey = newPRKey()
		if err := validatePRKey(prkey); err != nil {
			return "", fmt.Errorf("new prkey: %w", err)
		}
		op := keyop.T{
			Key:   prkeyKey,
			Op:    keyop.Set,
			Value: prkey,
		}
		if err := t.config.Set(op); err != nil {
			return "", err
		}
		if err := t.config.Commit(); err != nil {
			return "", err
		}
		return prkey, nil
	default:
		if err := validatePRKey(prkey); err != nil {
			return "", err
		}
		return prkey, nil
	}
}
