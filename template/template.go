package template

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Input struct {
	Object *unstructured.Unstructured `json:"object"`
	Config map[string]string          `json:"config"`
	// Supported is a map of child types that are supported.
	// Key format = "<kind>.<version>.<group>".
	Supported map[string]bool `json:"supported"`
}

type Output struct {
	Apply []*unstructured.Unstructured `json:"apply"`
	// TODO: Should Object be here also to allow updating the .spec?
	Status map[string]interface{} `json:"status"`
}
