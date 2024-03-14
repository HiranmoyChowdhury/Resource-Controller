package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Deletion Policy",type=string,JSONPath=`.spec.deletionPolicy`
// +kubebuilder:printcolumn:name="Availabe Replicas",type=integer,JSONPath=`.spec.status.availableReplicas`
// +kubebuilder:printcolumn:name="Image Tag",type=string,JSONPath=`.spec.deploymentSpec.image`
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=`.spec.serviceSpec.port`
// +kubebuilder:subresource:status

type RanChy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RanChySpec `json:"spec"`
	// +optional
	Status RanChyStatus `json:"status"`
}

// +kubebuilder:validation:Optional
type RanChySpec struct {
	// +optional
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
	// +optional
	DeploymentSpec DeploymentSpec `json:"deploymentSpec,omitempty"`
	// +optional
	ServiceSpec ServiceSpec `json:"serviceSpec,omitempty"`
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

const (
	DeletionPolicyDelete  DeletionPolicy = "Delete"
	DeletionPolicyWipeOut DeletionPolicy = "WipeOut"
)

type DeletionPolicy string

type DeploymentSpec struct {
	// +optional
	Name     string `json:"name,omitempty"`
	Replicas *int32 `json:"replicas,omitempty"`
	Image    string `json:"image"`
	// +optional
	Commands []string `json:"commands,omitempty"`
}
type ServiceSpec struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	ServiceType corev1.ServiceType `json:"type,omitempty"`
	Port        *int32             `json:"port,omitempty"`
	// +optional
	TargetPort *int32 `json:"targetPort,omitempty"`
	// +optional
	NodePort *int32 `json:"NodePort,omitempty"`
}

type RanChyStatus struct {
	AvailableReplicas *int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RanChyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RanChy `json:"items"`
}
