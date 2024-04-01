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
	"fmt"
	"reflect"

	"github.com/kraken-iac/common/types/option"
	corev1alpha1 "github.com/kraken-iac/kraken/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionTypeReady string = "Ready"

	ConfigTypeConfigMap string = "ConfigMap"
	ConfigTypeSecret    string = "Secret"
)

type ConfigExportEntry struct {
	Key           string `json:"key"`
	option.String `json:",inline"`
}

type ConfigMeta struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ConfigExportSpec defines the desired state of ConfigExport
type ConfigExportSpec struct {

	// +kubebuilder:validation:Enum=ConfigMap;Secret
	ConfigType string `json:"configType"`

	ConfigMeta ConfigMeta          `json:"configMetadata"`
	Entries    []ConfigExportEntry `json:"entries,omitempty"`
}

func (s ConfigExportSpec) GenerateDependencyRequestSpec() corev1alpha1.DependencyRequestSpec {
	dr := corev1alpha1.DependencyRequestSpec{}
	for _, entry := range s.Entries {
		if entry.ValueFrom != nil {
			entry.ValueFrom.AddToDependencyRequestSpec(&dr, reflect.String)
		}
	}
	return dr
}

func (s ConfigExportSpec) ToApplicableValues(depValues corev1alpha1.DependentValues) (map[string]string, error) {
	av := make(map[string]string, len(s.Entries))
	for _, entry := range s.Entries {
		if val, err := entry.ToApplicableValue(depValues); err != nil {
			return nil, err
		} else if val == nil {
			return nil, fmt.Errorf("no applicable value provided for %s", entry.Key)
		} else {
			av[entry.Key] = *val
		}
	}
	return av, nil
}

// ConfigExportStatus defines the observed state of ConfigExport
type ConfigExportStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ConfigExport is the Schema for the configexports API
type ConfigExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigExportSpec   `json:"spec,omitempty"`
	Status ConfigExportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigExportList contains a list of ConfigExport
type ConfigExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigExport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigExport{}, &ConfigExportList{})
}
