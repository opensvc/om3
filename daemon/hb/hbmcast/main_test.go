package hbmcast

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFragmentRoundTrip(t *testing.T) {
	msgID := uuid.New()
	chunk := []byte("the encrypted blob, opaque to this layer\x00\xff\x7b")

	f, err := decodeFragment(encodeFragment(msgID, 3, 7, chunk))
	require.NoError(t, err)
	assert.Equal(t, string(msgID[:]), f.MsgID)
	assert.Equal(t, 3, f.Index)
	assert.Equal(t, 7, f.Total)
	assert.Equal(t, chunk, f.Chunk)
}

// TestDecodeFragmentReadsTheJSONFraming is the compatibility guarantee: a
// peer on a release that predates the binary framing keeps being
// understood. The datagram is built the way that release built it.
func TestDecodeFragmentReadsTheJSONFraming(t *testing.T) {
	chunk := []byte{0x00, 0x01, 0xfe, 0xff}
	dgram, err := json.Marshal(fragment{MsgID: uuid.New().String(), Chunk: chunk, Index: 2, Total: 5})
	require.NoError(t, err)
	require.Equal(t, byte('{'), dgram[0], "the json framing has to stay distinguishable by its first byte")

	f, err := decodeFragment(dgram)
	require.NoError(t, err)
	assert.Equal(t, 2, f.Index)
	assert.Equal(t, 5, f.Total)
	assert.Equal(t, chunk, f.Chunk)
	assert.Len(t, f.MsgID, 36, "a json peer's message id is the uuid string")
}

// TestDecodeFragmentCopiesTheChunk pins the one thing that would corrupt
// messages silently. The receiver reads every datagram into one buffer
// and holds fragments in the assembly map until the message is complete,
// so a chunk pointing into that buffer is overwritten by the datagram
// after it.
func TestDecodeFragmentCopiesTheChunk(t *testing.T) {
	buf := encodeFragment(uuid.New(), 1, 2, []byte("first"))
	f, err := decodeFragment(buf)
	require.NoError(t, err)

	for i := range buf {
		buf[i] = 'X'
	}
	assert.Equal(t, []byte("first"), f.Chunk, "the chunk must not alias the read buffer")
}

func TestDecodeFragmentRejectsAnUnknownVersion(t *testing.T) {
	dgram := encodeFragment(uuid.New(), 1, 1, []byte("x"))
	dgram[3] = '9'
	_, err := decodeFragment(dgram)
	assert.Error(t, err, "an unknown framing version must not be guessed at")

	_, err = decodeFragment([]byte("neither json nor a fragment"))
	assert.Error(t, err)
}

// TestAFullChunkIsASendableDatagram is the arithmetic the json framing
// got wrong. MaxChunkSize is what every fragment but the last carries, so
// if a full one does not fit a datagram, no message over MaxChunkSize can
// be sent at all.
func TestAFullChunkIsASendableDatagram(t *testing.T) {
	const maxLegalUDPPayload = 65507

	dgram := encodeFragment(uuid.New(), 1, 2, make([]byte, MaxChunkSize))
	assert.LessOrEqual(t, len(dgram), maxLegalUDPPayload, "a full chunk must fit a UDP datagram")
	assert.LessOrEqual(t, len(dgram), MaxDatagramSize, "and the receiver's read buffer")

	// What it replaces, for the record: the same chunk as base64 in the
	// json envelope exceeded both.
	asJSON, err := json.Marshal(fragment{MsgID: uuid.New().String(), Chunk: make([]byte, MaxChunkSize), Index: 1, Total: 2})
	require.NoError(t, err)
	assert.Greater(t, len(asJSON), maxLegalUDPPayload,
		"if this ever fits, the bug this framing fixed is gone and the comment should say so")
	t.Logf("full chunk on the wire: binary %d bytes, json+base64 %d bytes", len(dgram), len(asJSON))
}

func BenchmarkDecodeFragmentBinary(b *testing.B) {
	dgram := encodeFragment(uuid.New(), 1, 2, make([]byte, MaxChunkSize))
	b.SetBytes(int64(len(dgram)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := decodeFragment(dgram); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeFragmentJSON(b *testing.B) {
	dgram, err := json.Marshal(fragment{MsgID: uuid.New().String(), Chunk: make([]byte, MaxChunkSize), Index: 1, Total: 2})
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(dgram)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := decodeFragment(dgram); err != nil {
			b.Fatal(err)
		}
	}
}
