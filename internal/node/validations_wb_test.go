package node

import (
	"context"
	"testing"

	"github.com/aws/smithy-go/ptr"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_isDrained(t *testing.T) {
	testCases := []struct {
		name string
		pods []corev1.Pod
		want bool
	}{
		{
			name: "drained: no pods",
			want: true,
		},
		{
			name: "drained: all pods are daemonsets",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind:       "DaemonSet",
								Controller: ptr.Bool(true),
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "not drained",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
				},
			},
			want: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isDrained(tc.pods)).To(Equal(tc.want))
		})
	}
}

func Test_getNode(t *testing.T) {
	nodeName := "test"
	testCases := []struct {
		name            string
		node            *corev1.Node
		setupFakeClient func() kubernetes.Interface
		err             error
	}{
		{
			name: "success",
			setupFakeClient: func() kubernetes.Interface {
				client := fake.NewSimpleClientset()
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
				}
				_, _ = client.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
				return client
			},
			err: nil,
		},
		{
			name: "consecutive errors",
			setupFakeClient: func() kubernetes.Interface {
				return fake.NewSimpleClientset()
			},
			err: errors.NewNotFound(schema.GroupResource{Group: "", Resource: "nodes"}, nodeName),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			client := tc.setupFakeClient()

			node, err := getNode(context.Background(), nodeName, client)

			if tc.err != nil {
				g.Expect(err).To(MatchError(tc.err))
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(node).ToNot(BeNil())
				g.Expect(node.Name).To(Equal(nodeName))
			}
		})
	}
}
