package cmd

import (
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/cmd/server"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/cmd/trigger"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// NewCmd creates the new command
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tp",
		Short: "a tool to trigger pipelines in a Jenkins server",
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	cmd.AddCommand(common.SplitCommand(trigger.NewCmdTrigger()))
	cmd.AddCommand(server.NewCmdServer())
	return cmd
}
