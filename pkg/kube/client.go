package kube

import (
	"context"
	"fmt"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/decrypt"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Client talks to a single cluster's Kubernetes API. Construct it with NewClient so RestConfig is
// populated; the fields are exported so tests can inject a fake Clientset.
type Client struct {
	Clientset  kubernetes.Interface
	RestConfig *rest.Config
}

func NewClient(config model.Cluster) (*Client, error) {
	restConfig, err := NewRestConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kube rest config: %v", err)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating kube client: %v", err)
	}

	return &Client{Clientset: client, RestConfig: restConfig}, nil
}

func newMetricsClient(cluster model.Cluster) (*metricsv1beta1.Clientset, error) {
	restClientConfig, err := NewRestConfig(cluster)
	if err != nil {
		return nil, err
	}

	metricsClient, err := metricsv1beta1.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	return metricsClient, nil
}

func newClient(configuration model.Cluster) (*kubernetes.Clientset, error) {
	restClientConfig, err := NewRestConfig(configuration)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(restClientConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func NewRestConfig(cluster model.Cluster) (*rest.Config, error) {
	var restClientConfig *rest.Config
	if cluster.Configuration != nil {
		kubeCfg, err := DecryptYaml(cluster.Configuration)
		if err != nil {
			return nil, err
		}

		config, err := clientcmd.NewClientConfigFromBytes(kubeCfg)
		if err != nil {
			return nil, err
		}

		restClientConfig, err = config.ClientConfig()
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		restClientConfig, err = clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			return nil, err
		}
	}

	return restClientConfig, nil
}

func DecryptYaml(data []byte) ([]byte, error) {
	return decrypt.DataWithFormat(data, formats.FormatFromString("yaml"))
}

// DiscoverIngressClass returns the IngressClass name to use for this cluster.
//
// Selection rules (in priority order):
//  1. If any IngressClass carries the annotation
//     "ingressclass.kubernetes.io/is-default-class=true", that one is used.
//     This is the standard Kubernetes convention for marking a cluster default.
//  2. If exactly one IngressClass exists (with no default annotation), it is
//     used - there is no ambiguity when only one option is present.
//  3. If multiple IngressClasses exist but none is marked default, an error is
//     returned. The operator must annotate one with
//     ingressclass.kubernetes.io/is-default-class=true.
//  4. If no IngressClasses exist at all, "" is returned silently.
func DiscoverIngressClass(ctx context.Context, cluster model.Cluster) (string, error) {
	client, err := newClient(cluster)
	if err != nil {
		return "", err
	}
	classes, err := client.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing IngressClasses: %w", err)
	}
	for _, ic := range classes.Items {
		if ic.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
			return ic.Name, nil
		}
	}
	switch len(classes.Items) {
	case 0:
		return "", nil
	case 1:
		return classes.Items[0].Name, nil
	default:
		return "", fmt.Errorf("found %d IngressClasses but none is annotated as default — annotate one with ingressclass.kubernetes.io/is-default-class=true", len(classes.Items))
	}
}

// DiscoverCertIssuer returns the cert-manager ClusterIssuer name to use for
// this cluster.
//
// Selection rules:
//  1. If exactly one ClusterIssuer exists, it is used — unambiguous.
//  2. If multiple ClusterIssuers exist we cannot choose safely (there is no
//     "default issuer" concept in cert-manager), so an error is returned.
//     The operator must configure the cert issuer explicitly.
//  3. If no ClusterIssuers exist, "" is returned silently — the cluster may
//     not use cert-manager at all, which is a valid setup.
//  4. If the ClusterIssuer CRD itself is not installed (API error), "" is
//     returned silently — cert-manager is not present on this cluster.
func DiscoverCertIssuer(ctx context.Context, cluster model.Cluster) (string, error) {
	restConfig, err := NewRestConfig(cluster)
	if err != nil {
		return "", err
	}
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return "", err
	}
	gvr := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "clusterissuers"}
	issuers, err := dynClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", nil // cert-manager CRD not installed - not an error
	}
	switch len(issuers.Items) {
	case 0:
		return "", nil
	case 1:
		return issuers.Items[0].GetName(), nil
	default:
		return "", fmt.Errorf("found %d ClusterIssuers — cannot auto-select, configure the cert issuer explicitly", len(issuers.Items))
	}
}

type ClusterResources struct {
	CPU        string
	Memory     string
	Autoscaled bool
	Nodes      int
}

func FindResources(cluster model.Cluster) (ClusterResources, error) {
	client, err := newClient(cluster)
	if err != nil {
		return ClusterResources{}, err
	}

	metricsClient, err := newMetricsClient(cluster)
	if err != nil {
		return ClusterResources{}, err
	}

	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ClusterResources{}, err
	}

	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ClusterResources{}, err
	}

	var totalCPUUsed, totalMemUsed, totalCPUAlloc, totalMemAlloc resource.Quantity

	for _, node := range nodes.Items {
		name := node.Name
		allocCPU := node.Status.Allocatable["cpu"]
		allocMem := node.Status.Allocatable["memory"]

		var usedCPU, usedMem resource.Quantity
		for _, metric := range nodeMetrics.Items {
			if metric.Name == name {
				usedCPU = metric.Usage["cpu"]
				usedMem = metric.Usage["memory"]
				break
			}
		}
		totalCPUUsed.Add(usedCPU)
		totalMemUsed.Add(usedMem)
		totalCPUAlloc.Add(allocCPU)
		totalMemAlloc.Add(allocMem)
	}

	clusterCPUPercent := percent(totalCPUUsed.MilliValue(), totalCPUAlloc.MilliValue())
	clusterMemPercent := percent(totalMemUsed.Value(), totalMemAlloc.Value())

	return ClusterResources{
		CPU:    fmt.Sprintf("%.1f%%", clusterCPUPercent),
		Memory: fmt.Sprintf("%.1f%%", clusterMemPercent),
		Nodes:  len(nodes.Items),
	}, nil
}

func percent(used, total int64) float64 {
	if total == 0 {
		return 0.0
	}
	return (float64(used) / float64(total)) * 100
}
