package jsondelta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshallUnmarshal(t *testing.T) {
	type testCase struct {
		op                    Operation
		expectedMarshalResult []byte
	}
	for _, tc := range []testCase{
		{
			op: Operation{
				OpPath:  OperationPath{"foo", "bar"},
				OpValue: NewOptValue([]interface{}{1, 2, "GO", []string{"a", "b"}}),
				OpKind:  "replace",
			},
			expectedMarshalResult: []byte(`[["foo","bar"],[1,2,"GO",["a","b"]]]`),
		},

		{
			op: Operation{
				OpPath: OperationPath{"foo", "bar"},
				OpKind: "remove",
			},
			expectedMarshalResult: []byte(`[["foo","bar"]]`),
		},
	} {
		inputOp := tc.op
		t.Run(inputOp.OpKind, func(t *testing.T) {
			marshalResult, err := json.Marshal(inputOp)
			require.Nil(t, err, "marshal error")
			require.Equal(t, tc.expectedMarshalResult, marshalResult,
				"unexpected marshalled value")

			outputOp := Operation{}
			err = json.Unmarshal(marshalResult, &outputOp)
			require.Nil(t, err, "unmarshal error")
			require.Equal(t, inputOp, outputOp)
			require.Equal(t, inputOp.OpPath, outputOp.OpPath)
			require.Equal(t, inputOp.OpKind, outputOp.OpKind)
			require.Equal(t, inputOp.OpValue, outputOp.OpValue)
		})
	}
}
