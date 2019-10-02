/*

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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// TemporaryClusterRoleBinding is the Schema for the temporaryclusterrolebindings API
type TemporaryClusterRoleBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// TTL is expiration time of temporary ClusterRoleBinding
	// +optional
	TTL string `json:"ttl,omitempty"`

	// Approver is an approver of ClusterRoleBinding
	Approver string `json:"approver,omitempty"`

	// The followings are same type which ClusterRoleBinding has
	Subjects []rbacv1.Subject `json:"subjects,omitempty"`
	RoleRef  rbacv1.RoleRef   `json:"roleRef,omitempty"`
}

// +kubebuilder:object:root=true

// TemporaryClusterRoleBindingList contains a list of TemporaryClusterRoleBinding
type TemporaryClusterRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemporaryClusterRoleBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemporaryClusterRoleBinding{}, &TemporaryClusterRoleBindingList{})
}
