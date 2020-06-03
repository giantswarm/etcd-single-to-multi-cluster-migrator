package migrator

import (
	"strconv"
	"time"

	"github.com/giantswarm/microerror"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	labelMasterID = "giantswarm.io/master-id"
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
	list := []string{"", "", ""}
	// sort nodes by masterID
	for _, n := range nodes {
		i, _ := strconv.Atoi(n.Labels[labelMasterID])

		list[i-1] = n.Name
	}

	return list
}

func waitForApiAvailable(c kubernetes.Interface) {
	time.Sleep(time.Second * 30)

	for {
		_, err := c.CoreV1().Namespaces().List(metav1.ListOptions{})

		if err == nil {
			break
		}
		time.Sleep(time.Second * 5)
	}
}
