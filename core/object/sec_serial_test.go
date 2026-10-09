package object

import (
	"math/big"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// The serial number is the generation and a uuid: an integer within the 20
// octets RFC 5280 allows, positive, said <generation>-<uuid> in the sec.
func TestCertSerial(t *testing.T) {
	serial, err := newCertSerial(maxSerialGeneration)
	require.NoError(t, err)
	assert.Equal(t, 1, serial.Sign(), "positive")
	assert.LessOrEqual(t, len(serial.Bytes()), 20, "20 octets at most")

	serial, err = newCertSerial(16)
	require.NoError(t, err)
	generation, id, ok := strings.Cut(serialText(serial), "-")
	require.True(t, ok, serialText(serial))
	assert.Equal(t, "16", generation)
	_, err = uuid.Parse(id)
	assert.NoError(t, err, "the uuid of %s", serialText(serial))

	_, err = newCertSerial(maxSerialGeneration + 1)
	assert.Error(t, err, "a generation the 20 octets can not hold")
}

func serialOf(t *testing.T, o Sec) (string, *big.Int) {
	t.Helper()
	text, err := o.DecodeKey("serial_number")
	require.NoError(t, err)
	b, err := o.DecodeKey("certificate")
	require.NoError(t, err)
	cert, err := certFromPEM(b)
	require.NoError(t, err)
	return string(text), cert.SerialNumber
}

// Each certificate of a sec is of the next generation, and the secs a ca
// signs, which each count from 1, have serial numbers of their own.
func TestGenCertSerial(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	ca, err := NewSec(naming.Path{Name: "ca", Kind: naming.KindSec, Namespace: "ns1"}, WithConfigData([]byte("[DEFAULT]\nbits = 2048\n")))
	require.NoError(t, err)
	require.NoError(t, ca.GenCert())

	newSec := func(name string) Sec {
		o, err := NewSec(naming.Path{Name: name, Kind: naming.KindSec, Namespace: "ns1"}, WithConfigData([]byte("[DEFAULT]\nca = ns1/sec/ca\nalt_names = node1\nbits = 2048\n")))
		require.NoError(t, err)
		return o
	}
	cert1, cert2 := newSec("cert1"), newSec("cert2")

	require.NoError(t, cert1.GenCert())
	s, serial1 := serialOf(t, cert1)
	assert.True(t, strings.HasPrefix(s, "1-"), s)
	assert.Equal(t, s, serialText(serial1), "the certificate carries the serial number of the key")

	require.NoError(t, cert1.GenCert())
	s, _ = serialOf(t, cert1)
	assert.True(t, strings.HasPrefix(s, "2-"), "the next generation: %s", s)

	require.NoError(t, cert2.GenCert())
	s, serial2 := serialOf(t, cert2)
	assert.True(t, strings.HasPrefix(s, "1-"), s)
	assert.NotEqual(t, serial1, serial2, "two secs of the ca, both of generation 1, have serial numbers of their own")
}

// A serial_number key of the time it was a counter alone is that counter: the
// next certificate is of the next generation.
func TestGenCertSerialAfterCounter(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := SetClusterConfig()
	require.NoError(t, err)

	o, err := NewSec(naming.Path{Name: "web", Kind: naming.KindSec, Namespace: "ns1"}, WithConfigData([]byte("[DEFAULT]\nalt_names = node1\nbits = 2048\n")))
	require.NoError(t, err)
	require.NoError(t, o.AddKey("serial_number", []byte("15")))
	require.NoError(t, o.GenCert())
	s, _ := serialOf(t, o)
	assert.True(t, strings.HasPrefix(s, "16-"), s)
}
