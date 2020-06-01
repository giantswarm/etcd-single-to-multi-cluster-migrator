package migrator

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	etcdserver "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/giantswarm/microerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	masterNodeCount         = 3
	masterNodeFetchInterval = time.Second * 10
)

type MigratorConfig struct {
	BaseDomain        string
	DockerRegistry    string
	EtcdCaFile        string
	EtcdCertFile      string
	EtcdEndpoint      string
	EtcdKeyFile       string
	EtcdStartingIndex int
	MasterNodeLabel   string
}

type Migrator struct {
	baseDomain        string
	dockerRegistry    string
	etcdStartingIndex int
	masterNodeLabel   string

	etcdClient *clientv3.Client
	k8sClient  kubernetes.Interface
}

func NewMigrator(config MigratorConfig) (*Migrator, error) {
	if config.BaseDomain == "" {
		return nil, microerror.Maskf(invalidConfigError, fmt.Sprintf("%T.BaseDomain must not be empty", config))
	}
	if config.DockerRegistry == "" {
		return nil, microerror.Maskf(invalidConfigError, fmt.Sprintf("%T.DockerRegistry must not be empty", config))
	}
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
		baseDomain:        config.BaseDomain,
		dockerRegistry:    config.DockerRegistry,
		etcdStartingIndex: config.EtcdStartingIndex,
		masterNodeLabel:   config.MasterNodeLabel,

		etcdClient: etcdClient,
		k8sClient:  k8sClient,
	}

	return m, nil
}

func (m *Migrator) Run() error {
	ctx := context.Background()

	var nodeNames []string
	for {
		// fetch  master nodeList
		nodeList, err := m.k8sClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: m.masterNodeLabel})
		if err != nil {
			return microerror.Mask(err)
		}
		if len(nodeList.Items) == masterNodeCount {
			fmt.Printf("Found %d masters nodeList %s, %s, %s.\n", masterNodeCount, nodeList.Items[0].Name, nodeList.Items[1].Name, nodeList.Items[2].Name)
			nodeNames = getNodeNames(nodeList.Items)
			break
		} else {
			fmt.Printf("Found %d masters nodeList but expected %d. Retrying in %.2fs\n", len(nodeList.Items), masterNodeCount, masterNodeFetchInterval.Seconds())
		}

		time.Sleep(masterNodeFetchInterval)
	}

	memberListResponse, err := m.etcdClient.MemberList(ctx)
	if err != nil {
		return microerror.Mask(err)
	}
	memberCount := len(memberListResponse.Members)

	if memberCount == masterNodeCount {
		fmt.Printf("Etcd cluster already has 3 nodes. Nothing to do. Exiting.\n")
		return nil
	} else if memberCount == 2 {
		// continue migration that was interrupted in the middle of the process
		err = m.addNodeToEtcdCluster(ctx, nodeNames, 3)
		if err != nil {
			return microerror.Mask(err)
		}

	} else if memberCount == 1 {
		//  ensure that first node has proper etcd peer url set to etcd1.xxxx.xxxx.xxx
		err = m.fixFirstNodePeerUrl(ctx, memberListResponse.Members)
		if err != nil {
			return microerror.Mask(err)
		}
		// add second node to the etcd cluster
		err = m.addNodeToEtcdCluster(ctx, nodeNames, 2)
		if err != nil {
			return microerror.Mask(err)
		}
		// add third node to the etcd cluster
		err = m.addNodeToEtcdCluster(ctx, nodeNames, 3)
		if err != nil {
			return microerror.Mask(err)
		}
	} else {
		fmt.Printf("unexpected number of nodes in etcd cluster\n")
		return microerror.Maskf(executionFailedError, fmt.Sprintf("found %d nodes in etcd cluster", memberCount))
	}

	memberListResponse, err = m.etcdClient.MemberList(ctx)
	if err != nil {
		return microerror.Mask(err)
	}

	fmt.Printf("ETCD cluster migration succesfuly finished. Member list %#v.\n", memberListResponse.Members)
	return nil
}

func (m *Migrator) fixFirstNodePeerUrl(ctx context.Context, etcdMembers []*etcdserver.Member) error {
	id := etcdMembers[0].ID
	peerUrls := []string{etcdPeerName(m.etcdStartingIndex, m.baseDomain)}

	_, err := m.etcdClient.MemberUpdate(ctx, id, peerUrls)
	if err != nil {
		return microerror.Mask(err)
	}
	fmt.Printf("Updated node %d PeerUrls to %#v.\n", id, peerUrls)
	return nil
}

func (m *Migrator) addNodeToEtcdCluster(ctx context.Context, nodeNames []string, nodeCount int) error {
	// nodeCount can only be 2 or 3
	// 2  when adding second node to a single node etcd cluster
	// 3 when adding third node to two node etcd cluster
	if nodeCount != 2 && nodeCount != 3 {
		return microerror.Maskf(executionFailedError, "nodeCount can only have values 2 or 3")
	}

	// add new node to the etcd cluster via etcd client API
	{
		nodeIndex := m.etcdStartingIndex + nodeCount - 1

		peerUrls := []string{etcdPeerName(nodeIndex, m.baseDomain)}
		r, err := m.etcdClient.MemberAdd(ctx, peerUrls)
		if err != nil {
			return microerror.Mask(err)
		}
		fmt.Printf("Sucesfully added new member %#v to the etcd cluster.\n", r.Member)
	}

	// execute commands on the second node so the node join cluster and sync data
	{
		initialClusterArg := fmt.Sprintf("--initial-cluster %s\\\\", initialCluster(m.etcdStartingIndex, m.baseDomain, nodeCount))
		sedInitialClusterCmd := fmt.Sprintf("sed -i 's/--initial-cluster .*\\\\/%s/g' /etc/systemd/system/etcd3.service", initialClusterArg)

		commands := []string{
			"systemctl stop etcd3",
			"sed -i 's/--initial-cluster-state new/--initial-cluster-state existing/g' /etc/systemd/system/etcd3.service",
			sedInitialClusterCmd,
			"systemctl daemon-reload",
			"systemctl start etcd3.service",
		}

		nodeName := nodeNames[nodeCount-1]

		err := m.runCommandsOnNode(nodeName, commands)
		if err != nil {
			return microerror.Mask(err)
		}
		fmt.Printf("Sucesfully added node %s to the etcd cluster.\n", nodeName)
	}

	return nil
}
