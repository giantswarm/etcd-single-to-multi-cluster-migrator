package migrator

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/giantswarm/microerror"
)

const (
	dialTimeout = time.Minute
)

func createEtcdClient(caFile string, certFile string, keyFile string, endpoint string) (*clientv3.Client, error) {
	etcdCertPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	etcdCaCert, err := CertPoolFromFile(caFile)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	config := clientv3.Config{
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

	client, err := clientv3.New(config)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return client, nil
}

func etcdPeerName(index int, baseDomain string) string {
	return fmt.Sprintf("https://etcd%d.%s:2380", index, baseDomain)
}

func initialCluster(startingIndex int, baseDomain string, nodesCount int) string {
	r := fmt.Sprintf("etcd%d=https://etcd%d.%s:2380", startingIndex, startingIndex, baseDomain)

	for i := 1; i < nodesCount; i++ {
		r += fmt.Sprintf(",etcd%d=https://etcd%d.%s:2380", startingIndex+i, startingIndex+i, baseDomain)
	}

	return r
}
