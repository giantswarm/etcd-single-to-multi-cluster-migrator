package migrator

import (
	"context"
	"fmt"
	"time"

	"github.com/giantswarm/backoff"
	"github.com/giantswarm/microerror"
	etcdclientv3 "go.etcd.io/etcd/clientv3"
	etcdserver "go.etcd.io/etcd/etcdserver/etcdserverpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	maxRetriesApi   = 20
	maxRetriesNodes = 100

	masterNodeCount         = 3
	masterNodeFetchInterval = time.Second * 10

	waitApiStartInterval = time.Second * 30
	waitApiRetryInterval = time.Second * 5
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

	etcdClient *etcdclientv3.Client
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
	defer m.etcdClient.Close()
	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	nodeNames, err := getMasterNodes(m.k8sClient, m.masterNodeLabel)
	if err != nil {
		return microerror.Mask(err)
	}

	memberListResponse, err := m.etcdClient.Cluster.MemberList(ctxWithTimeout)
	if err != nil {
		return microerror.Mask(err)
	}
	memberCount := len(memberListResponse.Members)

	fmt.Printf("Found %d etcd members in the cluster.\n", memberCount)

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

	fmt.Printf("ETCD cluster migration succesfuly finished.\n\n")
	return nil
}

// fixFirstNodePeerUrl ensure the peerURL for the first node in etcdcluster is properly set
// as it can have 'localhost' value from the previous version fo k8scloudconfig.
func (m *Migrator) fixFirstNodePeerUrl(ctx context.Context, etcdMembers []*etcdserver.Member) error {
	id := etcdMembers[0].ID
	peerUrls := []string{etcdPeerName(m.etcdStartingIndex, m.baseDomain)}

	_, err := m.etcdClient.Cluster.MemberUpdate(ctx, id, peerUrls)
	if err != nil {
		return microerror.Mask(err)
	}
	fmt.Printf("Updated first node PeerUrls to %s.\n", peerUrls)
	return nil
}

// addNodeToEtcdCluster configure etcd3 service on the second or third node in order
// to join the existing cluster via k8s job executed on the node and after that
// it will add the node to the etcd cluster via etcdv3 client API.

func (m *Migrator) addNodeToEtcdCluster(ctx context.Context, nodeNames []string, nodeCount int) error {
	// nodeCount can only be 2 or 3
	// 2 when adding second node to a single node etcd cluster
	// 3 when adding third node to two node etcd cluster
	if nodeCount != 2 && nodeCount != 3 {
		return microerror.Maskf(executionFailedError, "nodeCount can only have values 2 or 3")
	}

	if len(nodeNames) != 3 {
		return microerror.Maskf(executionFailedError, "nodeNames len must be 3")
	}

	// execute commands on the node to configure new etcd3 member so that it can join the existing cluster
	{
		nodeName := nodeNames[nodeCount-1]

		// the final sed command may look like this:
		// sed -i 's/--initial-cluster .*\\/--initial-cluster etcd1=https://etcd1.clusterd.domain.io:2380,etcd1=https://etcd2.clusterd.domain.io:2380/g' /etc/systemd/system/etcd3.service'
		sedReplaceRegEx := "--initial-cluster .*\\\\"
		sedReplaceWith := fmt.Sprintf("--initial-cluster %s\\\\", initialCluster(m.etcdStartingIndex, m.baseDomain, nodeCount))
		sedInitialClusterCmd := fmt.Sprintf("sed -i 's/%s/%s/g' /etc/systemd/system/etcd3.service", sedReplaceRegEx, sedReplaceWith)

		commands := []string{
			"systemctl stop etcd3",          // stop etcd3 service
			"rm -rf /var/lib/etcd/member",   // ensure the data folder is empty
			sedInitialClusterCmd,            // sed command to properly set initialCluster string
			"systemctl daemon-reload",       // load new etcd3 service file
			"systemctl start etcd3.service", // restart etcd3, after this etcd3 will start syncing data from the cluster
		}

		fmt.Printf("Configuring node %s for etcd cluster.\n", nodeName)
		// execute commands above on the node via k8s job
		err := m.runCommandsOnNode(nodeName, commands)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	nodeIndex := m.etcdStartingIndex + nodeCount - 1
	// add the new node to the etcd cluster via etcd client API
	{
		peerUrls := []string{etcdPeerName(nodeIndex, m.baseDomain)}
		r, err := m.etcdClient.Cluster.MemberAdd(ctx, peerUrls)
		if err != nil {
			return microerror.Mask(err)
		}
		fmt.Printf("Added new member %s to the etcd cluster.\n", r.Member.PeerURLs)
	}

	// wait until k8s api is available again, as etcd data sync will make API unavailable for short time
	err := waitForApiAvailable(m.k8sClient)
	if err != nil {
		return microerror.Mask(err)
	}

	fmt.Printf("Etcd cluster synced, node %s succesfully joined etcd cluster.\n", nodeNames[nodeCount-1])

	return nil
}

func getMasterNodes(c kubernetes.Interface, labelSelector string) ([]string, error) {
	var nodeNames []string

	b := backoff.NewMaxRetries(maxRetriesNodes, masterNodeFetchInterval)
	o := func() error {
		nodeList, err := c.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return microerror.Mask(err)
		}
		if len(nodeList.Items) == masterNodeCount {
			nodeNames = getNodeNames(nodeList.Items)
			fmt.Printf("Found %d masters %s, %s, %s.\n", masterNodeCount, nodeNames[0], nodeNames[1], nodeNames[2])
			return nil
		} else {
			fmt.Printf("Found %d masters but expected %d. Retrying in %.2fs\n", len(nodeList.Items), masterNodeCount, masterNodeFetchInterval.Seconds())
			return microerror.Mask(executionFailedError)
		}
	}
	err := backoff.Retry(o, b)
	if err != nil {
		fmt.Printf("Failed to reach k8s API after %d retries.\n", maxRetriesApi)
		return nil, microerror.Mask(err)
	}
	return nodeNames, nil
}

// waitForApiAvailable wait until k8s api is available which indicates that etcd cluster is synced with the new member.
func waitForApiAvailable(c kubernetes.Interface) error {
	fmt.Printf("Waiting for the etcd data sync.\n")
	time.Sleep(waitApiStartInterval)

	b := backoff.NewMaxRetries(maxRetriesApi, waitApiRetryInterval)
	o := func() error {
		_, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			fmt.Printf("API is still down. retrying in %.2f.\n", waitApiRetryInterval.Seconds())
			return microerror.Mask(err)
		}

		return nil
	}
	err := backoff.Retry(o, b)
	if err != nil {
		fmt.Printf("Failed to reach k8s API after %d retries.\n", maxRetriesApi)
		return microerror.Mask(err)
	}

	return nil
}
