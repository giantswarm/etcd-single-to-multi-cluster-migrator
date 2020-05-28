package migrator

import (
	"github.com/giantswarm/microerror"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
