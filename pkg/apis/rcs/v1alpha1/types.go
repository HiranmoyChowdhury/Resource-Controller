package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RanChy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RanChySpec   `json:"spec"`
	Status            RanChyStatus `json:"status"`
}
type RanChySpec struct {
	DeletionPolicy `json:"deletionPolicy"`
	DeploymentSpec `json:"deploymentSpec"`
	ServiceSpec    `json:"serviceSpec"`
	Labels         map[string]string `json:"labels"`
}

const (
	DeletionPolicyDelete  DeletionPolicy = "Delete"
	DeletionPolicyWipeOut DeletionPolicy = "WipeOut"
)

type DeletionPolicy string

type DeploymentSpec struct {
	DeploymentName  string `json:"deploymentName"`
	Replicas        *int32 `json:"replicas"`
	DeploymentImage string `json:"deploymentImage"`
}

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeNodePort     ServiceType = "NodePort"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
	ServiceTypeHeadless     ServiceType = "Headless"
	ServiceTypeHeadless_    ServiceType = ""
)

type ServiceType string
type ServiceSpec struct {
	ServiceName string `json:"serviceName"`
	ServiceType `json:"serviceType"`
	ServicePort *int32 `json:"servicePort"`
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
