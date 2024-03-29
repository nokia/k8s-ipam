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
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kustomize/kyaml/commandutil"
)

var pgr []string

func getMain(ctx context.Context) *cobra.Command {
	os.Setenv(commandutil.EnableAlphaCommmandsEnvName, "true")
	cmd := &cobra.Command{
		Use: "topogen",
		//Short:        overview.CliShort,
		//Long:         overview.CliLong,
		SilenceUsage: true,
		// We handle all errors in main after return from cobra so we can
		// adjust the error message coming from libraries
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := cmd.Flags().GetBool("help")
			if err != nil {
				return err
			}
			if h {
				return cmd.Help()
			}
			return cmd.Usage()
		},
	}

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// help and documentation
	cmd.InitDefaultHelpCmd()
	cmd.AddCommand(GetCommands(ctx, "topogen", version)...)

	// enable stack traces
	//cmd.PersistentFlags().BoolVar(&cmdutil.StackOnError, "stack-trace", false,
	//	"Print a stack-trace on failure")

	replace(cmd)

	cmd.AddCommand(versionCmd)
	hideFlags(cmd)
	return cmd
}

func replace(c *cobra.Command) {
	for i := range c.Commands() {
		replace(c.Commands()[i])
	}
	c.SetHelpFunc(newHelp(pgr, c))
}

func newHelp(e []string, c *cobra.Command) func(command *cobra.Command, strings []string) {
	if len(pgr) == 0 {
		return c.HelpFunc()
	}

	fn := c.HelpFunc()
	return func(command *cobra.Command, args []string) {
		stty := exec.Command("stty", "size")
		stty.Stdin = os.Stdin
		out, err := stty.Output()
		if err == nil {
			terminalHeight, err := strconv.Atoi(strings.Split(string(out), " ")[0])
			helpHeight := strings.Count(command.Long, "\n") +
				strings.Count(command.UsageString(), "\n")
			if err == nil && terminalHeight > helpHeight {
				// don't use a pager if the help is shorter than the console
				fn(command, args)
				return
			}
		}

		b := &bytes.Buffer{}
		pager := exec.Command(e[0])
		if len(e) > 1 {
			pager.Args = append(pager.Args, e[1:]...)
		}
		pager.Stdin = b
		pager.Stdout = c.OutOrStdout()
		c.SetOut(b)
		fn(command, args)
		if err := pager.Run(); err != nil {
			fmt.Fprintf(c.ErrOrStderr(), "%v", err)
			os.Exit(1)
		}
	}
}

var version = "unknown"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of kptgen",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", version)
	},
}

// hideFlags hides any cobra flags that are unlikely to be used by
// customers.
func hideFlags(cmd *cobra.Command) {
	flags := []string{
		// Flags related to logging
		"add_dir_header",
		"alsologtostderr",
		"log_backtrace_at",
		"log_dir",
		"log_file",
		"log_file_max_size",
		"logtostderr",
		"one_output",
		"skip_headers",
		"skip_log_headers",
		"stack-trace",
		"stderrthreshold",
		"vmodule",

		// Flags related to apiserver
		"as",
		"as-group",
		"cache-dir",
		"certificate-authority",
		"client-certificate",
		"client-key",
		"insecure-skip-tls-verify",
		"match-server-version",
		"password",
		"token",
		"username",
	}
	for _, f := range flags {
		_ = cmd.PersistentFlags().MarkHidden(f)
	}

	// We need to recurse into subcommands otherwise flags aren't hidden on leaf commands
	for _, child := range cmd.Commands() {
		hideFlags(child)
	}
}
