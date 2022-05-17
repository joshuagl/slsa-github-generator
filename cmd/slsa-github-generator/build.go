// Copyright 2022 SLSA Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "build",
		Short: "Run the passed sequence of commands",
		Long: `Execute the passed sequence of commands. Separate commands with
a semi-colon (;). This command assumes that it is being run in the context of a
Github Actions workflow.`,
		Args: cobra.MinimumNArgs(1),

		// TODO: disabling flag parsing is unexpected, but simplifies the CLI
		// for if users are not familiar with UNIX convention of using `--` to
		// separate flags from arguments.
		// DisableFlagParsing: true,

		Run: func(cmd *cobra.Command, args []string) {
			for _, arg := range args {
				// User may supply numerous commands, separated by a ';', which
				// should be run separately.
				for _, cmdln := range strings.Split(arg, ";") {
					cmdln := strings.Trim(cmdln, " ")
					cmdlst := strings.Split(strings.Trim(cmdln, " "), " ")

					cmd := exec.Command(cmdlst[0], cmdlst[1:]...)
					cmd.Env = os.Environ()
					// TODO: for long-running build processes users will
					// appreciate buffering output, rather than dumping it all
					// at once when the command execution has completed.
					out, err := cmd.Output()
					fmt.Println(string(out))
					check(err)
				}
			}
		},
	}
	return c
}
