package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// Controller is the Schema for the controllers API
type Controller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControllerSpec   `json:"spec,omitempty"`
	Status ControllerStatus `json:"status,omitempty"`
}

// ControllerSpec defines the desired state of Controller
type ControllerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Source  map[string]string `json:"source,omitempty"`
	CRDName string            `json:"crdName,omitempty"`
	// Type of children this app produces.
	Children []ChildType    `json:"children,omitempty"`
	Config   []ConfigSource `json:"config,omitempty"`
}

type ChildType struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

type ConfigSource struct {
	Secret    string `json:"secret,omitempty"`
	ConfigMap string `json:"configMap,omitempty"`
}

// ControllerStatus defines the observed state of Controller
type ControllerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ControllerList contains a list of Controller
type ControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Controller `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Controller{}, &ControllerList{})
}
