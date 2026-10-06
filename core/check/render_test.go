package check

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueText(t *testing.T) {
	for _, c := range []struct {
		value int64
		unit  string
		want  string
	}{
		{45, "%", "45%"},
		{2, "", "2"},
		{849000, "kb", "829Mi"},
		{1000, "Mb/s", "1Gb/s"},
		{100, "Mb/s", "100Mb/s"},
		{31, "C", "31°C"},
		{3, "inode", "3 inode"},
	} {
		assert.Equal(t, c.want, ValueText(c.value, c.unit), "%d %q", c.value, c.unit)
	}
}

// The value and the unit stay apart in the json, the text is only added.
func TestALineKeepsTheValueAndTheUnitApart(t *testing.T) {
	rs := NewResultSet()
	rs.Push(Result{DriverGroup: "fs_u", DriverName: "df", Instance: "/.free", Unit: "kb", Value: 849000})
	b, err := json.Marshal(rs.Lines())
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"fs_u","driver":"df","path":"","instance":"/.free","unit":"kb","value":849000,"value_text":"829Mi"}]`, string(b))
}
