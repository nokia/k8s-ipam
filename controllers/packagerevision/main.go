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

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	ipamv1alpha1 "github.com/henderiw-nephio/ipam/apis/ipam/v1alpha1"
	client "github.com/henderiw-nephio/ipam/pkg/alloc/allocclient"
	"github.com/henderiw-nephio/ipam/pkg/alloc/allocpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/yaml"
)

const (
	path = "/Users/henderiw/Documents/codeprojects/yndd/ipam/config/test"
)

func main() {
	ctx := context.Background()
	c, err := client.New(ctx, &client.Config{
		Address:  "127.0.0.1:9999",
		Insecure: true,
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fileInfo, err := os.Stat(path)
	m := []string{"*.yaml", "*.yml"}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var files []string
	if fileInfo.IsDir() {
		// this is a dir
		files, err = ReadFiles(path, true, m)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		// this is a single file
		files = []string{path}
	}

	inputs := []kio.Reader{}
	for _, path := range files {
		if includeFile(path, m) {
			yamlFile, err := ioutil.ReadFile(path)
			if err != nil {
				fmt.Printf("cannot read file: %s", path)
				os.Exit(1)
			}

			pathSplit := strings.Split(path, "/")
			if len(pathSplit) > 1 {
				path = filepath.Join(pathSplit[1:]...)
			}

			inputs = append(inputs, &kio.ByteReader{
				Reader: strings.NewReader(string(yamlFile)),
				SetAnnotations: map[string]string{
					kioutil.PathAnnotation: path,
				},
				DisableUnwrapping: true,
			})
		}
	}

	var pb kio.PackageBuffer
	err = kio.Pipeline{
		Inputs:  inputs,
		Filters: []kio.Filter{},
		Outputs: []kio.Writer{&pb},
	}.Execute()
	if err != nil {
		fmt.Printf("kio error: %s", err)
		os.Exit(1)
	}

	for _, rn := range pb.Nodes {

		if rn.GetApiVersion() == "ipam.nephio.org/v1alpha1" && rn.GetKind() == "IPAllocation" {
			//fmt.Printf("node: %v\n", rn.MustString())

			ipalloc := &ipamv1alpha1.IPAllocation{}
			if err := yaml.Unmarshal([]byte(rn.MustString()), ipalloc); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			//fmt.Printf("matchLabels: %v\n", ipalloc.Spec.Selector.MatchLabels)

			namespace := rn.GetNamespace()
			if namespace == "" {
				namespace = "default"
			}

			resp, err := c.AllocationRequest(ctx, &allocpb.Request{
				Namespace: namespace,
				Name:      rn.GetName(),
				Kind:      "ipam",
				Labels:    rn.GetLabels(),
				Spec: &allocpb.Spec{
					Selector: ipalloc.Spec.Selector.MatchLabels,
				},
			})
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			//fmt.Printf("response: prefix %s, parentprefix: %s", resp.GetPrefix(), resp.GetParentPrefix())

			ipalloc.Spec.ParentPrefix = resp.GetParentPrefix()
			ipalloc.Spec.Prefix = resp.GetPrefix()
			ipalloc.Status.ConditionedStatus.Conditions = append(ipalloc.Status.ConditionedStatus.Conditions, ipamv1alpha1.Condition{
				Kind:               ipamv1alpha1.ConditionKindReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			})

			b, err := yaml.Marshal(ipalloc)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Println("&&&&&&&&&&&&&&&")
			fmt.Println(string(b))
			fmt.Println("&&&&&&&&&&&&&&&")
		}

	}

	/*
		ctx := context.Background()
			c, err := client.New(ctx, &client.Config{
				Address:  "127.0.0.1:9999",
				Insecure: true,
			})
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			resp, err := c.AllocationRequest(ctx, &allocpb.Request{
				Namespace: "default",
				Name:      "grpcAlloc1",
				Kind:      "ipam",
				Labels: map[string]string{
					"grpc-client": "test",
				},
				Spec: &allocpb.Spec{
					Selector: map[string]string{
						"nephio.org/network-instance": "network-1",
						"nephio.org/prefix-name":      "network1-prefix1",
					},
				},
			})
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			fmt.Printf("response: prefix %s, parentprefix: %s", resp.GetPrefix(), resp.GetParentPrefix())
	*/

}

func ReadFiles(source string, recursive bool, match []string) ([]string, error) {
	filePaths := make([]string, 0)
	if recursive {
		err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// check for the file extension
			if includeFile(filepath.Ext(info.Name()), match) {
				filePaths = append(filePaths, path)
			} else {
				// check for exact file match
				if includeFile(info.Name(), match) {
					filePaths = append(filePaths, path)
				}
			}
			return nil
		})
		if err != nil {
			return filePaths, err
		}
	} else {
		if includeFile(filepath.Ext(source), match) {
			filePaths = append(filePaths, source)
		} else {
			files, err := os.ReadDir(source)
			if err != nil {
				return filePaths, err
			}
			for _, info := range files {
				if includeFile(filepath.Ext(info.Name()), match) {
					path := filepath.Join(source, info.Name())
					filePaths = append(filePaths, path)
				}
			}
		}
	}
	return filePaths, nil
}
func includeFile(path string, match []string) bool {
	for _, m := range match {
		file := filepath.Base(path)
		if matched, err := filepath.Match(m, file); err == nil && matched {
			return true
		}
	}
	return false
}
