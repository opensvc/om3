package object

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

func TestParseRefsMode(t *testing.T) {
	for s, want := range map[string]RefsMode{"": RefsMissing, "missing": RefsMissing, "skip": RefsSkip, "overwrite": RefsOverwrite} {
		m, err := ParseRefsMode(s)
		require.NoError(t, err, s)
		assert.Equal(t, want, m, s)
	}
	_, err := ParseRefsMode("all")
	assert.Error(t, err)
}

// The keys of the referenced datastores are saved decoded and named relative
// to the namespace, and applied back as the mode says: the absent ones
// added, the differing ones left and reported, or rewritten, in the
// namespace of the object they are applied for.
func TestRefsDocumentRoundTrip(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	p := naming.Path{Name: "app", Kind: naming.KindCfg, Namespace: "ns1"}
	o, err := NewDataStore(p)
	require.NoError(t, err)
	require.NoError(t, o.AddKey("a", []byte("1")))
	require.NoError(t, o.AddKey("b", []byte("2")))

	doc, err := RefsDocumentOf("ns1", naming.Paths{p})
	require.NoError(t, err)
	assert.Equal(t, RefsDocument{"cfg/app": {"a": []byte("1"), "b": []byte("2")}}, doc)
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	var back RefsDocument
	require.NoError(t, json.Unmarshal(b, &back))
	assert.Equal(t, doc, back, "the document goes through json")

	_, err = RefsDocumentOf("ns2", naming.Paths{p})
	assert.Error(t, err, "a datastore out of the namespace")

	require.NoError(t, o.RemoveKey("a"))
	require.NoError(t, o.ChangeKey("b", []byte("changed")))

	notes, err := ApplyRefs("ns1", doc, RefsSkip)
	require.NoError(t, err)
	assert.Empty(t, notes)

	notes, err = ApplyRefs("ns1", doc, RefsMissing)
	require.NoError(t, err)
	assert.Equal(t, []string{"add ns1/cfg/app key a", "leave ns1/cfg/app key b, its value differs from the snapshot's"}, notes)
	o, err = NewDataStore(p)
	require.NoError(t, err)
	v, err := o.DecodeKey("b")
	require.NoError(t, err)
	assert.Equal(t, "changed", string(v))

	notes, err = ApplyRefs("ns1", doc, RefsOverwrite)
	require.NoError(t, err)
	assert.Equal(t, []string{"rewrite ns1/cfg/app key b"}, notes)
	o, err = NewDataStore(p)
	require.NoError(t, err)
	v, err = o.DecodeKey("b")
	require.NoError(t, err)
	assert.Equal(t, "2", string(v))

	notes, err = ApplyRefs("ns2", doc, RefsMissing)
	require.NoError(t, err)
	assert.Equal(t, []string{"create ns2/cfg/app", "add ns2/cfg/app key a", "add ns2/cfg/app key b"}, notes, "a clone into another namespace")

	_, err = ApplyRefs("ns1", RefsDocument{"svc/web": {"a": nil}}, RefsMissing)
	assert.Error(t, err, "only sec and cfg are applied")
}
