/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type KrakenResourceDependency struct {
	Kind     string `json:"kind,omitempty"`
	Name     string `json:"name,omitempty"`
	Template string `json:"template,omitempty"`
}

type ConfigMapDependency struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key,omitempty"`
}

// DependencyRequestSpec defines the desired state of DependencyRequest
type DependencyRequestSpec struct {
	KrakenResourceDependencies []KrakenResourceDependency `json:"krakenResourceDependencies,omitempty"`

	ConfigMapDependencies []ConfigMapDependency `json:"configMapDependencies,omitempty"`
}

// DependencyRequestStatus defines the observed state of DependencyRequest
type DependencyRequestStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DependencyRequest is the Schema for the dependencyrequests API
type DependencyRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DependencyRequestSpec   `json:"spec,omitempty"`
	Status DependencyRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DependencyRequestList contains a list of DependencyRequest
type DependencyRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DependencyRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DependencyRequest{}, &DependencyRequestList{})
}
