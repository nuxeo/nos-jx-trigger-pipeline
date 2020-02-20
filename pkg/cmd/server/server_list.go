package server

import (
	"os"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/factory"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/table"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// ListOptions contains the command line arguments for this command
type ListOptions struct {
	jenkinsutil.JenkinsOptions

	JenkinsSelector jenkinsutil.JenkinsSelectorOptions
}

var (
	stepCustomPipelineLong = templates.LongDesc(`
		This command lists all the known Jenkins servers in the current namespace

`)

	stepCustomPipelineExample = templates.Examples(`
		# list the available jenkins servers in the current namespace
		tp server list
`)
)

// NewCmdList creates the new command
func NewCmdList() (*cobra.Command, *ListOptions) {
	o := &ListOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "lists the Jenkins servers for the current namespace",
		Long:    stepCustomPipelineLong,
		Example: stepCustomPipelineExample,
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, args []string) {
			common.SetLoggingLevel(cmd)
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.JenkinsSelector.AddFlags(cmd)

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	cmd.PersistentFlags().BoolVarP(&o.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *ListOptions) Run() error {
	var err error
	o.ClientFactory, err = factory.NewClientFactory()
	if err != nil {
		return err
	}
	o.ClientFactory.Batch = o.BatchMode
	o.ClientFactory.DevelopmentJenkinsURL = o.JenkinsSelector.DevelopmentJenkinsURL

	names, err := o.GetJenkinsServiceNames(&o.JenkinsSelector)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		log.Logger().Infof("No Jenkins Servers could be found. Please try %s to register one\n", util.ColorInfo("tp server add"))
		return nil
	}

	t := table.CreateTable(os.Stdout)
	t.AddRow("NAME")

	for _, name := range names {
		t.AddRow(name)
	}

	t.Render()
	return nil
}
