/*
Copyright 2024 The Sage Group plc or its licensors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"testing"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestNodeDrainer(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "NodeDrainer Suite")
}

type mockEvictionClient struct {
	mock.Mock
}

func (c *mockEvictionClient) Get(
	ctx context.Context,
	obj client.Object,
	subResource client.Object,
	opts ...client.SubResourceGetOption,
) error {
	args := c.Called(ctx, obj, subResource, opts)
	return args.Error(0)
}

func (c *mockEvictionClient) Create(
	ctx context.Context,
	obj client.Object,
	subResource client.Object,
	opts ...client.SubResourceCreateOption,
) error {
	args := c.Called(ctx, obj, subResource, opts)
	return args.Error(0)
}

func (c *mockEvictionClient) Update(
	ctx context.Context,
	obj client.Object,
	opts ...client.SubResourceUpdateOption,
) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}

func (c *mockEvictionClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.SubResourcePatchOption,
) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

type fakeClientWithEviction struct {
	client.WithWatch
	eviction mockEvictionClient
}

func (c *fakeClientWithEviction) SubResource(subResource string) client.SubResourceClient {
	if subResource == "eviction" {
		return &c.eviction
	}
	return c.WithWatch.SubResource(subResource)
}

var _ = ginkgo.Describe("NodeDrainer", func() {
	var g gomega.Gomega
	var ctx context.Context
	var nodes []*corev1.Node
	var pods []*corev1.Pod

	var fakeClient *fakeClientWithEviction
	var nodeDrainer reconcile.Reconciler
	var request reconcile.Request

	ginkgo.BeforeEach(func() {
		g = gomega.NewWithT(ginkgo.GinkgoT())
		ctx = context.Background()

		logOptions := zap.Options{}
		flagSet := flag.NewFlagSet("test", flag.PanicOnError)
		logOptions.BindFlags(flagSet)
		err := flagSet.Parse([]string{"--zap-log-level=2"})
		g.Expect(err).ToNot(gomega.HaveOccurred())
		logger := zap.New(zap.UseFlagOptions(&logOptions))
		log.SetLogger(logger)
		log.IntoContext(ctx, logger)
	})

	ginkgo.JustBeforeEach(func() {
		var objects []client.Object
		for _, node := range nodes {
			objects = append(objects, node)
		}
		for _, pod := range pods {
			objects = append(objects, pod)
		}
		fakeClient = &fakeClientWithEviction{
			fake.NewClientBuilder().WithObjects(objects...).Build(),
			mockEvictionClient{},
		}
		nodeDrainer = NewNodeDrainer(fakeClient, &DrainOptions{})
		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: nodes[0].Name,
			},
		}
	})

	ginkgo.When("node is not marked for draining", func() {
		podName := types.NamespacedName{
			Name:      "test-pod",
			Namespace: "test",
		}
		ginkgo.BeforeEach(func() {
			pods = []*corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
				},
			}}
			nodes = []*corev1.Node{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: corev1.NodeSpec{
					Taints:        []corev1.Taint{},
					Unschedulable: false,
				},
			}}
		})

		ginkgo.It("should not make the node unschedulable", func() {
			_, err := nodeDrainer.Reconcile(ctx, request)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			resultNode := corev1.Node{}
			err = fakeClient.Get(ctx, request.NamespacedName, &resultNode)
			g.Expect(err).ToNot(gomega.HaveOccurred(), "Unexpected error in fake client")
			g.Expect(resultNode.Spec.Unschedulable).To(
				gomega.BeFalse(),
				"Non-labeled nodes must not be marked unschedulable",
			)
		})

		ginkgo.It("should not evict node pods", func() {
			fakeClient.eviction.On(
				"Create",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Run(func(args mock.Arguments) {
				ginkgo.Fail("Should not evict pods from non-tainted node")
			})

			_, err := nodeDrainer.Reconcile(ctx, request)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			_, err = nodeDrainer.Reconcile(ctx, request)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			resultPod := corev1.Pod{}
			err = fakeClient.Get(ctx, podName, &resultPod)
			g.Expect(err).ToNot(gomega.HaveOccurred(), "Pods must not be deleted")
		})
	})

	ginkgo.When("node is marked for draining", func() {
		podName := types.NamespacedName{
			Name:      "test-pod",
			Namespace: "test",
		}
		ginkgo.BeforeEach(func() {
			pods = []*corev1.Pod{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
				},
				Spec: corev1.PodSpec{
					NodeName: "test-node",
				},
			}}
			nodes = []*corev1.Node{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{
						Key:    "ektopistis.io/drain",
						Effect: "NoSchedule",
						Value:  "yes",
					}},
					Unschedulable: false,
				},
			}}
		})

		ginkgo.It("should mark node unschedulable", func() {
			_, err := nodeDrainer.Reconcile(ctx, request)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			resultNode := corev1.Node{}
			err = fakeClient.Get(ctx, request.NamespacedName, &resultNode)
			g.Expect(err).ToNot(gomega.HaveOccurred(), "Unexpected error in fake client")
			g.Expect(resultNode.Spec.Unschedulable).To(
				gomega.BeTrue(), "Labeled nodes must be marked unschedulable")
		})

		ginkgo.When("alternative taint name is used", func() {
			ginkgo.BeforeEach(func() {
				nodes[0].Spec.Taints[0].Key = "some-other-taint"
			})

			ginkgo.It("should mark node unschedulable", func() {
				nodeDrainer = NewNodeDrainer(
					fakeClient,
					&DrainOptions{DrainTaintName: "some-other-taint"},
				)

				_, err := nodeDrainer.Reconcile(ctx, request)
				g.Expect(err).ToNot(gomega.HaveOccurred())

				resultNode := corev1.Node{}
				err = fakeClient.Get(ctx, request.NamespacedName, &resultNode)
				g.Expect(err).ToNot(gomega.HaveOccurred(), "Unexpected error in fake client")
				g.Expect(resultNode.Spec.Unschedulable).To(
					gomega.BeTrue(), "Nodes marked for draining must be set unschedulable")
			})
		})

		ginkgo.When("other unschedulable nodes are present", func() {
			ginkgo.BeforeEach(func() {
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other-node",
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{{
							Key:    "ektopistis.io/drain",
							Effect: "NoSchedule",
							Value:  "yes",
						}},
						Unschedulable: true,
					},
				})
			})
			ginkgo.It("should postpone cordoning", func() {
				result, err := nodeDrainer.Reconcile(ctx, request)
				g.Expect(err).ToNot(gomega.HaveOccurred())

				g.Expect(result.RequeueAfter).To(gomega.Equal(15 * time.Minute))

				resultNode := corev1.Node{}
				err = fakeClient.Get(ctx, request.NamespacedName, &resultNode)
				g.Expect(err).ToNot(gomega.HaveOccurred(), "Unexpected error in fake client")
				g.Expect(resultNode.Spec.Unschedulable).To(
					gomega.BeFalse(),
					"When the unschedulabel node is present, should not mark as unschedulable")
			})
		})

		ginkgo.When("pending pods are present", func() {
			podName := types.NamespacedName{
				Name:      "test-pod",
				Namespace: "test",
			}

			ginkgo.BeforeEach(func() {
				pods = append(pods, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pending-pod",
						Namespace: "test",
					},
					Spec: corev1.PodSpec{
						NodeName: "other-node",
					},
					Status: corev1.PodStatus{Phase: "Pending"},
				})
				nodes = append(nodes, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other-node",
					},
				})
			})

			ginkgo.It("should not evict pods", func() {
				fakeClient.eviction.On(
					"Create",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Run(func(args mock.Arguments) {
					ginkgo.Fail("Should not evict pods when pending pods are present")
				}).Return(nil).Once()

				_, err := nodeDrainer.Reconcile(ctx, request)
				g.Expect(err).ToNot(gomega.HaveOccurred())
				_, err = nodeDrainer.Reconcile(ctx, request)
				g.Expect(err).ToNot(gomega.HaveOccurred())

				resultPod := corev1.Pod{}
				err = fakeClient.Get(ctx, podName, &resultPod)
				g.Expect(err).ToNot(gomega.HaveOccurred(), "Pods must not be deleted")
			})
		})

		ginkgo.It("should evict node pods", func() {
			fakeClient.eviction.On(
				"Create",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				obj := args.Get(1).(client.Object)
				err := fakeClient.Delete(ctx, obj)
				g.Expect(err).ToNot(gomega.HaveOccurred(), "Unexpected error in fake client")
			}).Return(nil).Once()

			_, err := nodeDrainer.Reconcile(ctx, request)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			_, err = nodeDrainer.Reconcile(ctx, request)
			g.Expect(err).ToNot(gomega.HaveOccurred())

			resultPod := corev1.Pod{}
			err = fakeClient.Get(ctx, podName, &resultPod)
			g.Expect(err).To(gomega.MatchError(
				"pods \"test-pod\" not found"),
				"Pod must be deleted")
		})

		ginkgo.It("should retry eviction after errors", func() {
			fakeClient.eviction.On(
				"Create",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(errors.New("unexpected EOF")).Once()
			fakeClient.eviction.On(
				"Create",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				obj := args.Get(1).(client.Object)
				err := fakeClient.Delete(ctx, obj)
				g.Expect(err).ToNot(gomega.HaveOccurred(), "Unexpected error in fake client")
			}).Return(nil).Once()

			_, err := nodeDrainer.Reconcile(ctx, request) // Marks unschedulable.
			g.Expect(err).ToNot(gomega.HaveOccurred())
			_, err = nodeDrainer.Reconcile(ctx, request) // Encounters error.
			g.Expect(err).ToNot(gomega.HaveOccurred())
			_, err = nodeDrainer.Reconcile(ctx, request) // Retries.
			g.Expect(err).ToNot(gomega.HaveOccurred())

			resultPod := corev1.Pod{}
			err = fakeClient.Get(ctx, podName, &resultPod)
			g.Expect(err).To(gomega.MatchError(
				"pods \"test-pod\" not found"),
				"Pod must be deleted")
		})
	})
})
