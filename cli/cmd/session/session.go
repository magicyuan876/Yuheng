// Package sessioncmd holds `yuheng session` command tree (list / view /
// delete / resume / stop) for chat history and knowledge-chat streams.
//
// Package name `sessioncmd` (not `session`) so callers can `import sdk
// "github.com/magicyuan876/yuheng/client"` and use `sdk.Session` without
// shadowing - same hygiene as `profilecmd`.
package sessioncmd

import (
	"github.com/spf13/cobra"

	"github.com/magicyuan876/yuheng/cli/internal/cmdutil"
)

// NewCmd builds the `yuheng session` parent command.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage chat sessions",
	}
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdDelete(f))
	cmd.AddCommand(NewCmdResume(f))
	cmd.AddCommand(NewCmdStop(f))
	return cmd
}
