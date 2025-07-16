package commoncmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func Doc(items api.KeywordDefinitionItems, kind naming.Kind, driverName, kw string, depth int) (string, error) {
	var (
		buff string
		typ  string
	)
	index := keywords.ParseIndex(kw)
	section := index[0]
	depth += 1
	if driverName != "" {
		buff = fmt.Sprintf("%s %s\n\n", strings.Repeat("#", depth), driverName)
		l := strings.Split(driverName, ".")
		if len(l) > 1 {
			typ = l[1]
		}
	}
	slices.SortFunc(items, func(a, b api.KeywordDefinitionItem) int {
		return strings.Compare(a.Section+"."+a.Option, a.Section+"."+b.Option)
	})

	cfgL := make([]string, 0)
	cmdL := make([]string, 0)

	if typ != "" {
		cfgL = append(cfgL, fmt.Sprintf("\t%s = %s\n", "type", typ))
		cmdL = append(cmdL, fmt.Sprintf("--kw=\"%s=%s\"", "type", typ))
	}

	for _, item := range items {
		if item.Option == "type" {
			continue
		}
		if !item.Required {
			continue
		}
		cfgL = append(cfgL, fmt.Sprintf("\t%s = %s\n", item.Option, item.Example))
		cmdL = append(cmdL, fmt.Sprintf("--kw=\"%s=%s\"", item.Option, item.Example))
	}

	if len(cfgL) > 0 {
		buff += fmt.Sprint("Minimal configlet:\n\n")
		if driver.NewGroup(section) == driver.GroupUnknown {
			buff += fmt.Sprintf("\t[%s]\n", section)
		} else {
			buff += fmt.Sprintf("\t[%s#1]\n", section)
		}
		buff += strings.Join(cfgL, "") + "\n"
	}

	if len(cmdL) > 0 {
		buff += fmt.Sprint("Minimal setup command:\n\n")
		var selector string
		switch kind {
		case naming.KindInvalid:
			selector = "node"
		default:
			path := naming.Path{Namespace: "test", Kind: kind, Name: "foo"}
			selector = path.String()
		}
		if len(cmdL) > 1 {
			buff += fmt.Sprintf("\tom %s set \\\n\t\t", selector) + strings.Join(cmdL, " \\\n\t\t") + "\n\n"
		} else {
			buff += fmt.Sprintf("\tom %s set %s\n\n", selector, cmdL[0])
		}
	}

	for _, item := range items {
		buff += KeywordDoc(item, depth)
		buff += "\n"
	}
	return buff, nil
}

func KeywordDoc(item api.KeywordDefinitionItem, depth int) string {
	sprintProp := func(a, b string) string {
		return fmt.Sprintf("\t%-12s %s\n", a+":", b)
	}
	buff := fmt.Sprintf("%s %s\n\n", strings.Repeat("#", depth+1), item.Option)
	buff += sprintProp("required", fmt.Sprint(item.Required))
	buff += sprintProp("scopable", fmt.Sprint(item.Scopable))
	if len(item.Candidates) > 0 {
		buff += sprintProp("candidates", strings.Join(item.Candidates, ", "))
	}
	if len(item.Depends) > 0 {
		buff += sprintProp("depends", strings.Join(item.Depends, ", "))
	}
	if item.DefaultText != "" {
		buff += sprintProp("default", item.DefaultText)
	} else if item.Default != "" {
		buff += sprintProp("default", item.Default)
	}
	if item.Converter != "" {
		buff += sprintProp("convert", item.Converter)
	}
	buff += "\n"
	if item.Example != "" {
		buff += "Example:\n"
		buff += "\n"
		buff += "\t" + item.Option + " = " + item.Example + "\n"
		buff += "\n"
	}
	buff += item.Text
	buff += "\n"
	return buff
}
