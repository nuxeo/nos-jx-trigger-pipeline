package trigger

import (
	"fmt"
	"net/url"
	"os"
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
	Tail               bool
	Cancel             bool
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
	cmd.Flags().BoolVarP(&o.Tail, "tail", "t", false, "Tails the build log to the current terminal")
	cmd.Flags().BoolVarP(&o.Cancel, "cancel", "", false, "Cancel last build")
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
		if o.Branch == "" {
			o.Branch = "master"
		}
	}

	if o.JenkinsPath == "" {
		o.JenkinsPath = fmt.Sprintf("%s/%s/%s", gitInfo.Organisation, gitInfo.Name, o.Branch)
	}

	return o.TriggerPipeline(jenkinsClient, gitInfo)
}

// TriggerPipeline trigger a build based on the current git workspace
func (o *TriggerOptions) TriggerPipeline(jenkinsClient gojenkins.JenkinsClient, gitInfo *gits.GitRepository) error {
	job, err := o.getOrCreatePipelineFactory(jenkinsClient, gitInfo)()
	if err != nil {
		return errors.Wrapf(err, "cannot create pipeline for %s", job.FullName)
	}

	if o.Cancel {
		return o.cancelLastBuild(jenkinsClient, job, time.Minute*5)
	}

	build, err := o.triggerAndWaitForBuildToStart(jenkinsClient, job, time.Minute*5)
	if err != nil {
		return errors.Wrapf(err, "cannot trigger build for %s", job.FullName)
	}
	if !o.Tail {
		return err
	}

	err = o.JenkinsOptions.TailJenkinsBuildLog(&o.JenkinsSelector, job.FullName, &build)
	if err != nil {
		return errors.Wrapf(err, "cannot tail build for %s/%d", job.FullName, build.Number)
	}

	build, err = jenkinsClient.GetBuild(job, build.Number)
	if err != nil {
		return errors.Wrapf(err, "cannot state build for %s", job.FullName)
	}
	if build.Result != "Success" {
		message := fmt.Sprintf("build %s/%d result is %s", job.FullName, build.Number, build.Result)
		log.Logger().Info(message)
		err = errors.New(message)
	}
	return err
}

type PipelineFactory func() (gojenkins.Job, error)

func (o *TriggerOptions) getOrCreatePipelineFactory(jenkinsClient gojenkins.JenkinsClient, gitInfo *gits.GitRepository) PipelineFactory {
	if o.MultiBranchProject {
		return func() (gojenkins.Job, error) {
			return o.getOrCreateMultiBranchPipeline(jenkinsClient, gitInfo)
		}
	}
	return func() (gojenkins.Job, error) {
		return o.getOrCreateStandalonePipeline(jenkinsClient, gitInfo)
	}
}

func (o *TriggerOptions) getOrCreateStandalonePipeline(jenkinsClient gojenkins.JenkinsClient, gitInfo *gits.GitRepository) (gojenkins.Job, error) {
	var job gojenkins.Job
	var err error

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
				return job, err
			}
		} else {
			gitURL := gitInfo.URL
			log.Logger().Infof("Using git URL %s and branch %s", util.ColorInfo(gitURL), util.ColorInfo(o.Branch))

			err = helpers.Retry(3, time.Second*10, func() error {
				if err != nil {
					pipelineXML := jenkins.CreatePipelineXML(gitURL, o.Branch, o.Jenkinsfile)
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
				return job, err
			}

			job, err = jenkinsClient.GetJobByPath(paths...)
			if err != nil {
				return job, err
			}
		}
	}
	return job, err
}

func (o *TriggerOptions) getOrCreateMultiBranchPipeline(jenkinsClient gojenkins.JenkinsClient, gitInfo *gits.GitRepository) (gojenkins.Job, error) {
	job, err := jenkinsClient.GetMultiBranchJob(gitInfo.Organisation, gitInfo.Name, o.Branch)
	if err == nil {
		return job, err
	}
	job, err = jenkinsClient.GetJobByPath(gitInfo.Organisation, gitInfo.Name)
	if err != nil {
		return job, err
	}
	job.Url = jenkins.SwitchJenkinsBaseURL(job.Url, jenkinsClient.BaseURL())
	log.Logger().Infof("scanning multibranch project %s", job.FullName)
	err = jenkinsClient.Build(job, url.Values{})
	if err != nil {
		return job, errors.Wrapf(err, "failed to scan multibranch project %s", job.FullName)
	}
	log.Logger().Infof("waiting for job creation of %s/%s", job.FullName, o.Branch)
	pollJobCB := func() (bool, error) {
		job, err = jenkinsClient.GetMultiBranchJob(gitInfo.Organisation, gitInfo.Name, o.Branch)
		if err == nil {
			return true, nil
		}
		if is404(err) {
			return false, nil
		}
		return false, err
	}
	err = gojenkins.Poll(1*time.Second, 60*time.Second, fmt.Sprintf("poll for job %s", job.FullName), pollJobCB)
	if err != nil {
		return job, err
	}

	log.Logger().Infof("waiting for build cancellation %s", job.Url)
	var build gojenkins.Build
	err = gojenkins.Poll(1*time.Second, 60*time.Second, fmt.Sprintf("poll for build of %s", job.FullName), func() (bool, error) {
		build, err = jenkinsClient.GetLastBuild(job)
		if err != nil {
			if is404(err) {
				return false, nil
			}
			return false, err
		}
		if build.Number != 1 {
			return false, errors.New(fmt.Sprintf("not first build %s/%d", job.FullName, build.Number))
		}
		return true, nil
	})
	if err != nil {
		return job, err
	}
	err = o.cancelLastBuild(jenkinsClient, job, 5*time.Minute)
	return job, err
}

func (o *TriggerOptions) cancelLastBuild(jenkins gojenkins.JenkinsClient, job gojenkins.Job, waitTime time.Duration) error {
	build, err := jenkins.GetLastBuild(job)
	if err != nil {
		return err
	}
	if !build.Building {
		return nil
	}
	log.Logger().Infof("cancelling build %s/%d", job.FullName, build.Number)
	err = jenkins.StopBuild(job, build.Number)
	if err != nil {
		return err
	}
	err = gojenkins.Poll(1*time.Second, 60*time.Second, fmt.Sprintf("poll for cancel of %s/%d", job.FullName, build.Number),
		func() (bool, error) {
			build, err = jenkins.GetLastBuild(job)
			if err != nil {
				return false, err
			}
			if build.Building {
				return false, nil
			}
			return true, nil
		})
	return err
}

func (o *TriggerOptions) triggerAndWaitForBuildToStart(jenkins gojenkins.JenkinsClient, job gojenkins.Job, buildStartWaitTime time.Duration) (gojenkins.Build, error) {
	var build gojenkins.Build
	var err error
	previousBuildNumber := 0
	previousBuild, err := jenkins.GetLastBuild(job)
	if err != nil {
		if !is404(err) {
			return build, errors.Wrapf(err, "error finding last build for %s due to %v", job.FullName)
		}
	} else {
		previousBuildNumber = previousBuild.Number
	}
	err = jenkins.Build(job, nil)
	if err != nil {
		if !is404(err) {
			return build, errors.Wrapf(err, "error triggering build %s due to %v", job.FullName)
		}
	}
	// lets wait for a new build to start
	fn := func() (bool, error) {
		buildNumber := 0
		build, err = jenkins.GetLastBuild(job)
		if err != nil {
			if !is404(err) {
				return false, errors.Wrapf(err, "error finding last build for %s due to %v", job.FullName)
			}
		} else {
			buildNumber = build.Number
		}
		if previousBuildNumber != buildNumber {
			log.Logger().Infof("triggered job %s build #%d\n", job.FullName, buildNumber)
			return true, nil
		}
		return false, nil
	}
	err = gojenkins.Poll(1*time.Second, buildStartWaitTime, fmt.Sprintf("build to start for for %s", job.FullName), fn)
	return build, err
}

func is404(err error) bool {
	text := fmt.Sprintf("%s", err)
	return strings.HasPrefix(text, "404 ")
}
