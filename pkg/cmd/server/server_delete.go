package server

import (
	"fmt"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/factory"
	"github.com/jenkins-x/jx/v2/pkg/cmd/helper"
	"github.com/jenkins-x/jx/v2/pkg/cmd/templates"
	"github.com/jenkins-x/jx/v2/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteOptions contains the command line arguments for this command
type DeleteOptions struct {
	jenkinsutil.JenkinsOptions

	Name string
}

var (
	removeLong = templates.LongDesc(`
		This command removes a new Jenkins server from the registry of Jenkins servers

`)

	removeExample = templates.Examples(`
		# removes a Jenkins server from the registry by picking the server to remove
		%s remove

		# removes a specific named Jenkins server from the registry
		%s remove --name myserver
`)
)

// NewCmdDelete creates the new command
func NewCmdDelete() (*cobra.Command, *DeleteOptions) {
	o := &DeleteOptions{}
	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "removes a Jenkins server from the registry of Jenkins servers",
		Long:    removeLong,
		Example: fmt.Sprintf(removeExample, common.BinaryName, common.BinaryName),
		Aliases: []string{"rm", "remove"},
		Run: func(cmd *cobra.Command, args []string) {
			common.SetLoggingLevel(cmd)
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.Name, "name", "n", "", "the name of the Jenkins service to add")

	cmd.PersistentFlags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *DeleteOptions) Run() error {
	var err error
	if o.ClientFactory == nil {
		o.ClientFactory, err = factory.NewClientFactory()
		if err != nil {
			return err
		}
	}
	o.ClientFactory.Batch = o.BatchMode

	m, names, err := jenkinsutil.FindJenkinsServers(o.ClientFactory, nil)
	if err != nil {
		return err
	}

	name := o.Name
	if name != "" {
		jsvc := m[name]
		if jsvc == nil {
			return util.InvalidOption("name", name, names)
		}
	} else {
		if o.BatchMode {
			return util.MissingOption("name")
		}
		handles := common.GetIOFileHandles(o.IOFileHandles)
		name, err = util.PickName(names, "Jenkins server to delete:", "Select the name of the Jenkins server to remove from the registry", handles)
		if err != nil {
			return err
		}
	}
	jsvc := m[name]
	if jsvc == nil {
		return fmt.Errorf("could not find Jenkins service for: %s", name)
	}

	if jsvc.SecretName == "" {
		return fmt.Errorf("Jenkins service does not have a Secret name for: %s", name)
	}

	return o.deleteSecret(jsvc.SecretName)
}

func (o *DeleteOptions) deleteSecret(name string) error {
	kubeClient := o.ClientFactory.KubeClient
	ns := o.ClientFactory.Namespace

	err := kubeClient.CoreV1().Secrets(ns).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Logger().Warnf("the secret %s is not found", util.ColorInfo(name))
			return nil
		}
		return errors.Wrapf(err, "failed to delete Secret %s", name)
	}
	log.Logger().Infof("secret %s has been deleted", util.ColorInfo(name))
	return nil
}
