package migrator

import (
	"strconv"
	"testing"
)

func Test_initialCluster(t *testing.T) {
	testCases := []struct {
		name                   string
		startingIndex          int
		baseDomain             string
		nodesCount             int
		expectedInitialCluster string
	}{
		{
			name:                   "case 0: initial cluster for second node with starting index 0",
			startingIndex:          0,
			baseDomain:             "clusterID.gigantic.io",
			nodesCount:             2,
			expectedInitialCluster: "etcd0=https:\\/\\/etcd0.clusterID.gigantic.io:2380,etcd1=https:\\/\\/etcd1.clusterID.gigantic.io:2380",
		},
		{
			name:                   "case 1: initial cluster for third node with starting index 0",
			startingIndex:          0,
			baseDomain:             "clusterID.gigantic.io",
			nodesCount:             3,
			expectedInitialCluster: "etcd0=https:\\/\\/etcd0.clusterID.gigantic.io:2380,etcd1=https:\\/\\/etcd1.clusterID.gigantic.io:2380,etcd2=https:\\/\\/etcd2.clusterID.gigantic.io:2380",
		},
		{
			name:                   "case 2: initial cluster for second node with starting index 1",
			startingIndex:          1,
			baseDomain:             "clusterID.gigantic.io",
			nodesCount:             2,
			expectedInitialCluster: "etcd1=https:\\/\\/etcd1.clusterID.gigantic.io:2380,etcd2=https:\\/\\/etcd2.clusterID.gigantic.io:2380",
		},
		{
			name:                   "case 3: initial cluster for third node with starting index 1",
			startingIndex:          1,
			baseDomain:             "clusterID.gigantic.io",
			nodesCount:             3,
			expectedInitialCluster: "etcd1=https:\\/\\/etcd1.clusterID.gigantic.io:2380,etcd2=https:\\/\\/etcd2.clusterID.gigantic.io:2380,etcd3=https:\\/\\/etcd3.clusterID.gigantic.io:2380",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			initialCluster := initialCluster(tc.startingIndex, tc.baseDomain, tc.nodesCount)

			if initialCluster != tc.expectedInitialCluster {
				t.Fatalf("%s : expected initial cluster \n%s\nbut got \n%s", tc.name, tc.expectedInitialCluster, initialCluster)
			}
		})
	}
}
