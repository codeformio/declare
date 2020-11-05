package javascript_test

import (
	"testing"

	"github.com/codeformio/declare/template"
	"github.com/codeformio/declare/template/javascript"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTemplate(t *testing.T) {
	tmpl := javascript.Templater{
		Files: map[string]string{
			"control.js": mainSrc,
			"utils.js":   utilsSrc,
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
	require.Len(t, out.Apply, 1)
	require.Equal(t, "my-name", out.Apply[0].GetName())

	// json.NewEncoder(os.Stdout).Encode(out)
}

const mainSrc = `
function sync(request) {
  var obj = request.object;
  var isExposed = hasDefinedPort(obj);

  var toApply = [];

  if (isExposed) {
    toApply.push({
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
    })
  }

  return { apply: toApply };
}
`

const utilsSrc = `
function hasDefinedPort(obj) {
  return obj.spec.hasOwnProperty('port') && obj.spec.port > 0;
}
`
