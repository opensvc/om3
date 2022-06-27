package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/editor"
	"opensvc.com/opensvc/util/file"
)

type (
	// CmdObjectEdit is the cobra flag set of the print config command.
	CmdObjectEdit struct {
		Command *cobra.Command
		Global  object.OptsGlobal
		EditKey object.OptsEditKey
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectEdit) Init(kind string, parent *cobra.Command, selector *string) {
	t.Command = t.cmd(kind, selector)
	parent.AddCommand(t.Command)
	flag.Install(t.Command, t)
}

func (t *CmdObjectEdit) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "edit",
		Short:   "edit selected object and instance configuration",
		Aliases: []string{"edi", "ed", "e"},
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectEdit) do(selector string, c *client.T) error {
	sel := object.NewSelection(selector)
	wc := clientcontext.IsSet()
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		obj, err := object.NewFromPath(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %s\n", p, err)
			continue
		}
		var (
			ks object.Keystorer
			ok bool
		)
		if ks, ok = obj.(object.Keystorer); !ok {
			fmt.Fprintf(os.Stderr, "skip %s: not a keystore\n", p)
			continue
		}
		if !wc && object.Exists(p) {
			if err := t.doLocal(ks, c); err != nil {
				return err
			}
			continue
		}
		if err := t.doRemote(p, c); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectEdit) doLocal(obj object.Keystorer, c *client.T) error {
	return obj.EditKey(t.EditKey)
}

func fetchKey(p path.T, key string, c *client.T) (s []byte, err error) {
	var (
		b []byte
	)
	handle := c.NewGetKey()
	handle.Path = p.String()
	handle.Key = key
	b, err = handle.Do()
	if err != nil {
		return []byte{}, err
	}
	resp := struct {
		data []byte
	}{}
	if err = json.Unmarshal(b, &resp); err != nil {
		return []byte{}, err
	}
	return resp.data, nil
}

func pushKey(p path.T, key string, fName string, c *client.T) (err error) {
	var b []byte
	if b, err = os.ReadFile(fName); err != nil {
		return err
	}
	req := c.NewPostKey()
	req.Path = p.String()
	req.Key = key
	req.Data = b
	_, err = req.Do()
	if err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEdit) doRemote(p path.T, c *client.T) error {
	var (
		err    error
		refSum []byte
		buff   []byte
		f      *os.File
	)
	if buff, err = fetchKey(p, t.EditKey.Key, c); err != nil {
		return err
	}
	if f, err = ioutil.TempFile("", ".opensvc.edit.key.*"); err != nil {
		return err
	}
	fName := f.Name()
	defer os.Remove(fName)
	if _, err = f.Write([]byte(buff)); err != nil {
		return err
	}
	if refSum, err = file.MD5(fName); err != nil {
		return err
	}
	if err = editor.Edit(fName); err != nil {
		return err
	}
	if file.HaveSameMD5(refSum, fName) {
		fmt.Println("unchanged")
		return nil
	}
	if err = pushKey(p, t.EditKey.Key, fName, c); err != nil {
		return err
	}
	return nil
}

func (t *CmdObjectEdit) run(selector *string, kind string) {
	var (
		c   *client.T
		err error
	)
	mergedSelector := mergeSelector(*selector, t.Global.ObjectSelector, kind, "")
	if c, err = client.New(client.WithURL(t.Global.Server)); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
	if err = t.do(mergedSelector, c); err != nil {
		log.Error().Err(err).Msg("")
		os.Exit(1)
	}
}
