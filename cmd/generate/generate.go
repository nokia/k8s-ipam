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

	invv1alpha1 "github.com/nokia/k8s-ipam/apis/inv/v1alpha1"
	topov1alpha1 "github.com/nokia/k8s-ipam/apis/topo/v1alpha1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	nodes := []*invv1alpha1.Node{}
	links := []*invv1alpha1.Link{}
	endpoints := []*invv1alpha1.Endpoint{}
	for nodeName, n := range topo.Spec.Nodes {
		labels := map[string]string{}
		for k, v := range n.Labels {
			labels[k] = v
		}
		labels[invv1alpha1.NephioTopologyKey] = topo.Name
		nodes = append(nodes, invv1alpha1.BuildNode(
			metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: topo.Namespace,
				Labels:    labels,
			},
			invv1alpha1.NodeSpec{
				UserDefinedLabels: n.UserDefinedLabels,
				Location:          n.Location,
				ParametersRef:     n.ParametersRef,
				Provider:          n.Provider,
			},
			invv1alpha1.NodeStatus{},
		))
	}
	for _, l := range topo.Spec.Links {
		eps := make([]invv1alpha1.LinkEndpoint, 0, 2)

		// define labels - use all the node labels
		labels := map[string]string{}
		labels[invv1alpha1.NephioTopologyKey] = topo.Name
		for k, v := range l.UserDefinedLabels.Labels {
			labels[k] = v
		}
		for _, e := range l.Endpoints {
			for k, v := range topo.Spec.Nodes[e.NodeName].Labels {
				labels[k] = v
			}
		}

		for _, e := range l.Endpoints {
			eps = append(eps, invv1alpha1.LinkEndpoint{
				NodeName:      e.NodeName,
				InterfaceName: e.InterfaceName,
			})

			// the endpoint provide is the node provider
			e.Provider = topo.Spec.Nodes[e.NodeName].Provider

			epLabels := map[string]string{}
			for k, v := range labels {
				epLabels[k] = v
			}
			epLabels[invv1alpha1.NephioProviderKey] = topo.Spec.Nodes[e.NodeName].Provider

			endpoints = append(endpoints, invv1alpha1.BuildEndpoint(
				metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", e.NodeName, e.InterfaceName),
					Namespace: topo.Namespace,
					Labels:    epLabels,
				},
				e,
				invv1alpha1.EndpointStatus{},
			))
		}
		linkName := fmt.Sprintf("%s-%s-%s-%s", eps[0].NodeName, eps[0].InterfaceName, eps[1].NodeName, eps[1].InterfaceName)

		links = append(links, invv1alpha1.BuildLink(
			metav1.ObjectMeta{
				Name:      linkName,
				Namespace: topo.Namespace,
				Labels:    labels,
			},
			invv1alpha1.LinkSpec{
				Endpoints: eps,
				LinkProperties: invv1alpha1.LinkProperties{
					LagMember:         l.LagMember,
					Lacp:              l.Lacp,
					Lag:               l.Lag,
					UserDefinedLabels: l.UserDefinedLabels,
					ParametersRef:     l.ParametersRef,
				},
			},
			invv1alpha1.LinkStatus{},
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
