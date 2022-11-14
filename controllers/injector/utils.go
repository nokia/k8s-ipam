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
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
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

// Code adapted from Porch internal cmdrpkgpull and cmdrpkgpush
func ResourcesToPackageBuffer(resources map[string]string) (*kio.PackageBuffer, error) {
	keys := make([]string, 0, len(resources))
	//fmt.Printf("ResourcesToPackageBuffer: resources: %v\n", len(resources))
	for k := range resources {
		//fmt.Printf("ResourcesToPackageBuffer: resources key %s\n", k)
		if !includeFile(k) {
			continue
		}
		//fmt.Printf("ResourcesToPackageBuffer: resources key append %s\n", k)
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create kio readers
	inputs := []kio.Reader{}
	for _, k := range keys {
		//fmt.Printf("ResourcesToPackageBuffer: key %s\n", k)
		v := resources[k]
		inputs = append(inputs, &kio.ByteReader{
			Reader: strings.NewReader(v),
			SetAnnotations: map[string]string{
				kioutil.PathAnnotation: k,
			},
			DisableUnwrapping: true,
		})
	}

	var pb kio.PackageBuffer
	err := kio.Pipeline{
		Inputs:  inputs,
		Outputs: []kio.Writer{&pb},
	}.Execute()

	if err != nil {
		return nil, err
	}

	return &pb, nil
}

// import fails with kptfile/v1
var matchResourceContents = append(kio.MatchAll, "Kptfile")

func includeFile(path string) bool {
	//fmt.Printf("includeFile matchResourceContents: %v path: %s\n", matchResourceContents, path)
	for _, m := range matchResourceContents {
		file := filepath.Base(path)
		if matched, err := filepath.Match(m, file); err == nil && matched {
			//fmt.Printf("includeFile match: %v\n", path)
			return true
		}
	}
	//fmt.Printf("includeFile no match: %v\n", path)
	return false
}
