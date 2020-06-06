package jenkinsutil

import (
	"sort"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/common"
	"github.com/jenkins-x/jx/v2/pkg/log"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// DefaultJenkinsSelector default selector options to use if finding Jenkins services
	DefaultJenkinsSelector = JenkinsSelectorOptions{
		Selector:  JenkinsSelector,
		NameLabel: JenkinsNameLabel,
	}
)

// FindJenkinsServers discovers the jenkins services
func FindJenkinsServers(f *ClientFactory, jenkinsSelector *JenkinsSelectorOptions) (map[string]*JenkinsServer, []string, error) {
	m, err := findServersBySelector(f, jenkinsSelector)
	if err != nil {
		return nil, nil, err
	}

	m2, err := findFromSecretRegistry(f)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range m2 {
		m[k] = v
	}

	names := []string{}
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return m, names, nil
}

// findFromSecretRegistry discovers the Trigger Pipeline Secrets
func findFromSecretRegistry(f *ClientFactory) (map[string]*JenkinsServer, error) {
	m := map[string]*JenkinsServer{}
	kubeClient := f.KubeClient
	ns := f.Namespace

	selector := common.RegistryLabel + "= " + common.RegistryLabelValue
	secretsList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return m, errors.Wrapf(err, "failed to list Jenkins secrets in namespace %s with selector %s", ns, selector)
		}
	}

	for _, secret := range secretsList.Items {
		if secret.Labels != nil {
			name := secret.Labels[common.JenkinsNameLabel]
			if name != "" {
				u := ""
				if secret.Annotations != nil {
					u = secret.Annotations[common.JenkinsURLAnnotation]
				}
				auth := PopulateAuth(&secret)
				m[name] = &JenkinsServer{
					Name:       name,
					URL:        u,
					SecretName: secret.Name,
					Auth:       *auth,
				}
			}
		}
	}
	return m, nil
}

// findServersBySelector discovers the jenkins services
func findServersBySelector(f *ClientFactory, jenkinsSelector *JenkinsSelectorOptions) (map[string]*JenkinsServer, error) {
	m := map[string]*JenkinsServer{}
	if jenkinsSelector == nil {
		return m, nil
	}
	kubeClient := f.KubeClient
	ns := f.Namespace

	serviceInterface := kubeClient.CoreV1().Services(ns)
	selector := jenkinsSelector.Selector
	serviceList, err := serviceInterface.List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return m, errors.Wrapf(err, "failed to list Jenkins services in namespace %s with selector %s", ns, selector)
		}
	}

	secretsList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return m, errors.Wrapf(err, "failed to list Jenkins secrets in namespace %s with selector %s", ns, selector)
		}
	}

	for _, svc := range serviceList.Items {
		isHttp := false
		for _, p := range svc.Spec.Ports {
			// lets filter out services for the agent port - only http/https based services only
			if p.Port == 8080 {
				isHttp = true
				break
			}
		}
		if isHttp {
			name, jsvc, err := createJenkinsServiceFromSelector(f, svc, secretsList, jenkinsSelector)
			if err != nil {
				return m, err
			}
			if name != "" && jsvc != nil {
				m[name] = jsvc
			}
		}
	}
	return m, nil
}

func createJenkinsServiceFromSelector(f *ClientFactory, svc corev1.Service, secrets *corev1.SecretList, jenkinsSelector *JenkinsSelectorOptions) (string, *JenkinsServer, error) {
	name := ""
	if svc.Labels != nil {
		name = svc.Labels[jenkinsSelector.NameLabel]
	}
	if name == "" {
		name = svc.Name
	}
	if name == "" {
		return "", nil, nil
	}

	u, err := f.createJenkinsURL(name)
	if err != nil {
		return name, nil, errors.Wrapf(err, "failed to find URL for Jenkins %s", name)
	}

	// lets find the secret
	for _, sec := range secrets.Items {
		labels := sec.Labels
		if labels != nil {
			if labels[jenkinsSelector.NameLabel] == name {
				auth := PopulateAuth(&sec)
				return name, &JenkinsServer{
					Name: name,
					URL:  u,
					Auth: *auth,
				}, nil
			}
		}
	}
	log.Logger().Warnf("could not find a Secret with selector %s which has the label %s=%s", jenkinsSelector.Selector, jenkinsSelector.NameLabel, name)
	return "", nil, nil
}
