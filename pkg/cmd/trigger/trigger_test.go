package trigger_test

import (
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
	branch := "master"
	o.Dir = filepath.Join("test_data/sample")
	gitInfo := &gits.GitRepository{
		Host:         "https://github.com",
		Organisation: "myowner",
		Name:         "myrepo",
	}
	err := o.TriggerPipeline(jenkinsClient, gitInfo, branch)
	require.NoError(t, err, "should not have failed")
	assert.Equal(t, 1, len(jenkinsClient.BuildRequests), "should have a single build request")
	br := jenkinsClient.BuildRequests[0]
	t.Logf("got build request fullName: %s name: %s\n", br.Job.FullName, br.Job.Name)
}
