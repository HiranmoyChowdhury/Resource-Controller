package fake

import (
	corev1 "k8s.io/api/core/v1"
)

type RanChy struct {
	ObjectMeta        `json:"metadata,omitempty"`
	Status            RanChyStatus `json:"status"`
	Spec              RanChySpec   `json:"spec"`
	HideGeneratedInfo bool         `json:"HideGeneratedInfo,omitempty"`
}
type RanChySpec struct {
	DeletionPolicy DeletionPolicy    `json:"deletionPolicy,omitempty"`
	DeploymentSpec DeploymentSpec    `json:"deploymentSpec,omitempty"`
	ServiceSpec    ServiceSpec       `json:"serviceSpec,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

const (
	DeletionPolicyDelete  DeletionPolicy = "Delete"
	DeletionPolicyWipeOut DeletionPolicy = "WipeOut"
)

type DeletionPolicy string

type ObjectMeta struct {
	Name      string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
}

type DeploymentSpec struct {
	Name     string   `json:"name,omitempty"`
	Replicas *int32   `json:"replicas,omitempty"`
	Image    string   `json:"image"`
	Commands []string `json:"commands,omitempty"`
}
type ServiceSpec struct {
	Name        string             `json:"name,omitempty"`
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
	Port        *int32             `json:"port,omitempty"`
	TargetPort  *int32             `json:"targetPort,omitempty"`
	NodePort    *int32             `json:"NodePort,omitempty"`
}

type RanChyStatus struct {
	AvailableReplicas *int32 `json:"availableReplicas"`
}
