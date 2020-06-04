package migrator

import (
	"fmt"
	"sort"
	"time"

	"github.com/giantswarm/microerror"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	labelMasterID = "giantswarm.io/master-id"

	waitApiStartInterval = time.Second * 30
	waitApiRetryInterval = time.Second * 5
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

// getNodeNames return nodeName list ordered by master id label.
func getNodeNames(nodes []v1.Node) []string {
	// sort nodes by masterID
	sort.Slice(nodes, func(i int, j int) bool {
		return nodes[i].Labels[labelMasterID] < nodes[j].Labels[labelMasterID]
	})

	list := []string{
		nodes[0].Name,
		nodes[1].Name,
		nodes[2].Name,
	}
	return list
}

func waitForApiAvailable(c kubernetes.Interface) {
	fmt.Printf("Waiting for k8s api to avaiable again.\n")
	time.Sleep(waitApiStartInterval)

	for {
		_, err := c.CoreV1().Nodes().List(metav1.ListOptions{})

		if err == nil {
			break
		} else {
			fmt.Printf("API is still down.\n")
		}
		time.Sleep(waitApiRetryInterval)
	}
}
