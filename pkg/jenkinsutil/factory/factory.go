package factory

import (
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil"
	"github.com/jenkins-x/jx/v2/pkg/jxfactory"
	"k8s.io/client-go/rest"
)

// NewClientFactory creates a new Jenkins client factory
func NewClientFactory() (*jenkinsutil.ClientFactory, error) {
	return NewClientFactoryFromFactory(jxfactory.NewFactory())
}

// NewClientFactoryFromFactory creates a new Jenkins client factory from the underlying kube factory
func NewClientFactoryFromFactory(factory jxfactory.Factory) (*jenkinsutil.ClientFactory, error) {
	kubeClient, ns, err := factory.CreateKubeClient()
	if err != nil {
		return nil, err
	}
	return &jenkinsutil.ClientFactory{
		KubeClient: kubeClient,
		Namespace:  ns,
		Batch:      false,
		InCluster:  IsInCluster(),
	}, nil
}

// IsInCluster tells if we are running incluster
func IsInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}
