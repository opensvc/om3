package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"opensvc.com/opensvc/util/file"
)

type (
	CompFiles struct {
		*Obj
	}
	CompFile struct {
		Path string `json:"path"`
		Mode int    `json:"mode"`
		UID  int    `json:"uid"`
		GID  int    `json:"gid"`
		Fmt  string `json:"fmt"`
		Ref  string `json:"ref"`
	}
)

var compFilesInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_FILE_",
	ExampleValue: CompFile{
		Path: "/some/path/to/file",
		Fmt:  "root@corp.com     %%HOSTNAME%%@corp.com",
		UID:  500,
		GID:  500,
	},
	Description: `* Verify and install file content.
* Verify and set file or directory ownership and permission
* Directory mode is triggered if the path ends with /

Special wildcards::

  %%ENV:VARNAME%%   Any environment variable value
  %%HOSTNAME%%      Hostname
  %%SHORT_HOSTNAME%%    Short hostname
`,
	FormDefinition: `Desc: |
  A file rule, fed to the 'files' compliance object to create a directory or a file and set its ownership and permissions. For files, a reference content can be specified or pointed through an URL.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Class: file
    Type: json
    Format: dict

Inputs:
  -
    Id: path
    Label: Path
    DisplayModeLabel: path
    LabelCss: action16
    Mandatory: Yes
    Help: File path to install the reference content to. A path ending with '/' is treated as a directory and as such, its content need not be specified.
    Type: string

  -
    Id: mode
    Label: Permissions
    DisplayModeLabel: perm
    LabelCss: action16
    Help: "In octal form. Example: 644"
    Type: integer

  -
    Id: uid
    Label: Owner
    DisplayModeLabel: uid
    LabelCss: guy16
    Help: Either a user ID or a user name
    Type: string or integer

  -
    Id: gid
    Label: Owner group
    DisplayModeLabel: gid
    LabelCss: guy16
    Help: Either a group ID or a group name
    Type: string or integer

  -
    Id: ref
    Label: Content URL pointer
    DisplayModeLabel: ref
    LabelCss: loc
    Help: "Examples:
        http://server/path/to/reference_file
        https://server/path/to/reference_file
        ftp://server/path/to/reference_file
        ftp://login:pass@server/path/to/reference_file"
    Type: string

  -
    Id: fmt
    Label: Content
    DisplayModeLabel: fmt
    LabelCss: hd16
    Css: pre
    Help: A reference content for the file. The text can embed substitution variables specified with %%ENV:VAR%%.
    Type: text
`,
}

func init() {
	m["file"] = NewCompFiles
}

func NewCompFiles() interface{} {
	return &CompFiles{
		Obj: NewObj(),
	}
}

func (t *CompFiles) Add(s string) error {
	var data CompFile
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	t.Obj.Add(data)
	return nil
}

func GetFile(url string) ([]byte, error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (t CompFile) Content() ([]byte, error) {
	if t.Ref == "" {
		b := []byte(t.Fmt)
		if !bytes.HasSuffix(b, []byte("\n")) {
			b = append(b, []byte("\n")...)
		}
		return b, nil
	}
	s, err := GetFile(t.Ref)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (t CompFile) FileMode() (os.FileMode, error) {
	s := fmt.Sprintf("0%d", t.Mode)
	i, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return os.FileMode(0), err
	}
	return os.FileMode(i), nil
}

func (t CompFiles) checkMode(rule CompFile) ExitCode {
	target, err := rule.FileMode()
	if err != nil {
		t.VerboseErrorf("file %s parse target mode: %s\n", rule.Path, err)
		return ExitNok
	}
	current, err := file.Mode(rule.Path)
	if err != nil {
		t.VerboseErrorf("file %s get current mode: %s\n", rule.Path, err)
		return ExitNok
	}
	if target == current {
		t.VerboseInfof("file %s mode is %s\n", rule.Path, target)
		return ExitOk
	}
	t.VerboseErrorf("file %s mode should be %s but is be %s\n", rule.Path, target, current)
	return ExitNok
}

func (t CompFiles) fixMode(rule CompFile) ExitCode {
	target, err := rule.FileMode()
	if err != nil {
		t.Errorf("file %s parse target mode: %s\n", rule.Path, err)
		return ExitNok
	}
	err = os.Chmod(rule.Path, target)
	if err != nil {
		t.Errorf("file %s mode set to %s failed: %s\n", rule.Path, target, err)
		return ExitNok
	} else {
		t.Infof("file %s mode set to %s\n", rule.Path, target)
	}
	return ExitOk
}

func (t CompFiles) checkOwnership(rule CompFile) ExitCode {
	uid, gid, err := file.Ownership(rule.Path)
	e := ExitOk
	if err != nil {
		t.VerboseErrorf("file %s get current ownership: %s\n", rule.Path, err)
		return ExitNok
	}
	if uid != rule.UID {
		t.VerboseErrorf("file %s uid should be %d but is %d\n", rule.Path, rule.UID, uid)
		e = ExitNok
	} else {
		t.VerboseErrorf("file %s uid is %d\n", rule.Path, rule.UID)
	}
	if gid != rule.GID {
		t.VerboseErrorf("file %s gid should be %d but is %d\n", rule.Path, rule.GID, gid)
		e = ExitNok
	} else {
		t.VerboseErrorf("file %s gid is %d\n", rule.Path, rule.GID)
	}
	return e
}

func (t CompFiles) fixOwnership(rule CompFile) ExitCode {
	err := os.Chown(rule.Path, rule.UID, rule.GID)
	if err == nil {
		t.Infof("file %s ownership set to %d:%d\n", rule.Path, rule.UID, rule.GID)
		return ExitOk
	} else {
		t.Errorf("file %s ownership set to %d:%d failed: %s\n", rule.Path, rule.UID, rule.GID, err)
		return ExitNok
	}
}

func (t CompFiles) checkContent(rule CompFile) ExitCode {
	target, err := rule.Content()
	if err != nil {
		t.VerboseErrorf("file %s get target content: %s\n", rule.Path, err)
		return ExitNok
	}
	current, err := file.ReadAll(rule.Path)
	if err != nil {
		t.VerboseErrorf("file %s get current content: %s\n", rule.Path, err)
		return ExitNok
	}
	fragments := myers.ComputeEdits(span.URIFromPath(rule.Path), string(current), string(target))
	if len(fragments) == 0 {
		t.VerboseInfof("file %s content on target\n", rule.Path)
		return ExitOk
	}
	diff := fmt.Sprint(gotextdiff.ToUnified(rule.Path, rule.Path+".tgt", string(current), fragments))
	t.VerboseErrorf("%s", diff)
	return ExitNok
}

func (t CompFiles) fixContent(rule CompFile) ExitCode {
	target, err := rule.Content()
	if err != nil {
		t.Errorf("file %s get target content: %s\n", rule.Path, err)
		return ExitNok
	}
	f, err := ioutil.TempFile(filepath.Dir(rule.Path), filepath.Base(rule.Path)+".comp-file-")
	if err != nil {
		t.Errorf("file %s open temp: %s\n", rule.Path, err)
		return ExitNok
	}
	_, err = f.Write(target)
	if err != nil {
		t.Errorf("file %s write temp: %s\n", rule.Path, err)
		f.Close()
		return ExitNok
	}
	tempName := f.Name()
	f.Close()
	err = os.Rename(tempName, rule.Path)
	if err != nil {
		t.Errorf("file %s install temp: %s\n", rule.Path, err)
		return ExitNok
	}
	t.Infof("file %s rewritten\n", rule.Path)
	return ExitOk
}

func (t CompFiles) FixRule(rule CompFile) ExitCode {
	if e := t.checkContent(rule); e == ExitNok {
		if e := t.fixContent(rule); e == ExitNok {
			return e
		}
	}
	if e := t.checkOwnership(rule); e == ExitNok {
		if e := t.fixOwnership(rule); e == ExitNok {
			return e
		}
	}
	if e := t.checkMode(rule); e == ExitNok {
		if e := t.fixMode(rule); e == ExitNok {
			return e
		}
	}
	return ExitOk
}

func (t CompFiles) CheckRule(rule CompFile) ExitCode {
	var e, o ExitCode
	o = t.checkContent(rule)
	e = e.Merge(o)
	o = t.checkOwnership(rule)
	e = e.Merge(o)
	o = t.checkMode(rule)
	e = e.Merge(o)
	return e
}

func (t CompFiles) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFile)
		o := t.CheckRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompFiles) Fix() ExitCode {
	t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompFile)
		if e := t.FixRule(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}

func (t CompFiles) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompFiles) Info() ObjInfo {
	return compFilesInfo
}
