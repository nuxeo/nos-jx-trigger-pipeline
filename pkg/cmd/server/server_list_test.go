package server_test

import (
	"testing"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/cmd/server"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServerAddList(t *testing.T) {
	cf := NewFakeClientFactory()

	_, lo := server.NewCmdList()
	_, ao := server.NewCmdAdd()
	_, do := server.NewCmdDelete()

	lo.ClientFactory = cf
	do.ClientFactory = cf

	// lets add a server
	ao.ClientFactory = cf
	ao.BatchMode = true
	ao.JenkinsService.Name = "foo"
	ao.JenkinsService.URL = "https://foo.com/"
	ao.JenkinsService.Auth.Username = "myuser"
	ao.JenkinsService.Auth.ApiToken = "mytoken"
	err := ao.Run()
	require.NoError(t, err, "failed to add Jenkins server %s", ao.JenkinsService.Name)

}

// NewFakeClientFactory returns a fake factory for testing
func NewFakeClientFactory() *jenkinsutil.ClientFactory {
	return NewFakeClientFactoryWithObjects(nil, "jx")
}

// NewFakeClientFactoryWithObjects returns a fake factory for testing with the given initial objects
func NewFakeClientFactoryWithObjects(kubeObjects []runtime.Object, namespace string) *jenkinsutil.ClientFactory {
	if len(kubeObjects) == 0 {
		kubeObjects = append(kubeObjects, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"tag":  "",
					"team": namespace,
					"env":  "dev",
				},
			},
		})
	}
	return &jenkinsutil.ClientFactory{
		KubeClient: fake.NewSimpleClientset(kubeObjects...),
		Namespace:  namespace,
	}
}
