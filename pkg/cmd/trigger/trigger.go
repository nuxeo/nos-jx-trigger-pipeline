package trigger

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/helpers"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/factory"
	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/v2/pkg/cmd/helper"
	"github.com/jenkins-x/jx/v2/pkg/gits"

	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/cmd/templates"
	"github.com/jenkins-x/jx/v2/pkg/jenkins"
	"github.com/jenkins-x/jx/v2/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// TriggerOptions contains the command line arguments for this command
type TriggerOptions struct {
	jenkinsutil.JenkinsOptions

	MultiBranchProject bool
	Dir                string
	Jenkinsfile        string
	JenkinsPath        string
	JenkinsSelector    jenkinsutil.JenkinsSelectorOptions
	Branch             string
}

var (
	triggerLong = templates.LongDesc(`
		This command triggers the Jenkinsfile in the current directory in a Jenkins server

`)

	triggerExample = templates.Examples(`
		# triggers the Jenkinsfile in the current directory in a Jenkins server 
		%s
`)
)

// NewCmdTrigger creates the new command
func NewCmdTrigger() (*cobra.Command, *TriggerOptions) {
	o := &TriggerOptions{}
	cmd := &cobra.Command{
		Use:     "trigger",
		Short:   "triggers the Jenkinsfile in the current directory in a Jenkins server installed via the Jenkins Operator",
		Long:    triggerLong,
		Example: fmt.Sprintf(triggerExample, common.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			common.SetLoggingLevel(cmd)
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&o.MultiBranchProject, "multi-branch-project", "", false, "Use a Multi Branch Project in Jenkins")
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", ".", "the directory to look for the Jenkisnfile inside")
	cmd.Flags().StringVarP(&o.Jenkinsfile, "jenkinsfile", "f", jenkinsfile.Name, "The name of the Jenkinsfile to use")
	cmd.Flags().StringVarP(&o.JenkinsPath, "jenkins-path", "p", "", "The Jenkins folder path to create the pipeline inside. If not specified it defaults to the git 'owner/repoName/branch'")
	cmd.Flags().StringVarP(&o.JenkinsSelector.DevelopmentJenkinsURL, "dev-jenkins-url", "", "", "Specifies a local URL to access the jenkins server if you are not running this command inside a Kubernetes cluster and don't have Ingress resosurces for the Jenkins server and so cannot use Kubernetes Service discovery. E.g. could be 'http://localhost:8080' if you are using: kubectl port-forward jenkins-server1 8080:8080")
	cmd.Flags().StringVarP(&o.Branch, "branch", "", "", "the branch to trigger a build")
	o.JenkinsSelector.AddFlags(cmd)

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	cmd.PersistentFlags().BoolVarP(&o.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")
	return cmd, o
}

// Run implements the command
func (o *TriggerOptions) Run() error {
	var err error
	o.ClientFactory, err = factory.NewClientFactory()
	if err != nil {
		return err
	}
	o.ClientFactory.Batch = o.BatchMode
	o.ClientFactory.DevelopmentJenkinsURL = o.JenkinsSelector.DevelopmentJenkinsURL

	jenkinsClient, err := o.CreateJenkinsClientFromSelector(&o.JenkinsSelector)
	if err != nil {
		return err
	}

	if o.Dir == "" {
		o.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if o.Jenkinsfile == "" {
		o.Jenkinsfile = jenkinsfile.Name
	}
	gitInfo, err := o.FindGitInfo(o.Dir)
	if err != nil {
		return err
	}

	if o.Branch == "" {
		o.Branch, err = o.Git().Branch(o.Dir)
		if err != nil {
			return err
		}
	}

	if o.Branch == "" {
		o.Branch = "master"
	}

	return o.TriggerPipeline(jenkinsClient, gitInfo, o.Branch)
}

// TriggerPipeline triggers the pipeline after the main service clients are created
func (o *TriggerOptions) TriggerPipeline(jenkinsClient gojenkins.JenkinsClient, gitInfo *gits.GitRepository, branch string) error {
	jenkinsfileName := filepath.Join(o.Dir, o.Jenkinsfile)
	exists, err := util.FileExists(jenkinsfileName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s does not exist", jenkinsfileName)
	}

	if o.JenkinsPath == "" {
		o.JenkinsPath = fmt.Sprintf("%s/%s/%s", gitInfo.Organisation, gitInfo.Name, branch)
	}

	paths := strings.Split(o.JenkinsPath, "/")
	last := len(paths) - 1
	for i, path := range paths {
		folderPath := paths[0 : i+1]
		folder, err := jenkinsClient.GetJobByPath(folderPath...)
		fullPath := util.UrlJoin(folderPath...)
		jobURL := util.UrlJoin(jenkinsClient.BaseURL(), fullPath)

		if i < last {
			// lets ensure there's a folder
			err = helpers.Retry(3, time.Second*10, func() error {
				if err != nil {
					folderXML := jenkins.CreateFolderXML(jobURL, path)
					if i == 0 {
						err = jenkinsClient.CreateJobWithXML(folderXML, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s folder at %s in Jenkins", path, jobURL)
						}
					} else {
						folders := strings.Join(paths[0:i], "/job/")
						err = jenkinsClient.CreateFolderJobWithXML(folderXML, folders, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s folder in folders %s at %s in Jenkins", path, folders, jobURL)
						}
					}
				} else {
					c := folder.Class
					if c != "com.cloudbees.hudson.plugins.folder.Folder" {
						log.Logger().Warnf("Warning the folder %s is of class %s", jobURL, c)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			gitURL := gitInfo.CloneURL
			log.Logger().Infof("Using git URL %s and branch %s", util.ColorInfo(gitURL), util.ColorInfo(branch))

			err = helpers.Retry(3, time.Second*10, func() error {
				if err != nil {
					pipelineXML := jenkins.CreatePipelineXML(gitURL, branch, o.Jenkinsfile)
					if i == 0 {
						err = jenkinsClient.CreateJobWithXML(pipelineXML, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s pipeline at %s in Jenkins", path, jobURL)
						}
					} else {
						folders := strings.Join(paths[0:i], "/job/")
						err = jenkinsClient.CreateFolderJobWithXML(pipelineXML, folders, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s pipeline in folders %s at %s in Jenkins", path, folders, jobURL)
						}
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			job, err := jenkinsClient.GetJobByPath(paths...)
			if err != nil {
				return err
			}
			job.Url = jenkins.SwitchJenkinsBaseURL(job.Url, jenkinsClient.BaseURL())
			jobPath := strings.Join(paths, "/")
			log.Logger().Infof("triggering pipeline job %s", util.ColorInfo(jobPath))
			err = jenkinsClient.Build(job, url.Values{})
			if err != nil {
				return errors.Wrapf(err, "failed to trigger job %s", jobPath)
			}
		}
	}
	return nil
}
