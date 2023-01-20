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

package v1alpha1

import "fmt"

// GetCondition of this resource
func (x *IPPrefix) GetCondition(ck ConditionKind) Condition {
	return x.Status.GetCondition(ck)
}

// SetConditions of the Network Node.
func (x *IPPrefix) SetConditions(c ...Condition) {
	x.Status.SetConditions(c...)
}

func (x *IPPrefix) GetGenericNamespacedName() string {
	return fmt.Sprintf("%s-%s", x.GetNamespace(), x.GetName())
}

func (x *IPPrefix) GetPrefixKind() PrefixKind {
	return x.Spec.PrefixKind
}
