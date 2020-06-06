package jenkinsutil

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/v2/pkg/builds"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	jxjenkins "github.com/jenkins-x/jx/v2/pkg/jenkins"
	"github.com/jenkins-x/jx/v2/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/spf13/cobra"
)

type JenkinsOptions struct {
	ClientFactory *ClientFactory

	BatchMode     bool
	IOFileHandles *util.IOFileHandles
	git           gits.Gitter
}

// JenkinsSelectorOptions used to represent the options used to refer to a Jenkins.
// if nothing is specified it assumes the current team is using a static Jenkins server as its execution engine.
// otherwise we can refer to other additional Jenkins Apps to implement custom Jenkins servers
type JenkinsSelectorOptions struct {
	// JenkinsName the name of the Jenkins Operator Service for HTTP to use
	JenkinsName string

	// Selector label selector to find the Jenkins Operator Services
	Selector string

	// NameLabel label the label to find the name of the Jenkins service
	NameLabel string

	// DevelopmentJenkinsURL a local URL to use to talk to the jenkins server if the servers do not have Ingress
	// and you want to test out using the jenkins client locally
	DevelopmentJenkinsURL string
}

// AddFlags add the command flags for picking a custom Jenkins App to work with
func (o *JenkinsSelectorOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.JenkinsName, "jenkins", "", "", "The name of the Jenkin server provisioned by the Jenkins Operator")
	cmd.Flags().StringVarP(&o.Selector, "selector", "", JenkinsSelector, "The kubernetes label selector to find the Jenkins Operator Services for Jenkins HTTP servers")
	cmd.Flags().StringVarP(&o.NameLabel, "name-label", "", JenkinsNameLabel, "The kubernetes label used to specify the Jenkins service name")
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

// CreateJenkinsClientFromSelector is given either a specific jenkins service name to use, uses the selector to find it
// or prompts the user to pick one if not in batch mode.
func (o *JenkinsOptions) CreateJenkinsClientFromSelector(jenkinsSelector *JenkinsSelectorOptions) (gojenkins.JenkinsClient, error) {
	var err error
	_, jsvc, err := o.PickCustomJenkinsName(jenkinsSelector, true)
	if err != nil {
		return nil, err
	}

	return jsvc.CreateClient()
}

// PickCustomJenkinsName picks the name of a custom jenkins server App if available
func (o *JenkinsOptions) PickCustomJenkinsName(jenkinsSelector *JenkinsSelectorOptions, failIfNone bool) (string, *JenkinsServer, error) {
	m, names, err := FindJenkinsServers(o.ClientFactory, jenkinsSelector)
	if err != nil {
		return "", nil, err
	}
	name := jenkinsSelector.JenkinsName
	if name != "" {
		jsvc := m[name]
		if jsvc == nil {
			return "", nil, util.InvalidOption("jenkins", name, names)
		}
		return name, jsvc, nil
	}

	if o.BatchMode {
		name := os.Getenv(TriggerJenkinsServerEnv)
		if name != "" {
			jsvc := m[name]
			if jsvc == nil {
				return "", nil, fmt.Errorf("the $%s is %s but we can only find these Jenkins servers: %s", TriggerJenkinsServerEnv, name, strings.Join(names, ", "))
			}
			log.Logger().Infof("defaulting to Jenkins server %s at %s due to $%s", util.ColorInfo(name), jsvc.URL, TriggerJenkinsServerEnv)
			return name, jsvc, nil
		}
	}

	switch len(names) {
	case 0:
		if failIfNone {
			return "", nil, fmt.Errorf("No Jenkins services found. Try: tp server add")
		}
		return "", nil, nil

	case 1:
		name = names[0]

	default:
		if o.BatchMode {
			return "", nil, util.MissingOptionWithOptions("jenkins", names)
		}
		name, err = util.PickName(names, "Pick which Jenkins service you wish to use: ", "Add a Jenkins service via: tp server add", o.GetIOFileHandles())
		if err != nil {
			return "", nil, err
		}
	}
	return name, m[name], nil
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
