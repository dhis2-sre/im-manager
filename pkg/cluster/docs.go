// Package cluster provides CRUD operations for cluster management.
//
// This package implements the cluster domain logic including:
// - Creating, reading, updating, and deleting clusters
// - Managing cluster-group relationships
// - Handling Kubernetes configuration for clusters
//
// The package follows a layered architecture with:
// - Handler: HTTP request/response handling
// - Service: Business logic
// - Repository: Data access layer
//
// swagger:meta
package cluster

import "github.com/dhis2-sre/im-manager/pkg/model"

// swagger:model
// A cluster represents a cluster configuration
type Cluster struct {
	// The cluster model
	// in: body
	Body model.Cluster
}

// swagger:model
// A list of clusters
type Clusters struct {
	// The clusters
	// in: body
	Body []model.Cluster
}

// swagger:model
// Save cluster request
type _ struct {
	// The name of the cluster
	// required: true
	// example: production-cluster
	Name string `json:"name"`

	// The description of the cluster
	// required: true
	// example: Production Kubernetes cluster for DHIS2 deployments
	Description string `json:"description"`

	// The Kubernetes configuration file (kubeconfig)
	// This should be a YAML file containing cluster access credentials
	// required: true
	KubernetesConfiguration []byte `json:"kubernetesConfiguration"`
}

// swagger:model
// Update cluster request
type _ struct {
	// The name of the cluster
	// example: production-cluster-updated
	Name string `json:"name"`

	// The description of the cluster
	// example: Updated production Kubernetes cluster for DHIS2 deployments
	Description string `json:"description"`

	// The Kubernetes configuration file (kubeconfig)
	// This should be a YAML file containing cluster access credentials
	KubernetesConfiguration []byte `json:"kubernetesConfiguration"`
}

// swagger:parameters clusterCreate
type CreateClusterParams struct {
	// The cluster creation request
	// in: body
	// required: true
	Body CreateClusterRequest
}

// swagger:parameters clusterUpdate
type UpdateClusterParams struct {
	// The cluster ID
	// in: path
	// required: true
	ID uint `json:"id"`

	// The cluster update request
	// in: body
	// required: true
	Body UpdateClusterRequest
}

// swagger:parameters findClusterById clusterDelete
type _ struct {
	// The cluster ID
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:response Clusters
type ClustersResponse struct {
	// List of clusters
	// in: body
	Body []model.Cluster
}
