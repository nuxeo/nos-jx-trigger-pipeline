package jenkinsutil

import (
	"net/http"
	"strings"

	gojenkins "github.com/jenkins-x/golang-jenkins"
)

// JenkinsServer represents a jenkins server discovered via Service selectors or via the
// trigger-pipeline secrets
type JenkinsServer struct {
	// Name the name of the Jenkins server in the registry. Should be a valid kubernetes name
	Name string

	// URL the URL to connect to the Jenkins server
	URL string

	// SecretName the name of the Secret in the registry
	SecretName string

	// Auth the username and token used to access the Jenkins server
	Auth gojenkins.Auth
}

// CreateClient creates a Jenkins client for a jenkins service
func (j *JenkinsServer) CreateClient() (gojenkins.JenkinsClient, error) {
	// lets trim trailing slashes to avoid the client using a // in the generated URLs
	u := strings.TrimSuffix(j.URL, "/")
	jenkins := gojenkins.NewJenkins(&j.Auth, u)
	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	jenkins.SetHTTPClient(httpClient)
	return jenkins, nil
}
