package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type (
	CompFileincs struct {
		*Obj
	}
	CompFileinc struct {
		Path      string `json:"path"`
		Check     string `json:"check"`
		Replace   string `json:"replace"`
		Fmt       string `json:"fmt"`
		StrictFmt bool   `json:"strict_Fmt"`
		Ref       string `json:"ref"`
	}
)

const MAXSZ = 8 * 1024 * 1024

var (
	fileContentCache = map[string][]byte{}
	compFileincInfo  = ObjInfo{
		DefaultPrefix: "OSVC_COMP_FILEINC_",
		ExampleValue: CompFileinc{
			Path:  "/tmp/foo",
			Check: ".*some pattern.*",
			Fmt:   "full added content with %%HOSTNAME%%@corp.com: some pattern into the file",
		},
		Description: `* Verify or Change file content.
* The collector provides the format with wildcards.
* The module replace the wildcards with contextual values.
* The fmt must match the check pattern ['check' statement]
* The fmt is used to substitute any string matching the replace pattern ['replace' statement]

Wildcards:
%%ENV:VARNAME%%		Any environment variable value
%%HOSTNAME%%		Hostname
%%SHORT_HOSTNAME%%	Short hostname
`,
		FormDefinition: `
Desc: |
  A fileinc rule, fed to the 'fileinc' compliance object to verify a line matching the 'check' regular expression is present in the specified file. Alternatively, the 'replace' statement can be used to substitute any matching expression by string provided by 'fmt' or 'ref' content.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Class: fileinc
    Type: json
    Format: dict

Inputs:
  -
    Id: path
    Label: Path
    DisplayModeLabel: path
    LabelCss: hd16
    Mandatory: Yes
    Help: File path to search the matching line into.
    Type: string
  -
    Id: check
    Label: Check regexp
    DisplayModeLabel: check
    LabelCss: action16
    Mandatory: No
    Help: A regular expression. Matching the regular expression is sufficient to grant compliance. It is required to use either 'check' or 'replace'.
    Type: string
  -
    Id: replace
    Label: Replace regexp
    DisplayModeLabel: replace
    LabelCss: action16
    Mandatory: No
    Help: A regular expression. Any pattern matched by the regular expression will be replaced. It is required to use either 'check' or 'replace'.
    Type: string
  -
    Id: fmt
    Label: Format
    DisplayModeLabel: fmt
    LabelCss: action16
    Help: The line installed if the check pattern is not found in the file.
    Type: string
  -
    Id: strict_fmt
    Label: Strict Format
    DisplayModeLabel: strict fmt
    LabelCss: action16
    Help: Consider a line matching the check regexp invalid if the line is not strictly the same as fmt.
    Type: boolean
    Default: True
  -
    Id: ref
    Label: URL to format
    DisplayModeLabel: ref
    LabelCss: loc
    Help: An URL pointing to a file containing the line installed if the check pattern is not found in the file.
    Type: string
`,
	}
)

func init() {
	m["fileinc"] = NewCompFileincs
}

func NewCompFileincs() interface{} {
	return &CompFileincs{
		Obj: NewObj(),
	}
}

func (t *CompFileincs) Add(s string) error {
	var data CompFileinc
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	if strings.TrimSpace(data.Path) == "" {
		return fmt.Errorf("path should be in the dict: %s", s)
	}
	if data.Check == "" && data.Replace == "" {
		return fmt.Errorf("check or replace should be in the dict: %s", s)
	}
	if data.Check != "" && data.Replace != "" {
		return fmt.Errorf("check and replace can't be both in the dict: %s", s)
	}
	if data.Fmt != "" && data.Ref != "" {
		return fmt.Errorf("fmt and ref can't be both in the same dict: %s", s)
	}
	data.Path = strings.TrimSpace(data.Path)
	t.Obj.Add(data)
	return nil
}

func (t CompFileincs) getFileContentCache(path string) error {
	if _, ok := fileContentCache[path]; ok {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fileContentCache[path] = content
	return nil
}

func (t CompFileincs) loadFileContentCache(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fileContentCache[path] = content
	return nil
}

func (t CompFileincs) checkRule(rule CompFileinc) ExitCode {
	info, err := os.Stat(rule.Path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("the file %s does not exist\n", rule.Path)
			return ExitNok
		}
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if info.Size() > MAXSZ {
		t.Errorf("file %s is too large [%.2f Mb] to fit\n", rule.Path, float64(info.Size()/(1024*1024)))
		return ExitNok
	}
	if err := t.getFileContentCache(rule.Path); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	switch rule.Check {
	case "":
		return t.checkReplace(rule)
	default:
		return t.checkCheck(rule)
	}
}

func (t CompFileincs) getLineTochange(rule CompFileinc) ([]byte, error) {
	switch rule.Fmt {
	case "":
		byteContent, err := getFile(rule.Ref)
		return byteContent, err
	default:
		return []byte(rule.Fmt), nil
	}
}

func (t CompFileincs) checkCheck(rule CompFileinc) ExitCode {
	lineToAdd, err := t.getLineTochange(rule)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	reg, err := regexp.Compile(rule.Check)
	if err != nil {
		t.Errorf("the regex in rule does not compile: %s\n", err)
		return ExitNok
	}
	if !reg.Match(lineToAdd) {
		t.VerboseErrorf("rule error: '%s' does not match target content\n", rule.Check)
		return ExitNok
	}
	hasFoundMatch := false
	ok := false
	e := ExitOk
	scanner := bufio.NewScanner(bytes.NewReader(fileContentCache[rule.Path]))
	for scanner.Scan() {
		line := scanner.Text()
		if reg.Match([]byte(line)) {
			if len(lineToAdd) > 0 {
				if rule.StrictFmt && line != string(lineToAdd) {
					t.VerboseErrorf("pattern '%s' found in %s but not strictly equal to target\n", rule.Check, rule.Path)
				} else {
					t.VerboseInfof("line '%s' found in '%s'\n", line, rule.Path)
					ok = true
				}
			}
			if hasFoundMatch {
				t.Errorf("duplicate match of pattern '%s' in '%s'\n", rule.Check, rule.Path)
				e = e.Merge(ExitNok)
			}
			hasFoundMatch = true
		}
	}
	if len(lineToAdd) == 0 {
		if hasFoundMatch {
			t.VerboseErrorf("pattern '%s' found in %s\n", rule.Check, rule.Path)
		} else {
			t.VerboseInfof("pattern '%s' not found in %s\n", rule.Check, rule.Path)
			e = e.Merge(ExitNok)
		}
	} else if !ok {
		t.VerboseErrorf("line '%s' not found in '%s'\n", lineToAdd, rule.Path)
		e = e.Merge(ExitNok)
	} else if !hasFoundMatch {
		t.VerboseErrorf("pattern '%s' not found in %s\n", rule.Check, rule.Path)
		e = e.Merge(ExitNok)
	}
	return e
}

func (t CompFileincs) checkReplace(rule CompFileinc) ExitCode {
	e := ExitOk
	lineToAdd, err := t.getLineTochange(rule)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	reg, err := regexp.Compile(rule.Replace)
	if err != nil {
		t.Errorf("the regex in rule does not compile: %s\n", err)
		return ExitNok
	}
	scanner := bufio.NewScanner(bytes.NewReader(fileContentCache[rule.Path]))
	for scanner.Scan() {
		line := scanner.Text()
		if reg.Match([]byte(line)) {
			for _, stringMatch := range reg.FindAll([]byte(line), -1) {
				if line == string(lineToAdd) {
					t.VerboseInfof("%s : string '%s' found on target in line '%s'\n", rule.Path, stringMatch, line)
					continue
				}
				t.VerboseErrorf("%s : string '%s' should be replaced by '%s' in line '%s'\n", rule.Path, stringMatch, lineToAdd, line)
				e = e.Merge(ExitNok)
			}
		}
	}
	return e
}

func (t CompFileincs) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFileinc)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompFileincs) fixRule(rule CompFileinc) ExitCode {
	info, err := os.Stat(rule.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return ExitNok
		}
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if info.Size() > MAXSZ {
		return ExitNok
	}
	var e ExitCode
	switch rule.Check {
	case "":
		e = t.fixReplace(rule)
	default:
		e = t.fixCheck(rule)
	}
	if err = t.loadFileContentCache(rule.Path); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	return e
}

func (t CompFileincs) fixCheck(rule CompFileinc) ExitCode {
	reg, err := regexp.Compile(rule.Check)
	if err != nil {
		t.Errorf("the regex in rule does not compile: %s\n", err)
		return ExitNok
	}
	lineToAdd, err := t.getLineTochange(rule)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if !reg.Match(lineToAdd) {
		t.Errorf("rule error: '%s' does not match target content\n", rule.Check)
		return ExitNok
	}
	newFile, err := os.CreateTemp(filepath.Dir(rule.Path), "newFile")
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newFilePath := newFile.Name()
	oldFileStat, err := os.Stat(rule.Path)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	match := 0
	i := 0
	scanner := bufio.NewScanner(bytes.NewReader(fileContentCache[rule.Path]))
	for scanner.Scan() {
		i++
		line := scanner.Text()
		if reg.Match([]byte(line)) {
			match++
			if match == 1 {
				if rule.StrictFmt && line != string(lineToAdd) {
					t.Infof("rewrite %s:%d:'%s', new content: '%s'\n", rule.Path, i, line, lineToAdd)
					if _, err := newFile.Write(append(lineToAdd, '\n')); err != nil {
						t.Errorf("%s\n", err)
						return ExitNok
					}
				} else {
					if _, err := newFile.Write(append([]byte(line), '\n')); err != nil {
						t.Errorf("%s\n", err)
						return ExitNok
					}
				}
			} else if match > 1 {
				t.Infof("remove duplicate line %s:%d:'%s'\n", rule.Path, i, line)
			}
		} else {
			if _, err := newFile.Write(append([]byte(line), '\n')); err != nil {
				t.Errorf("%s\n", err)
				return ExitNok
			}
		}
	}
	if match == 0 && len(lineToAdd) > 0 {
		t.Infof("add line '%s' to %s\n", lineToAdd, rule.Path)
		if _, err := newFile.Write(append(lineToAdd, '\n')); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	}
	if err = newFile.Close(); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if err = os.Chmod(newFilePath, oldFileStat.Mode()); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if sysInfos := oldFileStat.Sys(); sysInfos != nil {
		if err = os.Chown(newFilePath, int(sysInfos.(*syscall.Stat_t).Uid), int(sysInfos.(*syscall.Stat_t).Gid)); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	} else {
		t.Errorf("can't change the owner of the file %s\n", newFilePath)
		return ExitNok
	}
	if err = os.Rename(newFilePath, rule.Path); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	t.Infof("adding %s in file %s\n", lineToAdd, rule.Path)
	return ExitOk
}

func (t CompFileincs) fixReplace(rule CompFileinc) ExitCode {
	reg, err := regexp.Compile(rule.Replace)
	if err != nil {
		t.Errorf("the regex in rule does not compile: %s\n", err)
		return ExitNok
	}
	lineToAdd, err := t.getLineTochange(rule)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newFile, err := os.CreateTemp(filepath.Dir(rule.Path), "newFile")
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	newFilePath := newFile.Name()
	oldFileStat, err := os.Stat(rule.Path)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	scanner := bufio.NewScanner(bytes.NewReader(fileContentCache[rule.Path]))
	for scanner.Scan() {
		line := scanner.Text()
		if _, err = newFile.Write(append([]byte(reg.ReplaceAllString(line, string(lineToAdd))), '\n')); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	}
	if err = newFile.Close(); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if err = os.Chmod(newFilePath, oldFileStat.Mode()); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if sysInfos := oldFileStat.Sys(); sysInfos != nil {
		if err = os.Chown(newFilePath, int(sysInfos.(*syscall.Stat_t).Uid), int(sysInfos.(*syscall.Stat_t).Gid)); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		}
	} else {
		t.Errorf("can't change the owner of the file %s\n", newFilePath)
		return ExitNok
	}
	if err = os.Rename(newFilePath, rule.Path); err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	t.Infof("replace the pattern %s with %s in file %s\n", rule.Replace, lineToAdd, rule.Path)
	return ExitOk
}

func (t CompFileincs) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFileinc)
		if t.checkRule(rule) == ExitNok {
			e = e.Merge(t.fixRule(rule))
		}
	}
	return e
}

func (t CompFileincs) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompFileincs) Info() ObjInfo {
	return compFileincInfo
}
