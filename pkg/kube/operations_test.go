package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeletePVCs(t *testing.T) {
	const namespace = "test-ns"

	tests := []struct {
		name          string
		pvcs          []*v1.PersistentVolumeClaim
		selectors     []string
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "deletes the single match",
			pvcs:          []*v1.PersistentVolumeClaim{labeledPVC(namespace, "data-db-0", "mydb-7-database")},
			selectors:     []string{"app.kubernetes.io/instance=mydb-7-database"},
			wantRemaining: 0,
		},
		{
			name: "deletes across multiple selectors",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-db-0", "mydb-7-database"),
				labeledPVC(namespace, "data-redis-0", "mydb-7-redis"),
			},
			selectors: []string{
				"app.kubernetes.io/instance=mydb-7-database",
				"app.kubernetes.io/instance=mydb-7-redis",
			},
			wantRemaining: 0,
		},
		{
			name:          "no matching pvc is a no-op",
			pvcs:          []*v1.PersistentVolumeClaim{labeledPVC(namespace, "data-db-0", "other")},
			selectors:     []string{"app.kubernetes.io/instance=mydb-7-database"},
			wantRemaining: 1,
		},
		{
			name:          "empty selectors deletes nothing",
			pvcs:          []*v1.PersistentVolumeClaim{labeledPVC(namespace, "data-db-0", "mydb-7-database")},
			selectors:     nil,
			wantRemaining: 1,
		},
		{
			name: "more than one match errors",
			pvcs: []*v1.PersistentVolumeClaim{
				labeledPVC(namespace, "data-db-0", "mydb-7-database"),
				labeledPVC(namespace, "data-db-1", "mydb-7-database"),
			},
			selectors: []string{"app.kubernetes.io/instance=mydb-7-database"},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			objs := make([]runtime.Object, len(tc.pvcs))
			for i, p := range tc.pvcs {
				objs[i] = p
			}
			c := &Client{Clientset: fake.NewSimpleClientset(objs...)}

			err := c.DeletePVCs(context.Background(), namespace, tc.selectors)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			remaining, err := c.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
			require.NoError(t, err)
			assert.Len(t, remaining.Items, tc.wantRemaining)
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
