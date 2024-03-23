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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StateDeclarationSpec defines the desired state of StateDeclaration
type StateDeclarationSpec struct {
	// These types are supported: bool, int64, float64, string, []interface{}, map[string]interface{} and nil
	Data apiextensionsv1.JSON `json:"data,omitempty"`
}

//+kubebuilder:object:root=true

// StateDeclaration is the Schema for the statedeclarations API
type StateDeclaration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec StateDeclarationSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// StateDeclarationList contains a list of StateDeclaration
type StateDeclarationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StateDeclaration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StateDeclaration{}, &StateDeclarationList{})
}
