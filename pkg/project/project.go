package project

var (
	description        = "The etcd-cluster-migrator will migrate 1 node etcd to 3 node cluster for HA master tenant cluster."
	gitSHA             = "n/a"
	name        string = "etcd-cluster-migrator"
	source      string = "https://github.com/giantswarm/etcd-cluster-migrator"
	version            = "v1.0.2"
)

func Description() string {
	return description
}

func GitSHA() string {
	return gitSHA
}

func Name() string {
	return name
}

func Source() string {
	return source
}

func Version() string {
	return version
}
