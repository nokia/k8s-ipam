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

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

// GetCondition of this resource
func (x *NetworkInstance) GetCondition(ck ConditionKind) Condition {
	return x.Status.GetCondition(ck)
}

// SetConditions of the Network Node.
func (x *NetworkInstance) SetConditions(c ...Condition) {
	x.Status.SetConditions(c...)
}

func (x *NetworkInstance) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      x.Name,
		Namespace: x.Namespace,
	}
}

func (c *NetworkInstance) GetNameFromNetworkInstancePrefix(prefix string) string {
	return fmt.Sprintf("%s-%s-%s", c.GetName(), "aggregate", strings.ReplaceAll(prefix, "/", "-"))
}

func (x *NetworkInstance) GetGenericNamespacedName() string {
	return fmt.Sprintf("%s-%s", x.GetNamespace(), x.GetName())
}
