package jsonnet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/codeformio/declare/template"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
)

type Templater struct {
	Files map[string]string
}

func (t *Templater) Template(c client.Reader, input *template.Input) (*template.Output, error) {
	vm := jsonnet.MakeVM()
	for _, ext := range extensions {
		vm.NativeFunction(ext)
	}
	vm.NativeFunction(getObjectExt(c))

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

// getObjectExt gets an object from the k8s API server.
// It expectes an inputs like:
// { apiVersion: "", kind: "", metadata: { name: "" } }
func getObjectExt(c client.Reader) *jsonnet.NativeFunction {
	return &jsonnet.NativeFunction{
		Name:   "getObject",
		Params: ast.Identifiers{"obj"},
		Func: func(args []interface{}) (interface{}, error) {
			obj, ok := args[0].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("unexpected type %T for 'obj' arg", args[0])
			}
			meta, ok := obj["metadata"].(map[string]interface{})
			if !ok {
				return nil, errors.New("'obj' arg missing .metadata object field")
			}
			name, ok := meta["name"].(string)
			if !ok {
				return nil, errors.New("'obj' arg missing .metadata.name string field")
			}
			namespace, ok := meta["namespace"].(string)
			if !ok {
				namespace = "default"
			}
			apiV, ok := obj["apiVersion"].(string)
			if !ok {
				return nil, errors.New("'obj' arg missing .apiVersion string field")
			}
			kind, ok := obj["kind"].(string)
			if !ok {
				return nil, errors.New("'obj' arg missing .kind string field")
			}

			var res unstructured.Unstructured
			res.SetGroupVersionKind(schema.FromAPIVersionAndKind(apiV, kind))
			if err := c.Get(context.Background(), types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}, &res); err != nil {
				if apierrors.IsNotFound(err) {
					return map[string]interface{}{}, nil
				}
				return nil, fmt.Errorf("getting object: %v", err)
			}

			return cleanJSON(res.Object), nil
		},
	}
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

// cleanJSON switches ints to float64's to make the jsonnet interpreter happy (it does
// not like ints).
func cleanJSON(in interface{}) interface{} {
	switch v := in.(type) {
	case []interface{}:
		for i, elem := range v {
			v[i] = cleanJSON(elem)
		}

	case int:
		return float64(v)
	case int64:
		return float64(v)

	case map[string]interface{}:
		for key, val := range v {
			v[key] = cleanJSON(val)
		}

	}

	return in
}
