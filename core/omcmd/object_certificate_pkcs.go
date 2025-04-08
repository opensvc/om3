package omcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"golang.org/x/term"
)

type (
	CmdObjectCertificatePKCS struct {
		OptsGlobal
	}
)

func ReadPasswordFromStdinOrPrompt(prompt string) ([]byte, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		if b, err := os.ReadFile("/dev/stdin"); err != nil {
			return nil, err
		} else {
			return b, nil
		}
	}

	fmt.Fprintf(os.Stderr, prompt)
	if b, err := term.ReadPassword(int(os.Stdin.Fd())); err != nil {
		fmt.Fprintln(os.Stderr, "")
		return nil, err
	} else {
		fmt.Fprintln(os.Stderr, "")
		return b, nil
	}
}

func (t *CmdObjectCertificatePKCS) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			o, err := object.New(p)
			if err != nil {
				return nil, err
			}
			store, ok := o.(object.SecureKeystore)
			if !ok {
				return nil, fmt.Errorf("%s is not a secure keystore", o)
			}

			b, err := ReadPasswordFromStdinOrPrompt("Password: ")
			if err != nil {
				return nil, err
			}
			return store.PKCS(string(b))
		}),
	).Do()
}
