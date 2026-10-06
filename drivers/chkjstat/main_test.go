package chkjstat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	out := "  S0     S1     E      O      M     CCS    YGC     YGCT    FGC    FGCT     GCT\n" +
		"  0,00  99,80  45,12  12,40  97,33     -     12    0,123     0    0,000    0,123\n"
	assert.Equal(t, []metric{
		{"S0", 0}, {"S1", 100}, {"E", 45}, {"O", 12}, {"M", 97},
		{"YGC", 12}, {"YGCT", 0}, {"FGC", 0}, {"FGCT", 0}, {"GCT", 0},
	}, parse([]byte(out)))
}

func TestFromEnviron(t *testing.T) {
	j, ok := fromEnviron("12", []byte("PATH=/bin\x00OPENSVC_RID=app#1\x00OPENSVC_SVCPATH=ns1/svc/web\x00"))
	assert.True(t, ok)
	assert.Equal(t, jvm{pid: "12", instance: "app#1", path: "ns1/svc/web"}, j)
	_, ok = fromEnviron("13", []byte("PATH=/bin\x00"))
	assert.False(t, ok, "a java the agent did not start is not reported")
}
