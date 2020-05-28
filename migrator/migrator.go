package migrator

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/giantswarm/microerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	masterNodeCound         = 3
	masterNodeFetchInterval = time.Second * 10
)

type MigratorConfig struct {
	EtcdCaFile      string
	EtcdCertFile    string
	EtcdEndpoint    string
	EtcdKeyFile     string
	MasterNodeLabel string
}

type Migrator struct {
	masterNodeLabel string

	etcdClient *clientv3.Client
	k8sClient  kubernetes.Interface
}

func NewMigrator(config MigratorConfig) (*Migrator, error) {
	if config.EtcdCaFile == "" {
		return nil, microerror.Maskf(invalidConfigError, fmt.Sprintf("%T.EtcdCaFile must not be empty", config))
	}
	if config.EtcdCertFile == "" {
		return nil, microerror.Maskf(invalidConfigError, fmt.Sprintf("%T.EtcdCertFile must not be empty", config))
	}
	if config.EtcdEndpoint == "" {
		return nil, microerror.Maskf(invalidConfigError, fmt.Sprintf("%T.EtcdEndpoint must not be empty", config))
	}
	if config.EtcdKeyFile == "" {
		return nil, microerror.Maskf(invalidConfigError, fmt.Sprintf("%T.EtcdKeyFile must not be empty", config))
	}

	etcdClient, err := createEtcdClient(config.EtcdCaFile, config.EtcdCertFile, config.EtcdKeyFile, config.EtcdEndpoint)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	k8sClient, err := createK8SClient()
	if err != nil {
		return nil, microerror.Mask(err)
	}

	m := &Migrator{
		etcdClient: etcdClient,
		k8sClient:  k8sClient,
	}

	return m, nil
}

func (m *Migrator) Run() error {
	for {
		// fetch  master nodes
		nodes, err := m.k8sClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: m.masterNodeLabel})
		if err != nil {
			return microerror.Mask(err)
		}
		if len(nodes.Items) == masterNodeCound {
			fmt.Printf("Found %d masters nodes %s, %s, %s.\n", masterNodeCound, nodes.Items[0].Name, nodes.Items[1].Name, nodes.Items[2].Name)
			break
		} else {
			fmt.Printf("Found %d masters nodes but expected %d. Retrying in %.2fs\n", len(nodes.Items), masterNodeCound, masterNodeFetchInterval.Seconds())
		}

		time.Sleep(masterNodeFetchInterval)
	}

	return nil
}
