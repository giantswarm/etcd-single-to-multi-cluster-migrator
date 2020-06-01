package migrator

import (
	"github.com/giantswarm/microerror"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sort"
)

const (
	labelMasterID = "masterID"
)

func createK8SClient() (kubernetes.Interface, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, microerror.Mask(err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return clientset, nil
}

func getNodeNames(nodes []v1.Node) []string {
	list := []string{
		nodes[0].Name,
		nodes[1].Name,
		nodes[2].Name,
	}
	// sort nodes by masterID
	sort.Slice(list, func(i int, j int) bool {
		return nodes[i].Labels[labelMasterID] < nodes[j].Labels[labelMasterID]
	})

	return list
}
