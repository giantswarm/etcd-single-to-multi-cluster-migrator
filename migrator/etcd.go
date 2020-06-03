package migrator

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/giantswarm/microerror"
	etcdclientv3 "go.etcd.io/etcd/clientv3"
)

const (
	dialTimeout = time.Minute
)

func createEtcdClient(caFile string, certFile string, keyFile string, endpoint string) (*etcdclientv3.Client, error) {
	etcdCertPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	etcdCaCert, err := CertPoolFromFile(caFile)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	config := etcdclientv3.Config{
		DialTimeout: dialTimeout,
		Endpoints: []string{
			endpoint,
		},

		TLS: &tls.Config{
			Certificates:       []tls.Certificate{etcdCertPair},
			ClientCAs:          etcdCaCert,
			RootCAs:            etcdCaCert,
			InsecureSkipVerify: false,
		},
	}

	client, err := etcdclientv3.New(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client, nil
}

func etcdPeerName(index int, baseDomain string) string {
	return fmt.Sprintf("https://etcd%d.%s:2380", index, baseDomain)
}

func initialCluster(startingIndex int, baseDomain string, nodesCount int) string {
	r := fmt.Sprintf("etcd%d=https:\\/\\/etcd%d.%s:2380", startingIndex, startingIndex, baseDomain)

	for i := 1; i < nodesCount; i++ {
		r += fmt.Sprintf(",etcd%d=https:\\/\\/etcd%d.%s:2380", startingIndex+i, startingIndex+i, baseDomain)
	}

	return r
}
