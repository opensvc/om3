package converters

import (
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/golang-collections/collections/set"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A value is written the way a configuration writes it, and the text
// converts back to the same value.
func TestFormatRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		converter Converter
		in        string
		want      string
	}{
		{Duration, "10s", "10s"},
		{Duration, "2m", "2m"},
		{Duration, "120s", "2m"},
		{Duration, "1h30m", "1h30m"},
		{Duration, "90m", "1h30m"},
		{Duration, "1h", "1h"},
		{Duration, "2d", "48h"},
		{Duration, "500ms", "500ms"},
		{Duration, "1m30s", "1m30s"},
		{Duration, "0", "0s"},
		{Duration, "15", "15s"},
		{Size, "5g", "5g"},
		{Size, "5120m", "5g"},
		{Size, "1536m", "1536m"},
		{Size, "5368709120", "5g"},
		{Size, "1000", "1000"},
		{Size, "1k", "1k"},
		{Size, "0", "0"},
		{Size, "2t", "2t"},
		{List, "a  b c", "a b c"},
		{Shlex, `ls -l "my file" 'x'`, `ls -l "my file" x`},
		{Umask, "022", "022"},
		{FileMode, "0644", "644"},
		{FileMode, "2755", "2755"},
		{Int, "7", "7"},
		{Bool, "true", "true"},
		{String, "hello world", "hello world"},
	} {
		v, err := tc.converter.Convert(tc.in)
		require.NoError(t, err, "%s %q", tc.converter, tc.in)
		got := Format(tc.converter, v)
		assert.Equal(t, tc.want, got, "%s %q", tc.converter, tc.in)

		back, err := tc.converter.Convert(got)
		require.NoError(t, err, "%s %q converted back from %q", tc.converter, tc.in, got)
		assert.Equal(t, v, back, "%s %q converted back from %q", tc.converter, tc.in, got)
	}
}

// Nothing is written as the empty text: an unset value, a nil pointer, no
// converter and no value.
func TestFormatNothing(t *testing.T) {
	for _, c := range []Converter{Duration, Size, Umask, FileMode, User, Group, nil} {
		v, err := func() (any, error) {
			if c == nil {
				return nil, nil
			}
			return c.Convert("")
		}()
		require.NoError(t, err)
		assert.Equal(t, "", Format(c, v), "%v", c)
	}
	assert.Equal(t, "", Format(nil, nil))
	var d *time.Duration
	assert.Equal(t, "", Format(Duration, d))
	var n *int64
	assert.Equal(t, "", Format(Size, n))
}

// A set is written as its sorted words, a user and a group as their names,
// and a value of no known kind as it prints.
func TestFormatOthers(t *testing.T) {
	v, err := Set.Convert("b a c")
	require.NoError(t, err)
	assert.Equal(t, "a b c", Format(Set, v))
	assert.Equal(t, "", Format(nil, (*set.Set)(nil)))

	u := &user.User{Username: "root", Uid: "0"}
	assert.Equal(t, "root", Format(User, u))
	assert.Equal(t, "", Format(User, (*user.User)(nil)))
	g := &user.Group{Name: "wheel", Gid: "10"}
	assert.Equal(t, "wheel", Format(Group, g))
	assert.Equal(t, "", Format(Group, (*user.Group)(nil)))

	i := int64(42)
	assert.Equal(t, "42", Format(nil, &i))
	assert.Equal(t, "3.5", Format(Float64, 3.5))
	mode := os.FileMode(0o022)
	assert.Equal(t, "022", Format(Umask, mode))
	assert.Equal(t, "5g", Format(Size, int64(5<<30)))
	assert.Equal(t, "2m", Format(Duration, 2*time.Minute))
	assert.Equal(t, "x", Format(Size, "x"), "a value of another type than the converter's is written as it is")
	assert.Equal(t, "x", Format(Duration, "x"))
}
