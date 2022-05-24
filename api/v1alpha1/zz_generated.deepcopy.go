//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIRule) DeepCopyInto(out *APIRule) {
	*out = *in
	if in.Headers != nil {
		in, out := &in.Headers, &out.Headers
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIRule.
func (in *APIRule) DeepCopy() *APIRule {
	if in == nil {
		return nil
	}
	out := new(APIRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiscoveryRule) DeepCopyInto(out *DiscoveryRule) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiscoveryRule.
func (in *DiscoveryRule) DeepCopy() *DiscoveryRule {
	if in == nil {
		return nil
	}
	out := new(DiscoveryRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DiscoveryRule) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiscoveryRuleList) DeepCopyInto(out *DiscoveryRuleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DiscoveryRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiscoveryRuleList.
func (in *DiscoveryRuleList) DeepCopy() *DiscoveryRuleList {
	if in == nil {
		return nil
	}
	out := new(DiscoveryRuleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DiscoveryRuleList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiscoveryRuleSpec) DeepCopyInto(out *DiscoveryRuleSpec) {
	*out = *in
	out.Period = in.Period
	if in.IPRange != nil {
		in, out := &in.IPRange, &out.IPRange
		*out = new(IPRangeRule)
		(*in).DeepCopyInto(*out)
	}
	if in.APIRule != nil {
		in, out := &in.APIRule, &out.APIRule
		*out = new(APIRule)
		(*in).DeepCopyInto(*out)
	}
	if in.TopologyRule != nil {
		in, out := &in.TopologyRule, &out.TopologyRule
		*out = new(TopologyRule)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiscoveryRuleSpec.
func (in *DiscoveryRuleSpec) DeepCopy() *DiscoveryRuleSpec {
	if in == nil {
		return nil
	}
	out := new(DiscoveryRuleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DiscoveryRuleStatus) DeepCopyInto(out *DiscoveryRuleStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DiscoveryRuleStatus.
func (in *DiscoveryRuleStatus) DeepCopy() *DiscoveryRuleStatus {
	if in == nil {
		return nil
	}
	out := new(DiscoveryRuleStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IPRangeRule) DeepCopyInto(out *IPRangeRule) {
	*out = *in
	if in.CIDRs != nil {
		in, out := &in.CIDRs, &out.CIDRs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Excludes != nil {
		in, out := &in.Excludes, &out.Excludes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPRangeRule.
func (in *IPRangeRule) DeepCopy() *IPRangeRule {
	if in == nil {
		return nil
	}
	out := new(IPRangeRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TopologyRule) DeepCopyInto(out *TopologyRule) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TopologyRule.
func (in *TopologyRule) DeepCopy() *TopologyRule {
	if in == nil {
		return nil
	}
	out := new(TopologyRule)
	in.DeepCopyInto(out)
	return out
}
