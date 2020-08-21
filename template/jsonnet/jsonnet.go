package jsonnet

import (
	"fmt"
	"log"
	"strconv"

	"github.com/codeformio/declare/template"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"

	"k8s.io/apimachinery/pkg/util/json"
)

type Templater struct {
	Files map[string]string
}

func (t *Templater) Template(input *template.Input) (*template.Output, error) {
	vm := jsonnet.MakeVM()
	for _, ext := range extensions {
		vm.NativeFunction(ext)
	}

	jsonInput, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshalling input: %w", err)
	}
	vm.TLACode("request", string(jsonInput))

	var output template.Output
	for filename, source := range t.Files {
		jsonOutput, err := vm.EvaluateSnippet(filename, source)
		if err != nil {
			log.Printf("/%s input: %s", filename, jsonInput)
			log.Fatalf("/%s error: %s", filename, err)
		}

		if err := json.Unmarshal([]byte(jsonOutput), &output); err != nil {
			return nil, fmt.Errorf("unmarshalling output: %w", err)
		}

		// TODO: Support multiple files.
		break
	}

	return &output, nil
}

var extensions = []*jsonnet.NativeFunction{
	// jsonUnmarshal adds a native function for unmarshaling JSON,
	// since there doesn't seem to be one in the standard library.
	{
		Name:   "jsonUnmarshal",
		Params: ast.Identifiers{"jsonStr"},
		Func: func(args []interface{}) (interface{}, error) {
			jsonStr, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'jsonStr' arg", args[0])
			}
			val := make(map[string]interface{})
			if err := json.Unmarshal([]byte(jsonStr), &val); err != nil {
				return nil, fmt.Errorf("can't unmarshal JSON: %v", err)
			}
			return val, nil
		},
	},

	// parseInt adds a native function for parsing non-decimal integers,
	// since there doesn't seem to be one in the standard library.
	{
		Name:   "parseInt",
		Params: ast.Identifiers{"intStr", "base"},
		Func: func(args []interface{}) (interface{}, error) {
			str, ok := args[0].(string)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'intStr' arg", args[0])
			}
			base, ok := args[1].(float64)
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'base' arg", args[1])
			}
			intVal, err := strconv.ParseInt(str, int(base), 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse 'intStr': %v", err)
			}
			return float64(intVal), nil
		},
	},
}
