package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
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
	tloadMpathData    = CompMpaths{}.loadMpathData
	tgetConfValues    = CompMpaths{}.getConfValues
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
		rule.Key = strings.TrimSpace(rule.Key)
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
		if err := t.verifyDeviceAndMultipathInfos(rule.Key, s); err != nil {
			t.Errorf("%s\n", err)
			return err
		}
		t.Obj.Add(rule)
	}
	return nil
}

func (t CompMpaths) verifyDeviceAndMultipathInfos(key string, dict string) error {
	splitKey := strings.Split(key, ".")
	for _, val := range splitKey {
		if val == "device" {
			b, err := regexp.Match("device.{([^}]+)}.{([^}]+)}", []byte(key))
			if err != nil {
				return err
			}
			if !b {
				return fmt.Errorf("in the key field device must be used with the form: device.{vendor}.{product} in the dict: %s", dict)
			}
		} else if val == "multipath" {
			b, err := regexp.Match("multipath.{([^}]+)}", []byte(key))
			if err != nil {
				return err
			}
			if !b {
				return fmt.Errorf("in the key field multipath must be used with the form: multipath.{WWID} in the dict: %s", dict)
			}
		}
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

func (t CompMpaths) getConfValues(key string, conf MpathConf) ([]string, error) {
	indexs, newKey, err := t.getIndex(key)
	if err != nil {
		return nil, err
	}
	splitKey := strings.Split(newKey, ".")
	switch splitKey[0] {
	case "blacklist":
		if len(splitKey) < 2 {
			return nil, fmt.Errorf(`the key %s is malformed: blacklist must be followed by ".anotherSection"`, key)
		}
		switch splitKey[1] {
		case "wwid":
			return conf.BlackList.Wwids, nil
		case "devnode":
			return conf.BlackList.Devnodes, nil
		case "device":
			if len(splitKey) < 3 {
				return nil, fmt.Errorf(`the key %s is malformed: blacklist.device.{vendor}.{product} must be followed by ".anotherSection"`, key)
			}
			for _, device := range conf.BlackList.Devices {
				if device.Attr["vendor"][0] == indexs[0] && device.Attr["product"][0] == indexs[1] {
					return device.Attr[splitKey[2]], nil
				}
			}
			return []string{}, nil
		default:
			return nil, fmt.Errorf("the key %s is malformed: unkwnow section %s", key, splitKey[1])
		}
	case "blacklist_exceptions":
		if len(splitKey) < 2 {
			return nil, fmt.Errorf(`the key %s is malformed: blacklist_exceptions must be followed by ".anotherSection"`, key)
		}
		switch splitKey[1] {
		case "wwid":
			return conf.BlackListExceptions.Wwids, nil
		case "devnode":
			return conf.BlackListExceptions.Devnodes, nil
		case "device":
			if len(splitKey) < 3 {
				return nil, fmt.Errorf(`the key %s is malformed: blacklist_exceptions.device.{vendor}.{product} must be followed by ".anotherSection"`, key)
			}
			for _, device := range conf.BlackListExceptions.Devices {
				if device.Attr["vendor"][0] == indexs[0] && device.Attr["product"][0] == indexs[1] {
					return device.Attr[splitKey[2]], nil
				}
			}
			return []string{}, nil
		default:
			return nil, fmt.Errorf("the key %s is malformed: unkwnow section %s", key, splitKey[1])
		}
	case "default":
		if len(splitKey) < 2 {
			return nil, fmt.Errorf(`the key %s is malformed: default must be followed by ".anotherSection"`, key)
		}
		return conf.Defaults.Attr[splitKey[1]], nil
	case "devices":
		if len(splitKey) < 3 {
			return nil, fmt.Errorf(`the key %s is malformed: devices must be followed by ".device.{vendor}.{product}.attribute"`, key)
		}
		if splitKey[1] != "device" {
			return nil, fmt.Errorf(`the key %s is malformed: devices must be followed by ".device.{vendor}.{product}.attribute"`, key)
		}
		for _, device := range conf.Devices {
			if device.Attr["vendor"][0] == indexs[0] && device.Attr["product"][0] == indexs[1] {
				return device.Attr[splitKey[2]], nil
			}
		}
		return []string{}, nil
	case "multipaths":
		if len(splitKey) < 3 {
			return nil, fmt.Errorf(`the key %s is malformed: multipaths must be followed by ".multipath.{wwid}.attribute"`, key)
		}
		if splitKey[1] != "multipath" {
			return nil, fmt.Errorf(`the key %s is malformed: multipaths must be followed by ".multipath.{wwid}"`, key)
		}
		for _, multipath := range conf.Multipaths {
			if multipath.Attr["wwid"][0] == indexs[0] {
				return multipath.Attr[splitKey[2]], nil
			}
		}
		return []string{}, nil
	case "overrides":
		if len(splitKey) < 2 {
			return nil, fmt.Errorf(`the key %s is malformed: overrides must be followed by ".anotherSection"`, key)
		}
		return conf.Overrides.Attr[splitKey[1]], nil
	default:
		return nil, fmt.Errorf("the first word of key must be in: [blacklist, blacklist_exceptions, defaults, devices, multipaths, overrides] in key: %s", key)
	}
}

func (t CompMpaths) getIndex(key string) ([2]string, string, error) {
	reg, err := regexp.Compile(`device.{([^}]+)}.{([^}]+)}`)
	if err != nil {
		return [2]string{}, key, err
	}
	indexs := reg.FindStringSubmatch(key)
	if len(indexs) > 2 {
		return [2]string{strings.Trim(strings.TrimSpace(indexs[1]), `""`), strings.Trim(strings.TrimSpace(indexs[2]), `"`)}, reg.ReplaceAllString(key, "device"), nil
	}
	reg, err = regexp.Compile(`multipath.{([^}]+)}`)
	if err != nil {
		return [2]string{}, reg.ReplaceAllString(key, ""), err
	}
	indexs = reg.FindStringSubmatch(key)
	if len(indexs) > 1 {
		return [2]string{strings.Trim(strings.TrimSpace(indexs[1]), `""`), ""}, reg.ReplaceAllString(key, "multipath"), nil
	}
	return [2]string{}, key, nil
}

func (t CompMpaths) checkRule(rule CompMpath) ExitCode {
	conf, err := tloadMpathData()
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	values, err := tgetConfValues(rule.Key, conf)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	if len(values) == 0 {
		t.VerboseErrorf("the key %s is not set\n", rule.Key)
		return ExitNok
	}
	switch rule.Value.(type) {
	case string:
		for _, val := range values {
			if val == rule.Value {
				t.VerboseInfof("%s=%s on target\n", rule.Key, rule.Value)
				return ExitOk
			}
		}
		t.VerboseErrorf("%s=%s is not set\n", rule.Key, rule.Value)
		return ExitNok
	default:
		switch rule.Op {
		case ">=":
			for _, val := range values {
				fVal, err := strconv.ParseFloat(val, 64)
				if err != nil {
					if !errors.Is(err, strconv.ErrSyntax) {
						t.Errorf("%s\n", err)
						return ExitNok
					}
					continue
				}
				if fVal >= rule.Value.(float64) {
					t.VerboseInfof("%s=%s on target\n", rule.Key, val)
					return ExitOk
				}
			}
			t.VerboseErrorf("the values of %s are %s, one on these value should be greater than or equal to %d\n", rule.Key, values, int(rule.Value.(float64)))
			return ExitNok
		case "<=":
			for _, val := range values {
				fVal, err := strconv.ParseFloat(val, 64)
				if err != nil {
					if !errors.Is(err, strconv.ErrSyntax) {
						t.Errorf("%s\n", err)
						return ExitNok
					}
					continue
				}
				if fVal <= rule.Value.(float64) {
					t.VerboseInfof("%s=%s on target\n", rule.Key, val)
					return ExitOk
				}
			}
			t.VerboseErrorf("the values of %s are %s, one on these value should be less than or equal to %d\n", rule.Key, values, int(rule.Value.(float64)))
			return ExitNok
		default:
			for _, val := range values {
				fVal, err := strconv.ParseFloat(val, 64)
				if err != nil {
					if !errors.Is(err, strconv.ErrSyntax) {
						t.Errorf("%s\n", err)
						return ExitNok
					}
					continue
				}
				if fVal == rule.Value.(float64) {
					t.VerboseInfof("%s=%s on target\n", rule.Key, val)
					return ExitOk
				}
			}
			t.VerboseErrorf("the values of %s are %s, one on these value should be equal to %d\n", rule.Key, values, int(rule.Value.(float64)))
			return ExitNok
		}
	}
}

func (t CompMpaths) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompMpath)
		e = e.Merge(t.checkRule(rule))
	}
	return e
}

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
