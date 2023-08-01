/*
Copyright 2023 The Nephio Authors.

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
	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
)

// GetCondition returns the condition based on the condition kind
func (r *LogicalInterconnect) GetCondition(t resourcev1alpha1.ConditionType) resourcev1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *LogicalInterconnect) SetConditions(c ...resourcev1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *LogicalInterconnect) GetTopologies() []string {
	topologies := []string{}
	for _, ep := range r.Spec.Endpoints {
		topologies = append(topologies, ep.Topologies...)
	}
	return topologies
}
