package server_test

import (
	"fmt"
	"testing"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/cmd/server"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServerAddList(t *testing.T) {
	cf := NewFakeClientFactory()

	servers := []string{"bar", "foo"}

	for _, name := range servers {
		// lets add a name
		_, ao := server.NewCmdAdd()
		ao.ClientFactory = cf
		ao.BatchMode = true
		ao.JenkinsService.Name = name
		ao.JenkinsService.URL = fmt.Sprintf("https://%s.acme.com", name)
		ao.JenkinsService.Auth.Username = fmt.Sprintf("myuser%s", name)
		ao.JenkinsService.Auth.ApiToken = fmt.Sprintf("mytoken%s", name)
		err := ao.Run()
		require.NoError(t, err, "failed to add Jenkins name %s", name)
	}

	_, lo := server.NewCmdList()
	lo.ClientFactory = cf
	err := lo.Run()
	require.NoError(t, err, "failed to list Jenkins servers")
	assert.Equal(t, lo.Results.Names, servers, "servers")

	for _, name := range servers {
		// lets delete a name
		_, do := server.NewCmdDelete()
		do.ClientFactory = cf
		do.ClientFactory = cf
		do.BatchMode = true
		do.Name = name
		err := do.Run()
		require.NoError(t, err, "failed to delete Jenkins name %s", name)
	}

	_, lo = server.NewCmdList()
	lo.ClientFactory = cf
	err = lo.Run()
	require.NoError(t, err, "failed to list Jenkins servers")
	assert.Empty(t, lo.Results.Names, "servers")
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
