/*
Copyright 2022.

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

// DiscoveryRuleSpec defines the desired state of DiscoveryRule
type DiscoveryRuleSpec struct {
	// discovery rule type: ip-range, rest api, netbox, consul,...
	Type string `json:"type,omitempty"`
	// wait period between discovery rule runs
	// +kubebuilder:default:="1m"
	Period metav1.Duration `json:"period,omitempty"`
	// enables the discovery rule
	Enabled bool `json:"enabled,omitempty"`
	// type specific fields
	// Properties runtime.RawExtension `json:"properties,omitempty"`
	// gNMI, netconf
	Protocol string `json:"protocol,omitempty"`
	// Port is the gNMI port number
	// +kubebuilder:default:=57400
	Port uint `json:"port,omitempty"`

	// secret name
	Credentials string `json:"credentials,omitempty"`
	// Insecure connection
	Insecure bool `json:"insecure,omitempty"`
	// certificate Name
	Certificate string `json:"certificate,omitempty"`
	// target namespace
	TargetNamespace string `json:"target-namespace,omitempty"`
	// target name template
	TargetNameTemplate string `json:"target-name-template,omitempty"`

	// IPRange Type
	// IP CIDR(s) to be scanned
	IPranges []string `json:"ip-ranges,omitempty"`
	// IP CIDR(s) to be excluded
	Excludes []string `json:"excludes,omitempty"`

	// API Type
	URL               string            `json:"url,omitempty"`
	Method            string            `json:"method,omitempty"`
	ResponseTemplate  string            `json:"response-template,omitempty"`
	APIInsecure       bool              `json:"api-insecure,omitempty"`
	CheckReachability bool              `json:"check-reachability,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	// TODO: should become a struct with username/password and/or token
	OAuth string `json:"oauth,omitempty"`

	// TopoWatch Type
	TopologyNamespace string `json:"topology-namespace,omitempty"`

	// NetBox Type

	// Consul Type
}

// DiscoveryRuleStatus defines the observed state of DiscoveryRule
type DiscoveryRuleStatus struct {
	StartTime int64 `json:"start-time,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DiscoveryRule is the Schema for the discoveryrules API
type DiscoveryRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiscoveryRuleSpec   `json:"spec,omitempty"`
	Status DiscoveryRuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DiscoveryRuleList contains a list of DiscoveryRule
type DiscoveryRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DiscoveryRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DiscoveryRule{}, &DiscoveryRuleList{})
}
