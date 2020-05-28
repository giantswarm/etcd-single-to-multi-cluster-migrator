package migrator

import (
	"crypto/tls"
	"github.com/giantswarm/microerror"
	"time"

	"github.com/coreos/etcd/clientv3"
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
