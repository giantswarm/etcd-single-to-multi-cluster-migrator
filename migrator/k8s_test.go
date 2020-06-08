package migrator

import (
	"strconv"
	"testing"

	v1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Controller_Resource_TCCPN_Template_Render(t *testing.T) {
	testCases := []struct {
		name            string
		nodes           []v1.Node
		sortedNodeNames []string
	}{
		{
			name: "case 0:  ordered nodes",
			nodes: []v1.Node{
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							labelMasterID: "1",
						},
					},
				},
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							labelMasterID: "2",
						},
					},
				},
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-3",
						Labels: map[string]string{
							labelMasterID: "3",
						},
					},
				},
			},
			sortedNodeNames: []string{"node-1", "node-2", "node-3"},
		},
		{
			name: "case 1:  not ordered nodes",
			nodes: []v1.Node{
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-3",
						Labels: map[string]string{
							labelMasterID: "3",
						},
					},
				},
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							labelMasterID: "1",
						},
					},
				},
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							labelMasterID: "2",
						},
					},
				},
			},
			sortedNodeNames: []string{"node-1", "node-2", "node-3"},
		},
		{
			name: "case 2: not ordered nodes",
			nodes: []v1.Node{
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-3",
						Labels: map[string]string{
							labelMasterID: "3",
						},
					},
				},
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							labelMasterID: "2",
						},
					},
				},
				{
					ObjectMeta: apismetav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							labelMasterID: "1",
						},
					},
				},
			},
			sortedNodeNames: []string{"node-1", "node-2", "node-3"},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			nodeNames := getNodeNames(tc.nodes)

			for i := 0; i < len(nodeNames); i++ {
				if nodeNames[i] != tc.sortedNodeNames[i] {
					t.Fatalf("sorted nodes are not equal")
				}

			}
		})
	}
}
