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

package generate

import (
	"context"
	"fmt"
	"os"

	"github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// NewRunner returns a command runner.
func NewRunner(ctx context.Context, parent string) *Runner {
	r := &Runner{
		Ctx: ctx,
	}
	c := &cobra.Command{
		Use:  "generate",
		Args: cobra.MaximumNArgs(2),
		//Short:   docs.CloneShort,
		//Long:    docs.CloneShort + "\n" + docs.CloneLong,
		//Example: docs.CloneExamples,
		RunE: r.runE,
	}

	r.Command = c
	r.Command.Flags().StringVarP(
		&r.TopologyPath, "topology", "t", "", "path to the topology file")
	return r
}

func NewCommand(ctx context.Context, parent, version string) *cobra.Command {
	return NewRunner(ctx, parent).Command
}

type Runner struct {
	Command      *cobra.Command
	TopologyPath string
	Ctx          context.Context
}

func (r *Runner) runE(c *cobra.Command, args []string) error {

	b, err := os.ReadFile(r.TopologyPath)
	if err != nil {
		return errors.Wrap(err, "cannot read topology yaml file")
	}

	topo := topov1alpha1.RawTopology{}
	if err := yaml.Unmarshal(b, &topo); err != nil {
		return errors.Wrap(err, "cannot unmarshal topology yaml file")
	}

	if err := validate(topo); err != nil {
		return err
	}

	if err := generateTopoDetails(topo); err != nil {
		return err
	}

	return nil
}

func validate(topo topov1alpha1.RawTopology) error {
	if err := validateLink2Nodes(topo); err != nil {
		return err
	}

	if err := validateLinks(topo); err != nil {
		return err
	}
	return nil
}

func validateLink2Nodes(topo topov1alpha1.RawTopology) error {
	invalidNodeRef := []string{}
	for _, l := range topo.Spec.Links {
		for _, e := range l.Endpoints {
			epString := fmt.Sprintf("%s:%s", e.NodeName, e.InterfaceName)
			if _, ok := topo.Spec.Nodes[e.NodeName]; !ok {
				invalidNodeRef = append(invalidNodeRef, epString)
			}
		}
	}
	if len(invalidNodeRef) != 0 {
		return fmt.Errorf("endpoints %q has no node reference", invalidNodeRef)
	}
	return nil
}

func validateLinks(topo topov1alpha1.RawTopology) error {
	endpoints := map[string]struct{}{}
	// dups accumulates duplicate links
	dups := []string{}
	for _, l := range topo.Spec.Links {
		for _, e := range l.Endpoints {
			epString := fmt.Sprintf("%s:%s", e.NodeName, e.InterfaceName)
			if _, ok := endpoints[epString]; ok {
				dups = append(dups, epString)
			}
			endpoints[epString] = struct{}{}
		}
	}
	if len(dups) != 0 {
		return fmt.Errorf("endpoints %q appeared more than once in the links section of the topology file", dups)
	}
	return nil
}

func generateTopoDetails(topo topov1alpha1.RawTopology) error {
	nodes := []*v1alpha1.Node{}
	links := []*v1alpha1.Link{}
	endpoints := []*v1alpha1.Endpoint{}
	for nodeName, n := range topo.Spec.Nodes {
		labels := map[string]string{}
		for k, v := range n.UserDefinedLabels.Labels {
			labels[k] = v
		}
		labels[v1alpha1.NephioTopologyKey] = topo.Name
		nodes = append(nodes, v1alpha1.BuildNode(
			v1.ObjectMeta{
				Name:      nodeName,
				Namespace: topo.Namespace,
				Labels:    labels,
			},
			v1alpha1.NodeSpec{
				UserDefinedLabels: n.UserDefinedLabels,
				Location:          n.Location,
				ParametersRef:     n.ParametersRef,
				Provider:          n.Provider,
			},
			v1alpha1.NodeStatus{},
		))
	}
	for _, l := range topo.Spec.Links {
		epNames := make([]string, 0, 2)
		for _, e := range l.Endpoints {
			epName := fmt.Sprintf("%s-%s", e.NodeName, e.InterfaceName)
			epNames = append(epNames, epName)

			labels := map[string]string{}
			for k, v := range l.UserDefinedLabels.Labels {
				labels[k] = v
			}
			labels[v1alpha1.NephioTopologyKey] = topo.Name

			// the endpoint provide is the node provider
			e.Provider = topo.Spec.Nodes[e.NodeName].Provider

			endpoints = append(endpoints, v1alpha1.BuildEndpoint(
				v1.ObjectMeta{
					Name:      epName,
					Namespace: topo.Namespace,
					Labels:    labels,
				},
				e,
				v1alpha1.EndpointStatus{},
			))

		}
		linkName := fmt.Sprintf("%s-%s", epNames[0], epNames[1])

		labels := map[string]string{}
		for k, v := range l.UserDefinedLabels.Labels {
			labels[k] = v
		}
		labels[v1alpha1.NephioTopologyKey] = topo.Name
		links = append(links, v1alpha1.BuildLink(
			v1.ObjectMeta{
				Name:      linkName,
				Namespace: topo.Namespace,
				Labels:    labels,
			},
			v1alpha1.LinkSpec{
				Endpoints: epNames,
				LinkProperties: v1alpha1.LinkProperties{
					LagMember:         l.LagMember,
					Lacp:              l.Lacp,
					Lag:               l.Lag,
					UserDefinedLabels: l.UserDefinedLabels,
					ParametersRef:     l.ParametersRef,
				},
			},
			v1alpha1.LinkStatus{},
		))
	}

	for _, n := range nodes {
		b, err := yaml.Marshal(n)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	for _, l := range links {
		b, err := yaml.Marshal(l)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	for _, e := range endpoints {
		b, err := yaml.Marshal(e)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}
