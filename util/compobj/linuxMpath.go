package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type (
	sectionMap   map[string]sectionMap
	MpathSection struct {
		Name   string
		Indent int
		Attr   map[string][]string
	}
	MpathBlackList struct {
		Name     string
		Wwids    []string
		Devnodes []string
		Devices  []MpathSection
	}
	MpathConf struct {
		BlackList           MpathBlackList
		BlackListExceptions MpathBlackList
		Defaults            MpathSection
		Devices             []MpathSection
		Multipaths          []MpathSection
		Overrides           MpathSection
	}
	CompMpaths struct {
		*Obj
	}
	CompMpath struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

var (
	MpathSectionsTree = sectionMap{
		"defaults": {},
		"blacklist": {
			"device": {},
		},
		"blacklist_exceptions": {
			"device": {},
		},
		"devices": {
			"device": {},
		},
		"multipaths": {
			"multipath": {},
		},
		"overrides": {},
	}
	compMpathInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_MPATH_",
		ExampleValue: []CompMpath{
			{
				Key:   "defaults.polling_interval",
				Op:    ">=",
				Value: 20,
			},
			{
				Key:   "device.{HP}.{HSV210.*}.prio",
				Op:    "=",
				Value: "alua",
			},
			{
				Key:   "blacklist.wwid",
				Op:    "=",
				Value: 600600000001,
			},
		},
		Description: `* Setup and verify the Linux native multipath configuration
`,
		FormDefinition: `Desc: |
  A rule to set a list of Linux multipath.conf parameters. Current values can be checked as equal, or superior/inferior to their target value.
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: linux_mpath
Inputs:
  -
    Id: key
    Label: Key
    DisplayModeTrim: 64
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: >
     The multipath.conf parameter to check.
     ex: defaults.polling_interval or
         device.device.{HP}.{HSV210.*} or
         multipaths.multipath.6006000000000000 or
         blacklist.wwid or
         blacklist.device.{HP}.{HSV210.*}
  -
    Id: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Default: "="
    Candidates:
      - "="
      - ">="
      - "<="
    Help: The comparison operator to use to check the parameter current value.
  -
    Id: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string or integer
    Help: The multipath.conf parameter target value.
`,
	}
)

func init() {
	m["linux_mpath"] = NewCompMpaths
}

func NewCompMpaths() interface{} {
	return &CompMpaths{
		Obj: NewObj(),
	}
}

func (t *CompMpaths) Add(s string) error {
	var data []CompMpath
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Key == "" {
			t.Errorf("key should be in the dict: %s\n", s)
			return fmt.Errorf("symlink should be in the dict: %s\n", s)
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=") {
			t.Errorf("op should be equal to =, >=, or <= in dict: %s\n", s)
			return fmt.Errorf("op should be equal to =, >=, or <= in dict: %s\n", s)
		}
		if rule.Value == nil {
			t.Errorf("value should be in dict: %s\n", s)
			return fmt.Errorf("value should be in dict: %s\n", s)
		}
		if _, ok := rule.Value.(float64); (rule.Op == ">=" || rule.Op == "<=") && !ok {
			t.Errorf("value should be an int when using operators >= or <= in dict: %s\n", s)
			return fmt.Errorf("value should be an int when using operators >= or <= in dict: %s\n", s)
		}
		_, okString := rule.Value.(string)
		_, okFloat64 := rule.Value.(float64)
		if !(okString || okFloat64) {
			t.Errorf("value should be an int or a string in dict: %s\n", s)
			return fmt.Errorf("value should be an int or a string in dict: %s\n", s)
		}
		t.Obj.Add(rule)
	}
	return nil
}

func (t CompMpaths) loadMpathData() (MpathConf, error) {
	mPathData := MpathConf{
		BlackList: MpathBlackList{
			Name:     "blacklist",
			Wwids:    []string{},
			Devnodes: []string{},
			Devices:  []MpathSection{},
		},
		BlackListExceptions: MpathBlackList{
			Name:     "blacklist_exceptions",
			Wwids:    []string{},
			Devnodes: []string{},
			Devices:  []MpathSection{},
		},
		Defaults: MpathSection{
			Name:   "default",
			Indent: 1,
			Attr:   map[string][]string{},
		},
		Devices:    []MpathSection{},
		Multipaths: []MpathSection{},
		Overrides: MpathSection{
			Name:   "overrides",
			Indent: 1,
			Attr:   map[string][]string{},
		},
	}
	buff, err := osReadFile(filepath.Join("/etc", "multipath.conf"))
	if err != nil {
		return MpathConf{}, err
	}
	buff = stripComments(buff)
	t.recursiveLoadFile(buff, MpathSectionsTree, []string{}, &mPathData, true)
	return mPathData, nil
}

func stripComments(buff []byte) []byte {
	newBuff := []byte{}
	scanner := bufio.NewScanner(bytes.NewReader(buff))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") && len(line) != 0 {
			if i := strings.Index(line, "#"); i != -1 {
				line = line[:i]
			}
			if len(line) > 0 {
				newBuff = append(newBuff, []byte(line+"\n")...)
			}
		}
	}
	return newBuff
}

func (t CompMpaths) recursiveLoadFile(buff []byte, sections sectionMap, chain []string, mPathData *MpathConf, originalCall bool) {
	for section, subsection := range sections {
		if originalCall {
			chain = []string{}
		}
		chain = append(chain, section)
		datas := t.loadSections(buff, section, originalCall)
		for _, data := range datas {
			t.loadKeyWords(data, subsection, chain, mPathData)
			t.recursiveLoadFile(data, subsection, chain, mPathData, false)
		}
	}
}

func (t CompMpaths) loadKeyWords(buff []byte, subsection sectionMap, chain []string, mPathData *MpathConf) {
	keywords := map[string][]string{}
	keyword := ""
	value := ""
	scanner := bufio.NewScanner(bytes.NewReader(buff))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		keyval := strings.SplitN(line, " ", 2)
		if len(keyval) != 2 {
			continue
		}
		keyword = strings.TrimSpace(keyval[0])
		value = strings.Trim(strings.TrimSpace(keyval[1]), `"`)
		if _, ok := subsection[keyword]; ok {
			continue
		}
		if (keyword == "wwid" || keyword == "devnode") && strings.HasPrefix(chain[len(chain)-1], "blacklist") {
			if _, ok := keywords[keyword]; !ok {
				keywords[keyword] = []string{value}
			} else {
				keywords[keyword] = append(keywords[keyword], value)
			}
		} else {
			keywords[keyword] = []string{value}
		}
	}
	switch {
	case chain[len(chain)-1] == "device" && chain[0] == "devices":
		mPathData.Devices = append(mPathData.Devices, MpathSection{
			Name:   "device",
			Indent: 2,
			Attr:   keywords,
		})
	case chain[len(chain)-1] == "multipath":
		mPathData.Multipaths = append(mPathData.Multipaths, MpathSection{
			Name:   "multipath",
			Indent: 2,
			Attr:   keywords,
		})
	case chain[len(chain)-1] == "device" && chain[0] == "blacklist":
		mPathData.BlackList.Devices = append(mPathData.BlackList.Devices, MpathSection{
			Name:   "device",
			Indent: 2,
			Attr:   keywords,
		})
	case chain[len(chain)-1] == "device" && chain[0] == "blacklist_exceptions":
		mPathData.BlackListExceptions.Devices = append(mPathData.BlackListExceptions.Devices, MpathSection{
			Name:   "device",
			Indent: 2,
			Attr:   keywords,
		})
	case chain[len(chain)-1] == "blacklist":
		if tmp, ok := keywords["wwid"]; ok {
			mPathData.BlackList.Wwids = tmp
		}
		if tmp, ok := keywords["devnode"]; ok {
			mPathData.BlackList.Devnodes = tmp
		}
	case chain[len(chain)-1] == "blacklist_exceptions":
		if tmp, ok := keywords["wwid"]; ok {
			mPathData.BlackListExceptions.Wwids = tmp
		}
		if tmp, ok := keywords["devnode"]; ok {
			mPathData.BlackListExceptions.Devnodes = tmp
		}
	case chain[len(chain)-1] == "defaults":
		mPathData.Defaults.Attr = keywords
	case chain[len(chain)-1] == "overrides":
		mPathData.Overrides.Attr = keywords
	}
}

func (t CompMpaths) loadSection(buff []byte, section string) ([]byte, []byte) {
	var start int
	if start = strings.Index(string(buff), section+" "); start == -1 {
		return nil, nil
	}
	buff = buff[start:]
	if start = strings.Index(string(buff), "{"); start == -1 {
		return nil, nil
	}
	depth := 1
	buff = buff[start+1:]
	for i, c := range buff {
		if c == '{' {
			depth += 1
		} else if c == '}' {
			depth -= 1
		}
		if depth == 0 {
			return buff[:i], buff[i+1:]
		}
	}
	return nil, nil
}

func (t CompMpaths) loadSections(buff []byte, section string, originalCall bool) [][]byte {
	sections := [][]byte{}
	if originalCall {
		b1, _ := t.loadSection(buff, section)
		return append(sections, b1)
	}
	for {
		b1, b2 := t.loadSection(buff, section)
		if b1 == nil && b2 == nil {
			break
		}
		buff = b2
		sections = append(sections, b1)
	}
	return sections
}

/*func (t CompMpaths) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		o := t.CheckSymlink(rule)
		e = e.Merge(o)
	}
	return e
}*/

/*func (t CompMpaths) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		e = e.Merge(t.fixSymlink(rule))
	}
	return e
}*/

func (t CompMpaths) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompMpaths) Info() ObjInfo {
	return compMpathInfo
}
