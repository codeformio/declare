package template

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Input struct {
	Object *unstructured.Unstructured `json:"object"`
}

type Output struct {
	Children []*unstructured.Unstructured `json:"children"`
	// TODO: Should Object be here also to allow updating the App?
}
