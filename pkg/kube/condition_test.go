package kube

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestConditionMatches(t *testing.T) {
	var nilCondition *Condition
	assert.True(t, nilCondition.Matches(nil))

	condition := &Condition{Parameter: "STORAGE_TYPE", Equals: "minio"}
	assert.True(t, condition.Matches(model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "minio"}}))
	assert.False(t, condition.Matches(model.DeploymentInstanceParameters{"STORAGE_TYPE": {Value: "s3"}}))
	assert.False(t, condition.Matches(model.DeploymentInstanceParameters{}))
	assert.False(t, condition.Matches(nil))
}
