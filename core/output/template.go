package output

import (
	"bytes"
	"text/template"

	"github.com/opensvc/om3/util/unstructured"
)

func (t Renderer) renderTemplate(options string) (string, error) {
	tmpl := template.New("output")
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
