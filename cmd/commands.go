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

package main

import (
	"context"

	"github.com/nokia/k8s-ipam/cmd/generate"
	"github.com/spf13/cobra"
)

// GetCommands returns the set of commands to be registered
func GetCommands(ctx context.Context, name, version string) []*cobra.Command {
	var c []*cobra.Command
	generateCmd := generate.NewCommand(ctx, name, version)

	c = append(c, generateCmd)
	return c
}
