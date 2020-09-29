package jsonnet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanJSON(t *testing.T) {
	cases := []struct {
		name   string
		input  interface{}
		output interface{}
	}{
		{name: "int", input: int(1), output: float64(1)},
		{name: "string", input: "abc", output: "abc"},
		{name: "slice", input: []interface{}{"a", 123}, output: []interface{}{"a", float64(123)}},
		{name: "object", input: map[string]interface{}{"a": "b", "c": int(3)}, output: map[string]interface{}{"a": "b", "c": float64(3)}},
		{name: "objectWithSlice", input: map[string]interface{}{"a": "b", "c": int(3), "slice": []interface{}{int(5)}}, output: map[string]interface{}{"a": "b", "c": float64(3), "slice": []interface{}{float64(5)}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.output, cleanJSON(c.input))
		})
	}
}
