package javascript

import (
	"fmt"

	"github.com/codeformio/declare/template"
	"github.com/robertkrimen/otto"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/util/json"
)

type Templater struct {
	Files map[string]string
}

func (t *Templater) Template(c client.Reader, input *template.Input) (*template.Output, error) {
	vm := otto.New()

	for fn, src := range t.Files {
		if _, err := vm.Run(src); err != nil {
			return nil, fmt.Errorf("%v: %w", fn, err)
		}
	}

	request := make(map[string]interface{})
	reqJsn, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshalling input to json: %w", err)
	}
	if err := json.Unmarshal(reqJsn, &request); err != nil {
		return nil, fmt.Errorf("unmarshalling input from json: %w", err)
	}

	val, err := vm.Call("sync", nil, request)
	if err != nil {
		return nil, fmt.Errorf("sync(request): %w", err)
	}

	exp, err := val.Export()
	if err != nil {
		return nil, fmt.Errorf("exporting return value: %w", err)
	}
	jsn, err := json.Marshal(exp)
	if err != nil {
		return nil, fmt.Errorf("marshalling return value to json: %w", err)
	}

	var output template.Output
	if err := json.Unmarshal(jsn, &output); err != nil {
		return nil, fmt.Errorf("unmarshalling json return value as expected output: %w", err)
	}

	return &output, nil
}
