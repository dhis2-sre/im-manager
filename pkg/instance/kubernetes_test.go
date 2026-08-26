package instance

import (
	"context"
	"fmt"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeletePersistentVolumeClaim(t *testing.T) {
	const (
		namespace    = "test-ns"
		instanceName = "mydb"
		groupID      = uint(7)
	)

	group := &model.Group{ID: groupID, Namespace: namespace}
	uniqueName := fmt.Sprintf("%s-%d", instanceName, groupID)

	tests := []struct {
		name          string
		stack         string
		pvcs          []*v1.PersistentVolumeClaim
		wantDeleted   int
		wantUnmatched int
	}{
		{
			name:  "dhis2-db",
			stack: "dhis2-db",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-db-0", uniqueName+"-database"),
			},
			wantDeleted: 1,
		},
		{
			name:  "dhis2",
			stack: "dhis2",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-db-0", uniqueName+"-database"),
				labeledPVC(namespace, "data-redis-0", uniqueName+"-redis"),
			},
			wantDeleted: 2,
		},
		{
			name:  "dhis2-core",
			stack: "dhis2-core",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-core-0", uniqueName),
				labeledPVC(namespace, "data-minio-0", uniqueName+"-minio"),
			},
			wantDeleted: 2,
		},
		{
			name:          "stack without volumes reports nothing",
			stack:         "whoami-go",
			pvcs:          nil,
			wantDeleted:   0,
			wantUnmatched: 0,
		},
		{
			name:          "selector matching nothing is reported rather than silent",
			stack:         "dhis2-db",
			pvcs:          nil,
			wantDeleted:   0,
			wantUnmatched: 1,
		},
		{
			name:  "every claim a selector matches is deleted",
			stack: "dhis2-db",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-db-0", uniqueName+"-database"),
				labeledPVC(namespace, "data-db-1", uniqueName+"-database"),
				labeledPVC(namespace, "data-db-2", uniqueName+"-database"),
			},
			wantDeleted: 3,
		},
		{
			name:  "one pattern matching nothing does not stop the others",
			stack: "dhis2",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-redis-0", uniqueName+"-redis"),
			},
			wantDeleted:   1,
			wantUnmatched: 1,
		},
		{
			name:  "a claim of another instance is left alone",
			stack: "dhis2-db",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-other-0", "otherdb-7-database"),
			},
			wantDeleted:   0,
			wantUnmatched: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			objs := make([]runtime.Object, len(tc.pvcs))
			for i, p := range tc.pvcs {
				objs[i] = p
			}
			fakeClient := fake.NewSimpleClientset(objs...)

			ks := &kubernetesService{client: fakeClient}
			inst := &model.DeploymentInstance{
				Name:      instanceName,
				StackName: tc.stack,
				Group:     group,
			}

			result, err := ks.deletePersistentVolumeClaim(inst)
			require.NoError(t, err)

			assert.Lenf(t, result.Deleted, tc.wantDeleted, "stack %q: deleted claims reported", tc.stack)
			assert.Lenf(t, result.UnmatchedSelectors, tc.wantUnmatched, "stack %q: selectors matching nothing reported", tc.stack)

			remaining, err := fakeClient.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)

			wantRemaining := len(tc.pvcs) - tc.wantDeleted
			assert.Lenf(t, remaining.Items, wantRemaining,
				"stack %q: expected %d PVC(s) remaining after deletePersistentVolumeClaim", tc.stack, wantRemaining)
		})
	}
}

func labeledPVC(namespace, name, instanceLabelValue string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/instance": instanceLabelValue},
		},
	}
}
