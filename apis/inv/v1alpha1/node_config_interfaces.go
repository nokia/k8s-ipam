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

func (r *NodeConfig) GetResourceRequirements(defaultConstraints map[string]string) corev1.ResourceRequirements {
	constraints := defaultConstraints
	if len(r.Spec.Constraints) != 0 {
		// override the default constraints if they exist
		for k := range defaultConstraints {
			if v, ok := r.Spec.Constraints[k]; ok {
				constraints[k] = v
			}
		}
	}
	req := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{},
	}
	for k, v := range constraints {
		req.Requests[corev1.ResourceName(k)] = resource.MustParse(v)
	}
	return req
}
