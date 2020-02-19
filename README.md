# Trigger Pipeline

[![Documentation](https://godoc.org/github.com/jenkins-x-labs/trigger-pipeline?status.svg)](http://godoc.org/github.com/jenkins-x-labs/trigger-pipeline)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x-labs/trigger-pipeline)](https://goreportcard.com/report/github.com/jenkins-x-labs/trigger-pipeline)

This project creates a small stand alone binary for triggering pipelines in [Jenkins](https://jenkins.io/) servers that are setup and managed via GitOps and the [Jenkins Operator](https://jenkinsci.github.io/kubernetes-operator/)

It helps bridge the gap between [Jenkins](https://jenkins.io/) and [Jenkins X](https://jenkins-x.io/) around ChatOps.


## Using Trigger Pipeline CLI

Download the binary and put it on your `$PATH` or use the trigger-pipeline container image in a pipeline step in Jenkins or Jenkins X / Tekton.

To trigger a pipeline in the default Jenkins server:

``` 
tp
```

If you have multiple Jenkins server custom resources in the current namespace then specify the Jenkins custom resource name on the command line:

``` 
tp  --jenkins myJenkinsServer
```

If you are not sure what Jenkins names are available in the current namespace run:

``` 
kubectl get jenkins
```

Which will list all of the available Jenkins custom resource names.

If you make a mistake and use a name which does not exist the `tp` executable will give you a meaninful error message togehter with listing all the available Jenkins custom resource names.


### Using Environment Variables

To make it easier to configure inside pipelines you can specify the Jenkins instance name via the `$TRIGGER_JENKINS_SERVER` environment variable

```   
export TRIGGER_JENKINS_SERVER="someJenkinsCrdName"
tp
```
 
For more information type: 

``` 
tp --help
```       

## How it works

Under the covers the `trigger-pipeline` process uses Kubernetes Selectors to find Jenkins `Service` and `Secret` resources created by the [Jenkins Operator](https://jenkinsci.github.io/kubernetes-operator/) when it provisions a Jenkins server pod as a result of a `Jenkins` custom resource being created. 

You can view the Jenkins custom resources in your namespace via:

``` 
kubectl get jenkins
```

The default selector used by the [Jenkins Operator](https://jenkinsci.github.io/kubernetes-operator/) is `app=jenkins-operator`. So you can view what Jenkins `Services` / `Secrets` will be used via:

``` 
kubectl get svc -l app=jenkins-operator
kubectl get secret -l app=jenkins-operator
```