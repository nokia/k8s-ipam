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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"
	"k8s.io/klog"
)

func main() {
	os.Exit(runMain())
}

// runMain does the initial setup in order to run kpt-gen. The return value from
// this function will be the exit code when kpt-gen terminates.
func runMain() int {
	var err error

	ctx := context.Background()

	// Enable commandline flags for klog.
	// logging will help in collecting debugging information from users
	klog.InitFlags(nil)

	cmd := getMain(ctx)

	err = cli.RunNoErrOutput(cmd)
	if err != nil {
		return handleErr(cmd, err)
	}
	return 0
}

// handleErr takes care of printing an error message for a given error.
func handleErr(cmd *cobra.Command, err error) int {
	fmt.Fprintf(cmd.ErrOrStderr(), "%s \n", err.Error())
	return 1
}
