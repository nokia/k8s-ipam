/*
Copyright 2023 Nokia.

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func (r *NodeConfig) GetModel(defaultModel string) string {
	model := defaultModel
	if r.Spec.Model != nil {
		model = *r.Spec.Model
	}
	return model
}

func (r *NodeConfig) GetImage(defaultImageName string) string {
	image := defaultImageName
	if r.Spec.Image != nil {
		image = *r.Spec.Image
	}
	return image
}

func (r *NodeConfig) GetResourceRequirements(defaultResourceLimits, defaultResourceRequests map[string]string) corev1.ResourceRequirements {
	if len(r.Spec.Resources.Limits) == 0 {
		r.Spec.Resources.Limits = corev1.ResourceList{}
		for k, v := range defaultResourceLimits {
			r.Spec.Resources.Limits[corev1.ResourceName(k)] = resource.MustParse(v)
		}
	}

	if len(r.Spec.Resources.Requests) == 0 {
		r.Spec.Resources.Requests = corev1.ResourceList{}
		for k, v := range defaultResourceRequests {
			r.Spec.Resources.Requests[corev1.ResourceName(k)] = resource.MustParse(v)
		}
	}
	return r.Spec.Resources
}
