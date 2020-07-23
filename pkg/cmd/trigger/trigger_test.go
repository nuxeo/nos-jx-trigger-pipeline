package trigger_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jenkins-x-labs/trigger-pipeline/pkg/cmd/trigger"
	"github.com/jenkins-x-labs/trigger-pipeline/pkg/jenkinsutil/fake"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestTrigger(t *testing.T) {
	_, o := trigger.NewCmdTrigger()

	jenkinsClient := &fake.FakeClient{}
	o.Branch = "master"
	o.Dir = filepath.Join("test_data/sample")
	gitInfo := &gits.GitRepository{
		Host:         "https://github.com",
		Organisation: "myowner",
		Name:         "myrepo",
	}
	o.JenkinsPath = fmt.Sprintf("%s/%s/%s", gitInfo.Organisation, gitInfo.Name, o.Branch)

	err := o.TriggerPipeline(jenkinsClient, gitInfo)
	require.NoError(t, err, "should not have failed")
	assert.Equal(t, 1, len(jenkinsClient.BuildRequests), "should have a single build request")
	br := jenkinsClient.BuildRequests[0]
	t.Logf("got build request fullName: %s name: %s\n", br.Job.FullName, br.Job.Name)
}
