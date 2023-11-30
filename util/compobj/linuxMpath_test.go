package main

import (
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestMpathAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRules     string
		expectError   bool
		expectedRules []interface{}
	}{
		"with a full rule": {
			jsonRules:   `[{"key":"lala", "op":"=", "value" : "ok"}]`,
			expectError: false,
			expectedRules: []interface{}{CompMpath{
				Key:   "lala",
				Op:    "=",
				Value: "ok",
			}},
		},

		"with missing key": {
			jsonRules:     `[{"op":"=", "value" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with missing op": {
			jsonRules:     `[{"key":"lala", "value" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with missing value": {
			jsonRules:     `[{"op":"=", "key" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with wrong op": {
			jsonRules:     `[{"key":"lala", "op":">>>", "value" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"when value is a bool": {
			jsonRules:     `[{"key":"lala", "op":"=", "value" : true}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with string value and op >=": {
			jsonRules:     `[{"key":"lala", "op":">=", "value" : "true"}]`,
			expectError:   true,
			expectedRules: nil,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompMpaths{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRules))
			} else {
				require.NoError(t, obj.Add(c.jsonRules))
				require.Equal(t, c.expectedRules, obj.rules)
			}
		})
	}
}

func TestLoadMpathData(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	testCases := map[string]struct {
		filePath     string
		expectedData MpathConf
	}{
		"with only a default section": {
			filePath: "./testdata/linuxMpath_conf_default",
			expectedData: MpathConf{
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
					Attr:   map[string][]string{"user_friendly_names": {"yes"}, "path_grouping_policy": {"multibus"}},
				},
				Devices:    []MpathSection{},
				Multipaths: []MpathSection{},
				Overrides: MpathSection{
					Name:   "overrides",
					Indent: 1,
					Attr:   map[string][]string{},
				},
			},
		},

		"with only blacklist section": {
			filePath: "./testdata/linuxMpath_conf_blacklist",
			expectedData: MpathConf{
				BlackList: MpathBlackList{
					Name:     "blacklist",
					Wwids:    []string{"*", `laal`},
					Devnodes: []string{`^hd[a-z]`},
					Devices: []MpathSection{{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					}},
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
			},
		},

		"with only blacklist_exceptions section": {
			filePath: "./testdata/linuxMpath_conf_blacklist_exceptions",
			expectedData: MpathConf{
				BlackList: MpathBlackList{
					Name:     "blacklist",
					Wwids:    []string{},
					Devnodes: []string{},
					Devices:  []MpathSection{},
				},
				BlackListExceptions: MpathBlackList{
					Name:     "blacklist_exceptions",
					Wwids:    []string{"*", `laal`},
					Devnodes: []string{`^hd[a-z]`},
					Devices: []MpathSection{{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					}},
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
			},
		},

		"with only devices section": {
			filePath: "./testdata/linuxMpath_conf_devices",
			expectedData: MpathConf{
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
				Devices: []MpathSection{
					{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					},
				},
				Multipaths: []MpathSection{},
				Overrides: MpathSection{
					Name:   "overrides",
					Indent: 1,
					Attr:   map[string][]string{}},
			},
		},

		"with only multipaths section": {
			filePath: "./testdata/linuxMpath_conf_multipaths",
			expectedData: MpathConf{
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
				Devices: []MpathSection{},
				Multipaths: []MpathSection{{
					Name:   "multipath",
					Indent: 2,
					Attr:   map[string][]string{"wwid": {"3600508b4000156d70001200000b0000"}},
				},
					{
						Name:   "multipath",
						Indent: 2,
						Attr:   map[string][]string{"wwid": {"1DEC_____321816758474"}, "alias": {"red"}, "rr_weight": {"priorities"}},
					},
				},
				Overrides: MpathSection{
					Name:   "overrides",
					Indent: 1,
					Attr:   map[string][]string{},
				},
			},
		},

		"with only a default override": {
			filePath: "./testdata/linuxMpath_conf_overrides",
			expectedData: MpathConf{
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
					Attr:   map[string][]string{"user_friendly_names": {"yes"}, "path_grouping_policy": {"multibus"}},
				},
			},
		},

		"with a full multipath file": {
			filePath: "./testdata/linuxMpath_conf_golden",
			expectedData: MpathConf{
				BlackList: MpathBlackList{
					Name:     "blacklist",
					Wwids:    []string{"*", `laal`},
					Devnodes: []string{`^hd[a-z]`},
					Devices: []MpathSection{{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					}},
				},
				BlackListExceptions: MpathBlackList{
					Name:     "blacklist_exceptions",
					Wwids:    []string{"*", `laal`},
					Devnodes: []string{`^hd[a-z]`},
					Devices: []MpathSection{{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					}},
				},
				Defaults: MpathSection{
					Name:   "default",
					Indent: 1,
					Attr:   map[string][]string{"user_friendly_names": {"yes"}, "path_grouping_policy": {"multibus"}},
				},
				Devices: []MpathSection{
					{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					},
				},
				Multipaths: []MpathSection{{
					Name:   "multipath",
					Indent: 2,
					Attr:   map[string][]string{"wwid": {"3600508b4000156d70001200000b0000"}},
				},
					{
						Name:   "multipath",
						Indent: 2,
						Attr:   map[string][]string{"wwid": {"1DEC_____321816758474"}, "alias": {"red"}, "rr_weight": {"priorities"}},
					},
				},
				Overrides: MpathSection{
					Name:   "overrides",
					Indent: 1,
					Attr:   map[string][]string{"user_friendly_names": {"yes"}, "path_grouping_policy": {"multibus"}},
				},
			},
		},
		"with a full multipath file and a different order": {
			filePath: "./testdata/linuxMpath_conf_golden2",
			expectedData: MpathConf{
				BlackList: MpathBlackList{
					Name:     "blacklist",
					Wwids:    []string{"*", `laal`},
					Devnodes: []string{`^hd[a-z]`},
					Devices: []MpathSection{{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					}},
				},
				BlackListExceptions: MpathBlackList{
					Name:     "blacklist_exceptions",
					Wwids:    []string{"*", `laal`},
					Devnodes: []string{`^hd[a-z]`},
					Devices: []MpathSection{{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					}},
				},
				Defaults: MpathSection{
					Name:   "default",
					Indent: 1,
					Attr:   map[string][]string{"user_friendly_names": {"yes"}, "path_grouping_policy": {"multibus"}},
				},
				Devices: []MpathSection{
					{
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"IBM"}, "product": {"3S42"}},
					}, {
						Name:   "device",
						Indent: 2,
						Attr:   map[string][]string{"vendor": {"HP"}, "product": {"*"}},
					},
				},
				Multipaths: []MpathSection{{
					Name:   "multipath",
					Indent: 2,
					Attr:   map[string][]string{"wwid": {"3600508b4000156d70001200000b0000"}},
				},
					{
						Name:   "multipath",
						Indent: 2,
						Attr:   map[string][]string{"wwid": {"1DEC_____321816758474"}, "alias": {"red"}, "rr_weight": {"priorities"}},
					},
				},
				Overrides: MpathSection{
					Name:   "overrides",
					Indent: 1,
					Attr:   map[string][]string{"user_friendly_names": {"yes"}, "path_grouping_policy": {"multibus"}},
				},
			},
		},
	}

	obj := CompMpaths{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.filePath)
			}
			mPathData, err := obj.loadMpathData()
			require.NoError(t, err)
			require.Equal(t, "", cmp.Diff(c.expectedData, mPathData))
		})
	}
}
