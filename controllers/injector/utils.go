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

package injector

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/utils"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

func UpdateValue(fp string, target, value *kyaml.RNode) error {
	fieldPath := utils.SmarterPathSplitter(fp, ".")
	createdField, createErr := target.Pipe(kyaml.LookupCreate(value.YNode().Kind, fieldPath...))
	if createErr != nil {
		return fmt.Errorf("error creating node: %w", createErr)
	}
	var targetFields []*kyaml.RNode
	targetFields = append(targetFields, createdField)
	for _, t := range targetFields {
		if err := SetFieldValue(&types.FieldOptions{
			Create: true,
		}, t, value); err != nil {
			return err
		}
	}
	return nil
}

func SetFieldValue(options *types.FieldOptions, targetField *kyaml.RNode, value *kyaml.RNode) error {
	//fmt.Printf("setFieldValue options: %v\n", options)
	//fmt.Printf("setFieldValue targetField: %v\n", targetField.MustString())
	//fmt.Printf("setFieldValue value: %v\n", value.MustString())
	value = value.Copy()
	if options != nil && options.Delimiter != "" {
		if targetField.YNode().Kind != kyaml.ScalarNode {
			return fmt.Errorf("delimiter option can only be used with scalar nodes")
		}
		tv := strings.Split(targetField.YNode().Value, options.Delimiter)
		v := kyaml.GetValue(value)
		// TODO: Add a way to remove an element
		switch {
		case options.Index < 0: // prefix
			tv = append([]string{v}, tv...)
		case options.Index >= len(tv): // suffix
			tv = append(tv, v)
		default: // replace an element
			tv[options.Index] = v
		}
		value.YNode().Value = strings.Join(tv, options.Delimiter)
	}
	if targetField.YNode().Kind == kyaml.ScalarNode {
		// For scalar, only copy the value (leave any type intact to auto-convert int->string or string->int)
		targetField.YNode().Value = value.YNode().Value
	} else {
		targetField.SetYNode(value.YNode())
	}

	return nil
}
