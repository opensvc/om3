package omcrypto

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unpooledCompress is the compress implementation from before the codec
// pooling, kept to assert the pooled one emits the very same bytes.
func unpooledCompress(b []byte) ([]byte, error) {
	var bb bytes.Buffer
	w := zlib.NewWriter(&bb)
	if _, err := w.Write(b); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return bb.Bytes(), nil
}

func testPayloads() [][]byte {
	large := make([]byte, 512*1024)
	for i := range large {
		large[i] = byte(i % 251)
	}
	return [][]byte{
		{},
		[]byte("a"),
		[]byte(`{"kind":"full","nodename":"dev2n1","gen":{"dev2n1":42}}`),
		bytes.Repeat([]byte("highly compressible "), 4096),
		large,
	}
}

func TestCompressRoundTrip(t *testing.T) {
	for i, payload := range testPayloads() {
		t.Run(fmt.Sprintf("payload%d", i), func(t *testing.T) {
			compressed, err := compress(payload)
			require.NoError(t, err)

			decompressed, err := decompress(compressed)
			require.NoError(t, err)
			assert.Equal(t, payload, decompressed)
		})
	}
}

// TestCompressIsUnchangedByPooling guards the wire format: a reused writer
// must emit the same stream a fresh one does, or a peer running another
// release would be decoding something else.
func TestCompressIsUnchangedByPooling(t *testing.T) {
	for i, payload := range testPayloads() {
		t.Run(fmt.Sprintf("payload%d", i), func(t *testing.T) {
			want, err := unpooledCompress(payload)
			require.NoError(t, err)

			// twice, so the second call runs on a writer the first
			// returned to the pool
			for _, round := range []string{"first", "reused"} {
				got, err := compress(payload)
				require.NoError(t, err, round)
				assert.Equal(t, want, got, round)
			}
		})
	}
}

// TestDecompressRejectsGarbage checks that a bad message does not poison the
// pooled reader for the messages that follow it.
func TestDecompressRejectsGarbage(t *testing.T) {
	good, err := compress([]byte("payload"))
	require.NoError(t, err)

	_, err = decompress([]byte("not a zlib stream"))
	assert.Error(t, err)

	decompressed, err := decompress(good)
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), decompressed)
}

func TestCompressIsConcurrencySafe(t *testing.T) {
	payloads := testPayloads()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 50; n++ {
				payload := payloads[(i+n)%len(payloads)]
				compressed, err := compress(payload)
				if !assert.NoError(t, err) {
					return
				}
				decompressed, err := decompress(compressed)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, payload, decompressed)
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkCompress(b *testing.B) {
	payload := bytes.Repeat([]byte(`{"path":"foo/svc/bar","avail":"up"},`), 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := compress(payload); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecompress(b *testing.B) {
	payload := bytes.Repeat([]byte(`{"path":"foo/svc/bar","avail":"up"},`), 64)
	compressed, err := compress(payload)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decompress(compressed); err != nil {
			b.Fatal(err)
		}
	}
}
