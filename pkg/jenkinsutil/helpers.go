package jenkinsutil

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/gits"
	jxjenkins "github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type JenkinsOptions struct {
	ClientFactory *ClientFactory

	BatchMode     bool
	IOFileHandles *util.IOFileHandles
	git           gits.Gitter
	jenkinsClient gojenkins.JenkinsClient
}

// JenkinsSelectorOptions used to represent the options used to refer to a Jenkins.
// if nothing is specified it assumes the current team is using a static Jenkins server as its execution engine.
// otherwise we can refer to other additional Jenkins Apps to implement custom Jenkins servers
type JenkinsSelectorOptions struct {
	// JenkinsName the name of the Jenkins Operator Service for HTTP to use
	JenkinsName string

	// Selector label selector to find the Jenkins Operator Services
	Selector string

	// DevelopmentJenkinsURL a local URL to use to talk to the jenkins server if the servers do not have Ingress
	// and you want to test out using the jenkins client locally
	DevelopmentJenkinsURL string

	// cached client
	cachedCustomJenkinsClient gojenkins.JenkinsClient
}

// AddFlags add the command flags for picking a custom Jenkins App to work with
func (o *JenkinsSelectorOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.JenkinsName, "jenkins", "", "", "The name of the Jenkin server provisioned by the Jenkins Operator")
	cmd.Flags().StringVarP(&o.Selector, "selector", "", "app=jenkins-operator", "The kubernetes label selector to find the Jenkins Operator Services for Jenkins HTTP servers")
}

// GetAllPipelineJobNames returns all the pipeline job names
func (o *JenkinsOptions) GetAllPipelineJobNames(jenkinsClient gojenkins.JenkinsClient, jobNames *[]string, jobName string) error {
	job, err := jenkinsClient.GetJob(jobName)
	if err != nil {
		return err
	}
	if len(job.Jobs) == 0 {
		*jobNames = append(*jobNames, job.FullName)
	}
	for _, j := range job.Jobs {
		err = o.GetAllPipelineJobNames(jenkinsClient, jobNames, job.FullName+"/"+j.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetJenkinsClient sets the JenkinsClient - usually used in testing
func (o *JenkinsOptions) SetJenkinsClient(jenkinsClient gojenkins.JenkinsClient) {
	o.jenkinsClient = jenkinsClient
}

// CustomJenkinsClient returns the Jenkins client for the custom jenkins app
func (o *JenkinsOptions) CustomJenkinsClient(jenkinsServiceName string) (gojenkins.JenkinsClient, error) {
	return o.ClientFactory.CreateJenkinsClient(jenkinsServiceName)
}

// CreateJenkinsClientFromSelector is given either a specific jenkins service name to use, uses the selector to find it
// or prompts the user to pick one if not in batch mode.
func (o *JenkinsOptions) CreateJenkinsClientFromSelector(jenkinsSelector *JenkinsSelectorOptions) (gojenkins.JenkinsClient, error) {
	var err error
	jenkinsServiceName, err := o.PickCustomJenkinsName(jenkinsSelector, true)
	if err != nil {
		return nil, err
	}
	jenkinsClient, err := o.CustomJenkinsClient(jenkinsServiceName)
	if err == nil {
		jenkinsSelector.cachedCustomJenkinsClient = jenkinsClient
	}
	return jenkinsClient, err
}

// JenkinsURLForSelector returns the default or the custom Jenkins URL
func (o *JenkinsOptions) JenkinsURLForSelector(jenkinsSelector *JenkinsSelectorOptions, kubeClient kubernetes.Interface, ns string) (string, error) {
	var err error
	jenkinsServiceName, err := o.PickCustomJenkinsName(jenkinsSelector, true)
	if err != nil {
		return "", err
	}
	return o.ClientFactory.JenkinsURL(jenkinsServiceName)
}

// PickCustomJenkinsName picks the name of a custom jenkins server App if available
func (o *JenkinsOptions) PickCustomJenkinsName(jenkinsSelector *JenkinsSelectorOptions, failIfNone bool) (string, error) {
	customJenkinsName := jenkinsSelector.JenkinsName
	if customJenkinsName == "" {
		names, err := o.GetJenkinsServiceNames(jenkinsSelector)
		if err != nil {
			return "", err
		}

		if o.BatchMode {
			name := os.Getenv(TriggerJenkinsServerEnv)
			if name != "" {
				if util.StringArrayIndex(names, name) < 0 {
					return "", fmt.Errorf("the $%s is %s but we can only find these Jenkins servers: %s", TriggerJenkinsServerEnv, name, strings.Join(names, ", "))
				}
				log.Logger().Infof("defaulting to Jenkins server %s due to $%s", util.ColorInfo(name), TriggerJenkinsServerEnv)
				return name, nil
			}
		}

		switch len(names) {
		case 0:
			if failIfNone {
				return "", fmt.Errorf("No Jenkins services found")
			}
			return "", nil

		case 1:
			customJenkinsName = names[0]

		default:
			if o.BatchMode {
				return "", util.MissingOptionWithOptions("jenkins", names)
			}
			customJenkinsName, err = util.PickName(names, "Pick which custom Jenkins App you wish to use: ", "Jenkins Apps are a way to add custom Jenkins servers into Jenkins X", o.GetIOFileHandles())
			if err != nil {
				return "", err
			}
		}
	}
	jenkinsSelector.JenkinsName = customJenkinsName
	if customJenkinsName == "" {
		return "", fmt.Errorf("failed to find a Jenkins server")
	}
	return customJenkinsName, nil
}

// GetJenkinsServiceNames returns the list of jenkins service names
func (o *JenkinsOptions) GetJenkinsServiceNames(jenkinsSelector *JenkinsSelectorOptions) ([]string, error) {
	kubeClient := o.ClientFactory.KubeClient
	ns := o.ClientFactory.Namespace

	serviceInterface := kubeClient.CoreV1().Services(ns)
	selector := jenkinsSelector.Selector
	serviceList, err := serviceInterface.List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to list Jenkins services in namespace %s with selector %s", ns, selector)
		}
	}

	names := []string{}
	for _, svc := range serviceList.Items {
		isHttp := false
		for _, p := range svc.Spec.Ports {
			if p.Port == 8080 {
				isHttp = true
				break
			}
		}
		if isHttp {
			names = append(names, svc.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// GetJenkinsJobs returns the existing Jenkins jobs
func (o *JenkinsOptions) GetJenkinsJobs(jenkinsSelector *JenkinsSelectorOptions, filter string) (map[string]gojenkins.Job, error) {
	jobMap := map[string]gojenkins.Job{}
	jenkins, err := o.CreateJenkinsClientFromSelector(jenkinsSelector)
	if err != nil {
		return jobMap, err
	}
	jobs, err := jenkins.GetJobs()
	if err != nil {
		return jobMap, err
	}
	o.AddJenkinsJobs(jenkins, &jobMap, filter, "", jobs)
	return jobMap, nil
}

// AddJenkinsJobs add the given jobs to Jenkins
func (o *JenkinsOptions) AddJenkinsJobs(jenkins gojenkins.JenkinsClient, jobMap *map[string]gojenkins.Job, filter string, prefix string, jobs []gojenkins.Job) {
	for _, j := range jobs {
		name := jxjenkins.JobName(prefix, &j)
		if jxjenkins.IsPipeline(&j) {
			if filter == "" || strings.Contains(name, filter) {
				(*jobMap)[name] = j
				continue
			}
		}
		if j.Jobs != nil {
			o.AddJenkinsJobs(jenkins, jobMap, filter, name, j.Jobs)
		} else {
			job, err := jenkins.GetJob(name)
			if err == nil && job.Jobs != nil {
				o.AddJenkinsJobs(jenkins, jobMap, filter, name, job.Jobs)
			}
		}
	}
}

// TailJenkinsBuildLog tail the build log of the given Jenkins jobs name
func (o *JenkinsOptions) TailJenkinsBuildLog(jenkinsSelector *JenkinsSelectorOptions, jobName string, build *gojenkins.Build) error {
	jenkins, err := o.CreateJenkinsClientFromSelector(jenkinsSelector)
	if err != nil {
		return nil
	}

	u, err := url.Parse(build.Url)
	if err != nil {
		return err
	}
	buildPath := u.Path
	log.Logger().Infof("%s %s", "tailing the log of", fmt.Sprintf("%s #%d", jobName, build.Number))
	// TODO Logger
	return jenkins.TailLog(buildPath, o.GetIOFileHandles().Out, time.Second, time.Hour*100)
}

// GetJenkinsJobName returns the Jenkins job name
func (o *JenkinsOptions) GetJenkinsJobName() string {
	owner := os.Getenv("REPO_OWNER")
	repo := os.Getenv("REPO_NAME")
	branch := o.GetBranchName("")

	if owner != "" && repo != "" && branch != "" {
		return fmt.Sprintf("%s/%s/%s", owner, repo, branch)
	}

	job := os.Getenv("JOB_NAME")
	if job != "" {
		return job
	}
	return ""
}

func (o *JenkinsOptions) GetBranchName(dir string) string {
	branch := builds.GetBranchName()
	if branch == "" {
		if dir == "" {
			dir = "."
		}
		var err error
		branch, err = o.Git().Branch(dir)
		if err != nil {
			log.Logger().Warnf("failed to get the git branch name in dir %s", dir)
		}
	}
	return branch
}

// GetIOFileHandles returns In, Out, and Err as an IOFileHandles struct
func (o *JenkinsOptions) GetIOFileHandles() util.IOFileHandles {
	if o.IOFileHandles == nil {
		o.IOFileHandles = &util.IOFileHandles{
			Err: os.Stderr,
			In:  os.Stdin,
			Out: os.Stdout,
		}
	}
	return *o.IOFileHandles
}

// Git returns the git client
func (o *JenkinsOptions) Git() gits.Gitter {
	if o.git == nil {
		o.git = gits.NewGitCLI()
	}
	return o.git
}

// SetGit sets the git client
func (o *JenkinsOptions) SetGit(git gits.Gitter) {
	o.git = git
}

// FindGitInfo parses the git information from the given directory
func (o *JenkinsOptions) FindGitInfo(dir string) (*gits.GitRepository, error) {
	_, gitConf, err := o.Git().FindGitConfigDir(dir)
	if err != nil {
		return nil, fmt.Errorf("Could not find a .git directory: %s\n", err)
	} else {
		if gitConf == "" {
			return nil, fmt.Errorf("No git conf dir found")
		}
		gitURL, err := o.Git().DiscoverUpstreamGitURL(gitConf)
		if err != nil {
			return nil, fmt.Errorf("Could not find the remote git source URL:  %s", err)
		}
		return gits.ParseGitURL(gitURL)
	}
}
