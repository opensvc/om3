package object

import (
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/xerrors"
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
		length += 1
	}
	if length < minLength {
		err := errors.Errorf("prkey %s is too short: %d < %d chars", s, length, minLength)
		errs = xerrors.Append(errs, err)
	}
	if length > maxLength {
		err := errors.Errorf("prkey %s is too long: %d > %d chars", s, length, maxLength)
		errs = xerrors.Append(errs, err)
	}
	if _, err := hex.DecodeString(s); err != nil {
		err = errors.Errorf("prkey %s is not parseable as hexa, %s", s, err)
		errs = xerrors.Append(errs, err)
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
			return "", errors.Wrap(err, "new prkey")
		}
		op := keyop.T{
			Key:   prkeyKey,
			Op:    keyop.Set,
			Value: prkey,
		}
		if err := t.config.SetKeys(op); err != nil {
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
