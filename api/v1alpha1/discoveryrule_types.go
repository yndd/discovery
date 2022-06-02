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
	"bytes"
	"text/template"

	targetv1 "github.com/yndd/target/apis/target/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelKeyDiscoveryRule = "discovery.yndd.io/discovery-rule"
	LabelKeyVendorType    = "discovery.yndd.io/vendor-type"
)

// DiscoveryRuleSpec defines the desired state of DiscoveryRule
// +kubebuilder:subresource:ipRange
type DiscoveryRuleSpec struct {
	// enables the discovery rule
	Enabled bool `json:"enabled,omitempty"`

	// wait period between discovery rule runs
	// +kubebuilder:default:="1m"
	Period metav1.Duration `json:"period,omitempty"`

	// gNMI, netconf
	Protocol string `json:"protocol,omitempty"`

	// Port is the gNMI port number
	// +kubebuilder:default:=57400
	Port uint `json:"port,omitempty"`

	// secret name where the credentials used to access the target are stored
	Credentials string `json:"credentials,omitempty"`

	// Insecure connection
	Insecure bool `json:"insecure,omitempty"`

	// certificate Name
	Certificate string `json:"certificate,omitempty"`

	// target template
	TargetTemplate *TargetTemplate `json:"targetTemplate,omitempty"`
	// IP range discovery rule
	IPRange *IPRangeRule `json:"ipRange,omitempty"`
	// API discovery rule
	APIRule *APIRule `json:"apiRule,omitempty"`
	// Topology discovery rule
	TopologyRule *TopologyRule `json:"topologyRule,omitempty"`
	// NetBox Type

	// Consul Type
}

type IPRangeRule struct {
	// list of CIDR(s) to be scanned
	CIDRs []string `json:"cidrs,omitempty"`
	// IP CIDR(s) to be excluded
	Excludes []string `json:"excludes,omitempty"`
	// number of concurrent IP scan
	ConcurrentScans int64 `json:"concurrentScans,omitempty"`
}

type APIRule struct {
	URL               string            `json:"url,omitempty"`
	Method            string            `json:"method,omitempty"`
	ResponseTemplate  string            `json:"responseTemplate,omitempty"`
	APIInsecure       bool              `json:"insecure,omitempty"`
	CheckReachability bool              `json:"checkReachability,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	// TODO: should become a struct with username/password and/or token
	OAuth string `json:"oauth,omitempty"`
}

type TopologyRule struct {
	// topology namespace
	Namespace string `json:"namespace,omitempty"`
	// topology name
	Name string `json:"name,omitempty"`
}

type TargetTemplate struct {
	// target namespace
	Namespace string `json:"namespace,omitempty"`

	// target name template
	NameTemplate string `json:"nameTemplate,omitempty"`

	// Annotations is a key value map to be copied to the target CR.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels is a key value map to be copied to the target CR.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// DiscoveryRuleStatus defines the observed state of DiscoveryRule
type DiscoveryRuleStatus struct {
	StartTime int64  `json:"startTime,omitempty"`
	Type      string `json:"type,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ENABLED",type="boolean",JSONPath=".spec.enabled",description="True if the discovery rule is enabled"
// +kubebuilder:printcolumn:name="PROTOCOL",type="string",JSONPath=".spec.protocol",description="Protocol used discover the target"
// +kubebuilder:printcolumn:name="PERIOD",type="string",JSONPath=".spec.period",description="Wait period between discovery rule runs"
// +kubebuilder:printcolumn:name="CREDENTIALS",type="string",JSONPath=".spec.credentials",description="Secret name where the credentials used to access the target are stored"
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

func (dr *DiscoveryRule) GetTargetLabels(t *targetv1.TargetSpec) (map[string]string, error) {
	if dr.Spec.TargetTemplate == nil {
		return map[string]string{
			LabelKeyVendorType:    string(t.Properties.VendorType),
			LabelKeyDiscoveryRule: dr.GetName(),
		}, nil
	}
	return dr.buildTags(dr.Spec.TargetTemplate.Labels, t)
}

func (dr *DiscoveryRule) GetTargetAnnotations(t *targetv1.TargetSpec) (map[string]string, error) {
	if dr.Spec.TargetTemplate == nil {
		return map[string]string{
			LabelKeyVendorType:    string(t.Properties.VendorType),
			LabelKeyDiscoveryRule: dr.GetName(),
		}, nil
	}
	return dr.buildTags(dr.Spec.TargetTemplate.Annotations, t)
}

func (dr *DiscoveryRule) buildTags(m map[string]string, t *targetv1.TargetSpec) (map[string]string, error) {
	// initialize map if empty
	if m == nil {
		m = make(map[string]string)
	}
	// add vendor-type and discovery-rule labels
	if t != nil {
		if _, ok := m[LabelKeyVendorType]; !ok {
			m[LabelKeyVendorType] = string(t.Properties.VendorType)
		}
		if _, ok := m[LabelKeyDiscoveryRule]; !ok {
			m[LabelKeyDiscoveryRule] = dr.GetName()
		}
	}
	// render values templates
	labels := make(map[string]string, len(m))
	b := new(bytes.Buffer)
	for k, v := range m {
		tpl, err := template.New(k).Parse(v)
		if err != nil {
			return nil, err
		}
		b.Reset()
		err = tpl.Execute(b, t)
		if err != nil {
			return nil, err
		}
		labels[k] = b.String()
	}
	return labels, nil
}
