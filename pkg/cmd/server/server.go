package server

import (
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// NewCmdServer creates the new command
func NewCmdServer() *cobra.Command {
	command := &cobra.Command{
		Use:     "server",
		Short:   "commands for working with Jenkins servers",
		Aliases: []string{"servers", "jenkins"},
		Run: func(command *cobra.Command, args []string) {
			err := command.Help()
			if err != nil {
				log.Logger().Errorf(err.Error())
			}
		},
	}
	command.AddCommand(common.SplitCommand(NewCmdAdd()))
	command.AddCommand(common.SplitCommand(NewCmdDelete()))
	command.AddCommand(common.SplitCommand(NewCmdList()))
	return command
}
