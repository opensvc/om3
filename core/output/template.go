package output

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/danwakefield/fnmatch"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/util/unstructured"
)

func (t Renderer) renderTemplate(options string) (string, error) {
	drvName := func(i any) string {
		var drvID driver.ID
		switch a := i.(type) {
		case driver.ID:
			drvID = a
		case string:
			drvID = driver.Parse(a)
		}
		if drvID.IsZero() {
			return ""
		}
		return drvID.Name
	}
	drvGroup := func(i any) string {
		var drvID driver.ID
		switch a := i.(type) {
		case driver.ID:
			drvID = a
		case string:
			drvID = driver.Parse(a)
		}
		if drvID.IsZero() {
			return ""
		}
		return drvID.Group.String()
	}
	resName := func(i any) string {
		var rid *resourceid.T
		switch a := i.(type) {
		case *resourceid.T:
			rid = a
		case string:
			rid, _ = resourceid.Parse(a)
		}
		if rid == nil {
			return ""
		}
		return rid.Index()
	}
	resGroup := func(i any) string {
		var rid *resourceid.T
		switch a := i.(type) {
		case *resourceid.T:
			rid = a
		case string:
			rid, _ = resourceid.Parse(a)
		}
		if rid == nil {
			return ""
		}
		return rid.DriverGroup().String()
	}
	objKind := func(i any) string {
		var path naming.Path
		switch a := i.(type) {
		case naming.Path:
			path = a
		case string:
			path, _ = naming.ParsePath(a)
		}
		if path.IsZero() {
			return ""
		}
		return path.Kind.String()
	}
	objName := func(i any) string {
		var path naming.Path
		switch a := i.(type) {
		case naming.Path:
			path = a
		case string:
			path, _ = naming.ParsePath(a)
		}
		if path.IsZero() {
			return ""
		}
		return path.Name
	}
	objNamespace := func(i any) string {
		var path naming.Path
		switch a := i.(type) {
		case naming.Path:
			path = a
		case string:
			path, _ = naming.ParsePath(a)
		}
		if path.IsZero() {
			return ""
		}
		return path.Namespace
	}
	hasPrefix := func(s, prefix string) bool {
		return strings.HasPrefix(s, prefix)
	}
	hasSuffix := func(s, suffix string) bool {
		return strings.HasSuffix(s, suffix)
	}
	contains := func(slice any, entry any) (bool, error) {
		if s, ok := slice.([]string); ok {
			if e, ok := entry.(string); ok {
				return slices.Contains(s, e), nil
			}
		}
		if s, ok := slice.([]int); ok {
			if e, ok := entry.(int); ok {
				return slices.Contains(s, e), nil
			}
		}
		return false, nil
	}
	fnMatch := func(pattern, s string) bool {
		return fnmatch.Match(pattern, s, 0)
	}
	reMatch := func(pattern, s string) (bool, error) {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return false, err
		}

		return r.MatchString(s), nil
	}
	tmpl := template.New("output").Funcs(template.FuncMap{
		"drvName":      drvName,
		"drvGroup":     drvGroup,
		"resName":      resName,
		"resGroup":     resGroup,
		"objKind":      objKind,
		"objName":      objName,
		"objNamespace": objNamespace,
		"hasPrefix":    hasPrefix,
		"hasSuffix":    hasSuffix,
		"contains":     contains,
		"reMatch":      reMatch,
		"fnMatch":      fnMatch,
	})
	tmpl, err := tmpl.Parse(options)
	if err != nil {
		return "", err
	}

	var data any
	if i, ok := t.Data.(getItemser); ok {
		data = i.GetItems()
	} else {
		data = t.Data
	}
	unstructuredData, err := unstructured.NewListWithData(data)
	if err != nil {
		return "", err
	}
	w := bytes.NewBuffer([]byte{})
	err = tmpl.Execute(w, unstructuredData)
	return w.String(), err
}
