package server

import (
	"fmt"
	"os"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/factory"
	"github.com/jenkins-x/jx/v2/pkg/cmd/helper"
	"github.com/jenkins-x/jx/v2/pkg/cmd/templates"
	"github.com/jenkins-x/jx/v2/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/table"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/spf13/cobra"
)

// ListOptions contains the command line arguments for this command
type ListOptions struct {
	jenkinsutil.JenkinsOptions

	JenkinsSelector jenkinsutil.JenkinsSelectorOptions

	Results ListResults
}

// ListResults the results of the operation
type ListResults struct {
	Names   []string
	Servers map[string]*jenkinsutil.JenkinsServer
}

var (
	listLong = templates.LongDesc(`
		This command lists all the known Jenkins servers in the current namespace

`)

	listExample = templates.Examples(`
		# list the available jenkins servers in the current namespace
		%s list
`)
)

// NewCmdList creates the new command
func NewCmdList() (*cobra.Command, *ListOptions) {
	o := &ListOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "lists the Jenkins servers for the current namespace",
		Long:    listLong,
		Example: fmt.Sprintf(listExample, common.BinaryName),
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, args []string) {
			common.SetLoggingLevel(cmd)
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.JenkinsSelector.AddFlags(cmd)

	cmd.PersistentFlags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *ListOptions) Run() error {
	var err error
	if o.ClientFactory == nil {
		o.ClientFactory, err = factory.NewClientFactory()
		if err != nil {
			return err
		}
	}
	o.ClientFactory.Batch = o.BatchMode
	o.ClientFactory.DevelopmentJenkinsURL = o.JenkinsSelector.DevelopmentJenkinsURL

	m, names, err := jenkinsutil.FindJenkinsServers(o.ClientFactory, &o.JenkinsSelector)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		log.Logger().Infof("No Jenkins Servers could be found. Please try %s to register one\n", util.ColorInfo("tp server add"))
		return nil
	}

	o.Results.Names = names
	o.Results.Servers = m

	t := table.CreateTable(os.Stdout)
	t.AddRow("NAME", "URL")

	for _, name := range names {
		jsvc := m[name]
		if jsvc != nil {
			t.AddRow(name, jsvc.URL)
		}
	}

	t.Render()
	return nil
}
