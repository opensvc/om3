package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/opensvc/om3/v3/util/file"
)

type (
	CompFiles struct {
		*Obj
	}
	CompFile struct {
		Path string      `json:"path"`
		Mode *int        `json:"mode"`
		UID  interface{} `json:"uid"`
		GID  interface{} `json:"gid"`
		Fmt  *string     `json:"fmt"`
		Ref  string      `json:"ref"`
	}
)

var (
	collectorSafeGetMetaFunc = collectorSafeGetMeta

	stringFmt = "root@corp.com     %%HOSTNAME%%@corp.com"

	compFilesInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_FILE_",
		ExampleValue: CompFile{
			Path: "/some/path/to/file",
			Fmt:  &stringFmt,
			UID:  500,
			GID:  500,
		},
		Description: `* Verify and install file content.
* Verify and set file or directory ownership and permission
* Directory mode is triggered if the path ends with /
* Only for the fmt field : if the newline character is not present at the end of the text, one is automatically added

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
)

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
	if data.Path == "" {
		err := fmt.Errorf("path should be in the dict: %s", s)
		t.Errorf("%s\n", err)
		return err
	}
	if data.Ref == "" && data.Fmt == nil {
		err := fmt.Errorf("ref or fmt should be in the dict: %s", s)
		t.Errorf("%s\n", err)
		return err
	}
	t.Obj.Add(data)
	return nil
}

func (t CompFile) Content() ([]byte, error) {
	if t.Ref == "" {
		b := []byte(*t.Fmt)
		if !bytes.HasSuffix(b, []byte("\n")) {
			b = append(b, []byte("\n")...)
		}
		return subst(b), nil
	}
	b, err := getFile(t.Ref)
	if err != nil {
		return nil, err
	}
	return subst(b), nil
}

func (t CompFile) ParseUID() (int, error) {
	switch v := t.UID.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		cmd := exec.Command("bash", "-c", "getent passwd "+t.UID.(string)+" | cut -d: -f3")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return -1, fmt.Errorf("%w: %s", err, output)
		}
		if len(output) == 0 {
			return -1, fmt.Errorf("the user %s does not exist", t.UID.(string))
		}
		uid, err := strconv.ParseInt(string(output)[:len(output)-1], 10, 64)
		if err != nil {
			return -1, err
		}
		return int(uid), nil
	default:
		return -1, nil
	}
}

func (t CompFile) ParseGID() (int, error) {
	switch v := t.GID.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		cmd := exec.Command("bash", "-c", "getent group "+t.GID.(string)+" | cut -d: -f3")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return -1, fmt.Errorf("%w: %s", err, output)
		}
		if len(output) == 0 {
			return -1, fmt.Errorf("the group %s does not exist", t.GID.(string))
		}
		gid, err := strconv.ParseInt(string(output)[:len(output)-1], 10, 64)
		if err != nil {
			return -1, err
		}
		return int(gid), nil
	default:
		return -1, nil
	}
}

func (t CompFile) FileMode() (os.FileMode, error) {
	s := fmt.Sprintf("0%d", *t.Mode)
	i, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return os.FileMode(0), err
	}
	return os.FileMode(i), nil
}

func (t CompFiles) checkPathExistance(rule CompFile) ExitCode {
	_, err := os.Lstat(rule.Path)
	if err != nil {
		if os.IsNotExist(err) {
			t.VerboseErrorf("the file %s does not exist\n", rule.Path)
			return ExitNok
		}
		t.VerboseErrorf("can't check if the file %s exist: %s\n", rule.Path, err)
		return ExitNok
	}
	return ExitOk
}

func (t CompFiles) checkMode(rule CompFile) ExitCode {
	if rule.Mode == nil {
		return ExitNotApplicable
	}
	target, err := rule.FileMode()
	if strings.HasSuffix(rule.Path, "/") {
		target = target | os.ModeDir
	}
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
	if strings.HasSuffix(rule.Path, "/") {
		target = target | os.ModeDir
	}
	if err != nil {
		t.Errorf("file %s parse target mode: %s\n", rule.Path, err)
		return ExitNok
	}
	err = os.Chmod(rule.Path, target)
	if err != nil {
		t.Errorf("file %s mode set to %s failed: %s\n", rule.Path, target, err)
		return ExitNok
	}
	t.Infof("file %s mode set to %s\n", rule.Path, target)
	return ExitOk
}

func (t CompFiles) checkOwnership(rule CompFile) ExitCode {
	e := ExitOk
	targetUID, err := rule.ParseUID()
	if err != nil {
		t.VerboseErrorf("%s\n", err)
		e = e.Merge(ExitNok)
	}
	targetGID, err := rule.ParseGID()
	if err != nil {
		t.VerboseErrorf("%s\n", err)
		e = e.Merge(ExitNok)
	}
	if e == ExitNok {
		return ExitNok
	}
	uid, gid, err := file.Ownership(rule.Path)
	if err != nil {
		t.VerboseErrorf("file %s get current ownership: %s\n", rule.Path, err)
		return ExitNok
	}

	if targetUID < 0 {
		// ignore
	} else if uid != targetUID {
		t.VerboseErrorf("file %s uid should be %d but is %d\n", rule.Path, targetUID, uid)
		e = ExitNok
	} else {
		t.VerboseErrorf("file %s uid is %d\n", rule.Path, targetUID)
	}

	if targetGID < 0 {
		// ignore
	} else if gid != targetGID {
		t.VerboseErrorf("file %s gid should be %d but is %d\n", rule.Path, targetGID, gid)
		e = ExitNok
	} else {
		t.VerboseErrorf("file %s gid is %d\n", rule.Path, targetGID)
	}
	return e
}

func (t CompFiles) fixOwnership(rule CompFile) ExitCode {
	e := ExitOk
	targetUID, err := rule.ParseUID()
	if err != nil {
		t.Errorf("%s\n", err)
		e = e.Merge(ExitNok)
	}
	targetGID, err := rule.ParseGID()
	if err != nil {
		t.Errorf("%s\n", err)
		e = e.Merge(ExitNok)
	}
	if e == ExitNok {
		return ExitNok
	}
	err = os.Chown(rule.Path, targetUID, targetGID)
	if err == nil {
		t.Infof("file %s ownership set to %d:%d\n", rule.Path, targetUID, targetGID)
		return ExitOk
	} else {
		t.Errorf("file %s ownership set to %d:%d failed: %s\n", rule.Path, targetUID, targetGID, err)
		return ExitNok
	}
}

func (t CompFile) isSafeRef() bool {
	if t.Ref == "" {
		return false
	}
	if strings.HasPrefix(t.Ref, "safe") {
		return true
	}
	return false
}

func (t CompFiles) checkSafeRef(rule CompFile) ExitCode {
	meta, err := collectorSafeGetMetaFunc(rule.Ref)
	currentMD5, err := file.MD5(rule.Path)
	if err != nil {
		t.VerboseErrorf("file %s md5sum: %s\n", rule.Path, err)
		return ExitNok
	}
	if hex.EncodeToString(currentMD5) != meta.MD5 {
		t.VerboseErrorf("file %s content md5 differs from its reference\n", rule.Path)
		return ExitNok
	}
	t.VerboseInfof("file %s md5 verified\n", rule.Path)
	return ExitOk
}

func (t CompFiles) checkContent(rule CompFile) ExitCode {
	if rule.Ref == "" && rule.Fmt == nil {
		return ExitNotApplicable
	}
	if rule.isSafeRef() {
		return t.checkSafeRef(rule)
	}
	target, err := rule.Content()
	if err != nil {
		t.VerboseErrorf("file %s get target content: %s\n", rule.Path, err)
		return ExitNok
	}
	current, err := os.ReadFile(rule.Path)
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
	t.VerboseErrorf("%s\n", diff)
	return ExitNok
}

func (t CompFiles) fixContent(rule CompFile) ExitCode {
	target, err := rule.Content()
	if err != nil {
		t.Errorf("file %s get target content: %s\n", rule.Path, err)
		return ExitNok
	}
	f, err := os.CreateTemp(filepath.Dir(rule.Path), filepath.Base(rule.Path)+".comp-file-")
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
	if _, err := backup(rule.Path); err != nil {
		t.Errorf("file %s backup: %s\n", rule.Path, err)
		return ExitNok
	}
	err = os.Rename(tempName, rule.Path)
	if err != nil {
		t.Errorf("file %s install temp: %s\n", rule.Path, err)
		return ExitNok
	}
	t.Infof("file %s rewritten\n", rule.Path)
	return ExitOk
}

func (t CompFiles) fixPathExistence(rule CompFile) ExitCode {
	if strings.HasSuffix(rule.Path, "/") {
		err := os.Mkdir(rule.Path, 0666)
		if err != nil {
			t.Errorf("can't create the dir: %s\n", rule.Path)
			return ExitNok
		}
		return ExitOk
	}

	err := os.MkdirAll(filepath.Dir(rule.Path), 0666)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	f, err := os.Create(rule.Path)
	if err != nil {
		t.Errorf("can't create the file: %s\n", rule.Path)
		return ExitNok
	}
	if err = f.Close(); err != nil {
		t.Errorf("can't close the file: %s\n", rule.Path)
		return ExitNok
	}
	t.Infof("create the file %s\n", rule.Path)
	return ExitOk
}

func (t CompFiles) FixRule(rule CompFile) ExitCode {
	if e := t.checkPathExistance(rule); e == ExitNok {
		if e := t.fixPathExistence(rule); e == ExitNok {
			return ExitNok
		}
	}
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
	if o = t.checkPathExistance(rule); o == ExitNok {
		return ExitNok
	}
	e = e.Merge(o)
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
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFile)
		e = e.Merge(t.FixRule(rule))
	}
	return e
}

func (t CompFiles) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompFiles) Info() ObjInfo {
	return compFilesInfo
}
