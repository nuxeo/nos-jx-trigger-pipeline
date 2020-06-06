package jenkinsutil

import (
	"fmt"
	"net/url"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/v2/pkg/kube"
	"github.com/jenkins-x/jx/v2/pkg/kube/services"
	"github.com/jenkins-x/jx/v2/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	// this is so that we load the auth plugins so we can connect to, say, GCP

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type ClientFactory struct {
	KubeClient            kubernetes.Interface
	Namespace             string
	Batch                 bool
	InCluster             bool
	DevelopmentJenkinsURL string
}

// CreateJenkinsClient creates a new Jenkins client for the given custom Jenkins App
func (f *ClientFactory) CreateJenkinsClient(jenkinsName string) (gojenkins.JenkinsClient, error) {
	selector := DefaultJenkinsSelector
	selector.JenkinsName = jenkinsName
	m, names, err := FindJenkinsServers(f, &selector)
	if err != nil {
		return nil, err
	}
	jsvc := m[jenkinsName]
	if jsvc != nil {
		return jsvc.CreateClient()
	}
	return nil, util.InvalidOption("jenkins", jenkinsName, names)
}

// JenkinsURL gets a given jenkins service's URL
func (f *ClientFactory) JenkinsURL(jenkinsServiceName string) (string, error) {
	client := f.KubeClient
	ns := f.Namespace
	url, err := services.FindServiceURL(client, ns, jenkinsServiceName)
	if err != nil {
		// lets try the real environment
		realNS, _, err := kube.GetDevNamespace(client, ns)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get the dev namespace from '%s' namespace", ns)
		}
		if realNS != ns {
			url, err = services.FindServiceURL(client, realNS, jenkinsServiceName)
			if err != nil {
				return "", errors.Wrapf(err, "failed to find service URL for %s in namespaces %s and %s", jenkinsServiceName, realNS, ns)
			}
			return url, nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("%s in namespace %s", err, ns)
	}
	return url, err
}

// PopulateAuth populates the gojenkins Auth
func PopulateAuth(secret *v1.Secret) *gojenkins.Auth {
	userAuth := &gojenkins.Auth{}
	if userAuth.Username == "" {
		userAuth.Username = string(secret.Data[kube.JenkinsAdminUserField])
	}
	if userAuth.ApiToken == "" {
		userAuth.ApiToken = string(secret.Data[kube.JenkinsAdminApiToken])
	}
	if userAuth.BearerToken == "" {
		userAuth.BearerToken = string(secret.Data[kube.JenkinsBearTokenField])
	}

	// jenkins operator keys
	if userAuth.Username == "" {
		userAuth.Username = string(secret.Data["user"])
	}
	if userAuth.ApiToken == "" {
		userAuth.ApiToken = string(secret.Data["token"])
	}
	return userAuth
}

func (f *ClientFactory) createJenkinsURL(jenkinsServiceName string) (string, error) {
	svcURL := ""
	var err error
	if f.InCluster {
		svcURL = "http://" + jenkinsServiceName + ":8080"
	} else {
		svcURL, err = services.FindServiceURL(f.KubeClient, f.Namespace, jenkinsServiceName)
		if err != nil {
			log.Logger().Debugf("ignoring error finding jenkins service URL for %s as it probably has no Ingress: %s", jenkinsServiceName, err.Error())
		}
	}
	if svcURL == "" {
		if f.InCluster {
			// lets use the local service URL
			svcURL = "http://" + jenkinsServiceName
		} else {
			// lets allow the developer to pass in a custom URL if we are testing locally without ingress on the jenkins server
			// and we are using: kubectl port-forward jenkins-server1 8080:8080
			svcURL = f.DevelopmentJenkinsURL
			if svcURL == "" {
				svcURL = "http://localhost:8080"
			}
		}
	}
	_, err = url.Parse(svcURL)
	if err != nil {
		return svcURL, errors.Wrapf(err, "failed to parse jenkins URL %s", svcURL)
	}
	return svcURL, nil
}
