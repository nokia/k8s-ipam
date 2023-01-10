/*
Copyright 2022 Nokia.

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

package meta

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func AddLabels(o metav1.Object, labels map[string]string) {
	l := o.GetLabels()
	if l == nil {
		o.SetLabels(labels)
		return
	}
	for k, v := range labels {
		l[k] = v
	}
	o.SetLabels(l)
}

func RemoveLabels(o metav1.Object, labels ...string) {
	l := o.GetLabels()
	if l == nil {
		return
	}
	for _, k := range labels {
		delete(l, k)
	}
	o.SetLabels(l)
}
