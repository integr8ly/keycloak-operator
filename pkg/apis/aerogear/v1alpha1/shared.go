package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

const (
	SharedServiceKind       = "SharedService"
	SharedServiceSliceKind  = "SharedServiceSlice"
	SharedServicePlanKind   = "SharedServicePlan"
	SharedServiceActionKind = "SharedServiceAction"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SharedServiceSpec   `json:"spec"`
	Status            SharedServiceStatus `json:"status"`
}

type SharedServiceStatus struct {
	CommonStatus
}

type CommonStatus struct {
	Ready bool `json:"ready"`
}

type SharedServiceSpec struct {
	MaxInstances int `json:"maxInstances"`
	MinInstances int `json:"minInstances"`
	MaxSlices    int `json:"maxSlices"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SharedService `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServiceSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SharedServiceSliceSpec   `json:"spec"`
	Status            SharedServiceSliceStatus `json:"status"`
}

type SharedServiceSliceSpec struct {
	ProvidedParams *runtime.RawExtension `json:"providedParams"`
	ServiceType    string                `json:"serviceType"`
}

type SharedServiceSliceStatus struct {
	CommonStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServiceSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SharedServiceSlice `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServicePlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SharedServicePlanSpec   `json:"spec"`
	Status            SharedServicePlanStatus `json:"status"`
}

type SharedServicePlanSpec struct {
}
type SharedServicePlanStatus struct {
	CommonStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServicePlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SharedServicePlan `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServiceAction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SharedServiceActionSpec   `json:"spec"`
	Status            SharedServiceActionStatus `json:"status"`
}

type SharedServiceActionSpec struct {
	ProvidedParams *runtime.RawExtension `json:"providedParams"`
	ServiceType    string                `json:"serviceType"`
}

type SharedServiceActionStatus struct {
	CommonStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharedServiceActionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SharedServiceAction `json:"items"`
}

// StatusSharedConfig manages the capacity of a shared service
type StatusSharedConfig struct {
	MaxSlices     int `json:"maxSlices"`
	CurrentSlices int `json:"currentSlices"`
}
