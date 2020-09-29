package jsonnet_test

import (
	"testing"

	"github.com/codeformio/declare/template"
	"github.com/codeformio/declare/template/jsonnet"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTemplate(t *testing.T) {
	tmpl := jsonnet.Templater{
		Files: map[string]string{
			"source.jsonnet": source,
		},
	}

	out, err := tmpl.Template(nil, &template.Input{
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-name",
				},
				"spec": map[string]interface{}{
					"port": 80,
				},
			},
		},
	})

	require.NoError(t, err)
	require.Len(t, out.Children, 1)
	require.Equal(t, "my-name", out.Children[0].GetName())

	// json.NewEncoder(os.Stdout).Encode(out)
}

const source = `
function(request) {
  local obj = request.object,
  local isExposed = std.objectHas(obj.spec, 'port') && obj.spec.port > 0,
  children:
  (
    if isExposed then [
      {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: {
          name: obj.metadata.name,
        },
        spec: {
          selector: {
  	  app: obj.metadata.name,
  	},
          ports: [
            {
              protocol: 'TCP',
              port: obj.spec.port,
              targetPort: obj.spec.port,
            },
          ],
        },
      },
    ] else []
  )
}
`
