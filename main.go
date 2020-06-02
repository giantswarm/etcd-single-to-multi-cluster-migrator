package main

import (
	"fmt"
	"os"

	"github.com/giantswarm/microerror"
	flag "github.com/spf13/pflag"

	"github.com/giantswarm/etcd-cluster-migrator/migrator"
	"github.com/giantswarm/etcd-cluster-migrator/pkg/project"
)

type Flag struct {
	BaseDomain        string
	DockerRegistry    string
	EtcdCaFile        string
	EtcdCertFile      string
	EtcdEndpoint      string
	EtcdKeyFile       string
	EtcdStartingIndex int
	MasterNodesLabel  string
}

func main() {
	err := mainError()
	if err != nil {
		panic(fmt.Sprintf("%#v", err))
	}
}

func mainError() error {
	var err error

	var f Flag
	flag.StringVar(&f.BaseDomain, "base-domain", "abcde.k8s.ginger.eu-west-1.aws.gigantic.io", "Base domain that is used for the etcd DNS address.")
	flag.StringVar(&f.DockerRegistry, "docker-registry", "quay.io", "Docker registry for the run command container.")
	flag.StringVar(&f.EtcdCaFile, "etcd-ca-file", "/etc/kubernetes/ssl/etcd/server-ca.pem", "Filepath to the etcd CA file.")
	flag.StringVar(&f.EtcdCertFile, "etcd-crt-file", "/etc/kubernetes/ssl/etcd/server-crt.pem", "Filepath to the etcd certificate file.")
	flag.StringVar(&f.EtcdEndpoint, "etcd-endpoint", "127.0.0.1:2379", "Etcd endpoint for connection to the etcd server.")
	flag.StringVar(&f.EtcdKeyFile, "etcd-key-file", "/etc/kubernetes/ssl/etcd/server-key.pem", "Filepath to the etcd private key file.")
	flag.IntVar(&f.EtcdStartingIndex, "etcd-starting-index", 1, "Starting index for the etcd DNS address.")
	flag.StringVar(&f.MasterNodesLabel, "master-node-label", "role=master", "Label selector to match against all master nodes.")

	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("%s:%s - %s", project.Name(), project.Version(), project.GitSHA())
		return nil
	}
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		flag.Usage()
		return nil
	}
	flag.Parse()

	var m *migrator.Migrator
	{
		c := migrator.MigratorConfig{
			BaseDomain:        f.BaseDomain,
			DockerRegistry:    f.DockerRegistry,
			EtcdCaFile:        f.EtcdCaFile,
			EtcdCertFile:      f.EtcdCertFile,
			EtcdEndpoint:      f.EtcdEndpoint,
			EtcdKeyFile:       f.EtcdKeyFile,
			EtcdStartingIndex: f.EtcdStartingIndex,
			MasterNodeLabel:   f.MasterNodesLabel,
		}

		m, err = migrator.NewMigrator(c)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	err = m.Run()

	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}
