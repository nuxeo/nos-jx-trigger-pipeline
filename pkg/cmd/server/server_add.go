package server

import (
	"fmt"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/factory"
	"github.com/jenkins-x/jx/v2/pkg/cmd/helper"
	"github.com/jenkins-x/jx/v2/pkg/cmd/templates"
	"github.com/jenkins-x/jx/v2/pkg/kube/naming"
	"github.com/jenkins-x/jx/v2/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddOptions contains the command line arguments for this command
type AddOptions struct {
	jenkinsutil.JenkinsOptions

	JenkinsService jenkinsutil.JenkinsServer
}

var (
	addLong = templates.LongDesc(`
		This command adds a new Jenkins server to the registry of Jenkins servers

`)

	addExample = templates.Examples(`
		# adds a new Jenkins server to the registry of Jenkins servers so it can be used to trigger pipelines
		%s add
`)
)

// NewCmdAdd creates the new command
func NewCmdAdd() (*cobra.Command, *AddOptions) {
	o := &AddOptions{}
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "adds a new Jenkins server to the registry of Jenkins servers",
		Long:    addLong,
		Example: fmt.Sprintf(addExample, common.BinaryName),
		Aliases: []string{"create", "new"},
		Run: func(cmd *cobra.Command, args []string) {
			common.SetLoggingLevel(cmd)
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.JenkinsService.Name, "name", "n", "", "the name of the Jenkins service to add")
	cmd.Flags().StringVarP(&o.JenkinsService.URL, "url", "u", "", "the URL to use to invoke the Jenkins service")
	cmd.Flags().StringVarP(&o.JenkinsService.Auth.Username, "username", "r", "", "the username to use to invoke the Jenkins service")
	cmd.Flags().StringVarP(&o.JenkinsService.Auth.ApiToken, "token", "t", "", "the API token to use to invoke the Jenkins service")

	cmd.PersistentFlags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *AddOptions) Run() error {
	var err error
	if o.ClientFactory == nil {
		o.ClientFactory, err = factory.NewClientFactory()
		if err != nil {
			return err
		}
	}
	o.ClientFactory.Batch = o.BatchMode

	j := &o.JenkinsService

	err = o.populateJenkinsService(j)
	if err != nil {
		return err
	}
	return o.createJenkinsService(j)
}

func (o *AddOptions) populateJenkinsService(j *jenkinsutil.JenkinsServer) error {
	if o.BatchMode {
		if j.Name == "" {
			return util.MissingOption("name")
		}
		if j.URL == "" {
			return util.MissingOption("url")
		}
		if j.Auth.Username == "" {
			return util.MissingOption("username")
		}
		if j.Auth.ApiToken == "" {
			return util.MissingOption("token")
		}
		return nil
	}
	var err error
	handles := common.GetIOFileHandles(o.IOFileHandles)
	if j.Name == "" {
		j.Name, err = util.PickValue("name of the Jenkins server:", "", true,
			"each Jenkins server needs a unique name", handles)
		if err != nil {
			return err
		}
		j.Name = naming.ToValidName(j.Name)
	}
	if j.URL == "" {
		j.URL, err = util.PickValue("HTTP/HTTPS URL of the Jenkins server:", "", true,
			"we need the HTTP API URL so we can interact with the remote Jenkins", handles)
		if err != nil {
			return err
		}
	}
	if j.Auth.Username == "" {
		j.Auth.Username, err = util.PickValue("user name used to access Jenkins:", "admin", true,
			"we need the username to be used when accessing the remote Jenkins", handles)
		if err != nil {
			return err
		}
	}
	if j.Auth.ApiToken == "" {
		tokenURL := jenkinsTokenURL(j.URL)
		log.Logger().Infof("\nPlease go to %s to generate the API token:\n", util.ColorInfo(tokenURL))
		log.Logger().Infof("click the %s button\n", util.ColorInfo("Add new Token"))
		log.Logger().Infof("click the %s button\n", util.ColorInfo("Generate"))
		log.Logger().Infof("Then COPY the token and enter in into the form below:\n\n")

		j.Auth.ApiToken, err = util.PickPassword("API token used to access Jenkins:",
			"we need the API token to be used when accessing the remote Jenkins", handles)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *AddOptions) createJenkinsService(j *jenkinsutil.JenkinsServer) error {
	j.Name = naming.ToValidName(j.Name)
	secretsName := "tp-" + j.Name

	kubeClient := o.ClientFactory.KubeClient
	ns := o.ClientFactory.Namespace
	secretInterface := kubeClient.CoreV1().Secrets(ns)

	secret, err := secretInterface.Get(secretsName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to load Secret %s in namespace %s", secretsName, ns)
	}
	if secret == nil {
		secret = &corev1.Secret{}
	}
	secret.Name = secretsName
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Labels[common.RegistryLabel] = common.RegistryLabelValue
	secret.Labels[common.JenkinsNameLabel] = j.Name
	secret.Annotations[common.JenkinsURLAnnotation] = j.URL

	secret.Data[common.SecretKeyUser] = []byte(j.Auth.Username)
	secret.Data[common.SecretKeyToken] = []byte(j.Auth.ApiToken)

	if secret.ResourceVersion != "" {
		_, err = secretInterface.Update(secret)
		if err != nil {
			return errors.Wrapf(err, "failed to update Secret %s in namespace %s", secretsName, ns)
		}
	} else {
		_, err = secretInterface.Create(secret)
		if err != nil {
			return errors.Wrapf(err, "failed to create Secret %s in namespace %s", secretsName, ns)
		}
	}
	log.Logger().Infof("saved Jenkins server into Secret %s", util.ColorInfo(secretsName))
	return nil
}

func jenkinsTokenURL(url string) string {
	return util.UrlJoin(url, "/me/configure")
}
