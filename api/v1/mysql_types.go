/*
 * @Author: Tomato
 * @Date: 2026-03-30 00:38:27
 */
/*
Copyright 2026.

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

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MySQLSpec defines the desired state of MySQL
type MySQLSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// 主节点配置
	// +required
	Master *MasterSpec `json:"master"`

	// cpu资源限制, 主从节点共享相同的配置
	// +required
	Cpu *resource.Quantity `json:"cpu"`

	// 内存资源限制, 主从节点共享相同的配置
	// +required
	Memory *resource.Quantity `json:"memory"`

	// 挂载的磁盘容量, 主从节点共享相同的配置
	// +required
	Storage *resource.Quantity `json:"storage"`

	// 存储插件配置, 不配置时为standard
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// 从节点配置
	// +optional
	Replica *ReplicaSpec `json:"replica,omitempty"`
}

type MasterSpec struct {
	// mysql root密码
	// +required
	RootPassword *string `json:"rootPassword"`
}

type ReplicaSpec struct {
	// 从节点数量
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Size *int32 `json:"size"`
}

// MySQLStatus defines the observed state of MySQL.
type MySQLStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the MySQL resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MySQL is the Schema for the mysqls API
type MySQL struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of MySQL
	// +required
	Spec MySQLSpec `json:"spec"`

	// status defines the observed state of MySQL
	// +optional
	Status MySQLStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MySQLList contains a list of MySQL
type MySQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []MySQL `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MySQL{}, &MySQLList{})
}
