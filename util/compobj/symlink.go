package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type (
	CompSymlinks struct {
		*Obj
	}
	CompSymlink struct {
		Symlink string `json:"symlink"`
		Target  string `json:"target"`
	}
)

var compSymlinkInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_SYMLINK_",
	ExampleValue: CompSymlink{
		Symlink: "/tmp/foo",
		Target:  "/tmp/bar",
	},
	Description: `* Verify symlink's existence.
* The collector provides the format with wildcards.
* The module replace the wildcards with contextual values.
* In the 'fix' the symlink is created (and intermediate dirs if required).
* There is no check or fix for target's existence.
* There is no check or fix for mode or ownership of either symlink or target.
`,
	FormDefinition: `Desc: |
	A symfile rule, fed to the 'symlink' compliance object to create a Unix symbolic link.
  Css: comp48
  
  Outputs:
	-
	  Dest: compliance variable
	  Class: symlink
	  Type: json
	  Format: dict
  
  Inputs:
	-
	  Id: symlink
	  Label: Symlink path
	  DisplayModeLabel: symlink
	  LabelCss: hd16
	  Mandatory: Yes
	  Help: The full path of the symbolic link to check or create.
	  Type: string
  
	-
	  Id: target
	  Label: Target path
	  DisplayModeLabel: target
	  LabelCss: hd16
	  Mandatory: Yes
	  Help: The full path of the target file pointed by the symlink.
	  Type: string
`,
}

func init() {
	m["symlink"] = NewCompSymlinks
}

func NewCompSymlinks() interface{} {
	return &CompSymlinks{
		Obj: NewObj(),
	}
}

func (t *CompSymlinks) Add(s string) error {
	var data CompSymlink
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	if data.Symlink == "" {
		err := fmt.Errorf("symlink should be in the dict: %s", s)
		t.Errorf("%s\n", err)
		return err
	}
	if data.Target == "" {
		err := fmt.Errorf("target should be in the dict: %s", s)
		t.Errorf("%s\n", err)
		return err
	}
	t.Obj.Add(data)
	return nil
}

func (t CompSymlinks) CheckSymlink(rule CompSymlink) ExitCode {
	tgt, err := os.Readlink(rule.Symlink)
	if err != nil {
		t.VerboseErrorf("symlink %s does not exist\n", rule.Symlink)
		return ExitNok
	}
	if tgt != rule.Target {
		t.VerboseErrorf("symlink %s does not point to %s\n", rule.Symlink, rule.Target)
		return ExitNok
	}
	if t.verbose {
		t.VerboseInfof("symlink %s -> %s is ok\n", rule.Symlink, rule.Target)
	}
	return ExitOk
}

func (t CompSymlinks) fixLink(rule CompSymlink) ExitCode {
	d := filepath.Dir(rule.Symlink)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		if err := os.MkdirAll(d, 0511); err != nil {
			t.Errorf("symlink: can not create dir %s to host the symlink %s\n", d, rule.Symlink)
			return ExitNok
		}
	}
	err := os.Symlink(rule.Target, rule.Symlink)
	if err != nil {
		t.Errorf("Can't create symlink %s\n", rule.Symlink)
		return ExitNok
	}
	t.Infof("create symlink %s\n", rule.Symlink)
	return ExitOk
}

func (t CompSymlinks) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		e = e.Merge(t.CheckSymlink(rule))
	}
	return e
}

func (t CompSymlinks) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		if t.CheckSymlink(rule) == ExitNok {
			e = e.Merge(t.fixLink(rule))
		}
	}
	return e
}

func (t CompSymlinks) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompSymlinks) Info() ObjInfo {
	return compSymlinkInfo
}
