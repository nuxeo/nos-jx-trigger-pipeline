package server

import (
	"fmt"
	"os"
	"sort"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/factory"
	"github.com/jenkins-x/jx/v2/pkg/cmd/helper"
	"github.com/jenkins-x/jx/v2/pkg/cmd/templates"
	"github.com/jenkins-x/jx/v2/pkg/table"
	"github.com/spf13/cobra"
)

// JobsOptions contains the command line arguments for this command
type JobsOptions struct {
	jenkinsutil.JenkinsOptions

	JenkinsSelector jenkinsutil.JenkinsSelectorOptions

	Filter string
}

var (
	jobsLong = templates.LongDesc(`
		This command lists the Jobs in a given Jenkins server

`)

	jobsExample = templates.Examples(`
		# list the jobs in a Jenkins server
		%s jobs
`)
)

// NewCmdJobs creates the new command
func NewCmdJobs() (*cobra.Command, *JobsOptions) {
	o := &JobsOptions{}
	cmd := &cobra.Command{
		Use:     "jobs",
		Short:   "lists the Jobs in a given Jenkins server",
		Long:    jobsLong,
		Example: fmt.Sprintf(jobsExample, common.BinaryName),
		Aliases: []string{"job"},
		Run: func(cmd *cobra.Command, args []string) {
			common.SetLoggingLevel(cmd)
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.Filter, "filter", "f", "", "filter string to filter the available jobs")
	o.JenkinsSelector.AddFlags(cmd)

	cmd.PersistentFlags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *JobsOptions) Run() error {
	var err error
	o.ClientFactory, err = factory.NewClientFactory()
	if err != nil {
		return err
	}
	o.ClientFactory.Batch = o.BatchMode
	o.ClientFactory.DevelopmentJenkinsURL = o.JenkinsSelector.DevelopmentJenkinsURL

	jobs, err := o.GetJenkinsJobs(&o.JenkinsSelector, o.Filter)
	if err != nil {
		return err
	}

	names := []string{}
	for k := range jobs {
		names = append(names, k)
	}
	sort.Strings(names)

	t := table.CreateTable(os.Stdout)
	t.AddRow("NAME")

	for _, name := range names {
		t.AddRow(name)
	}

	t.Render()
	return nil
}
