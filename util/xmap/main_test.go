package xmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	tests := map[string]struct {
		data   interface{}
		output []string
	}{
		"empty map": {
			data:   map[string]string{},
			output: []string{},
		},
		"map with string index": {
			data:   map[string]string{"foo": "bar"},
			output: []string{"foo"},
		},
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		output := Keys(test.data)
		assert.Equal(t, test.output, output)
	}
}

func TestCopy(t *testing.T) {
	makeData := func() map[string]string {
		return map[string]string{"riri": "red", "fifi": "blue", "loulou": "green"}
	}

	t.Run("copy with no modifications", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		assert.Equal(t, makeData(), clone)
	})
	t.Run("copy and then a modification on the original map", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		ori["fifi"] = "brown"
		assert.Equal(t, clone["fifi"], "blue")
	})
	t.Run("copy and then a deletion on the original map", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		delete(ori, "fifi")
		_, ok := clone["fifi"]
		assert.Equal(t, ok, true)
	})
	t.Run("copy and then adding an element in the original map", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		ori["picsou"] = "black"
		_, ok := clone["picsou"]
		assert.Equal(t, ok, false)
	})
	t.Run("copy and then a modification on the copied map", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		clone["fifi"] = "brown"
		assert.Equal(t, ori["fifi"], "blue")
	})
	t.Run("copy and then a deletion on the copied map", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		delete(clone, "fifi")
		_, ok := ori["fifi"]
		assert.Equal(t, ok, true)
	})
	t.Run("copy and then adding an element in the copied map", func(t *testing.T) {
		ori := makeData()
		clone := Copy(ori)
		clone["picsou"] = "black"
		_, ok := ori["picsou"]
		assert.Equal(t, ok, false)
	})
}
