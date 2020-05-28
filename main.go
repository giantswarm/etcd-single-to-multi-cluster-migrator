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
	EtcdCaFile       string
	EtcdCertFile     string
	EtcdKeyFile      string
	MasterNodesLabel string
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
	flag.StringVar(&f.EtcdCaFile, "etcd-ca-file", "/etc/kubernetes/ssl/etcd/etcd-ca.pem", "Filepath to the etcd CA file.")
	flag.StringVar(&f.EtcdCaFile, "etcd-ca-file", "/etc/kubernetes/ssl/etcd/etcd-ca.pem", "Filepath to the etcd CA file.")
	flag.StringVar(&f.EtcdCaFile, "etcd-ca-file", "/etc/kubernetes/ssl/etcd/etcd-ca.pem", "Filepath to the etcd CA file.")

	flag.StringVar(&f.MasterNodesLabel, "eni-tag-value", "test", "Tag value that will be used to found the requested ENI in AWS API, this tag should identify one unique ENI.")

	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("%s:%s - %s", project.Name(), project.Version(), project.GitSHA())
		return nil
	}
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		flag.Usage()
		return nil
	}
	flag.Parse()

	var m migrator.Migrator
	{
		c := migrator.MigratorConfig{}

		m, err := migrator.NewMigrator(c)
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
