package main

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultDrainTaintName = "ektopistis.io/drain"
)

func getFullPodName(pod *corev1.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

type DrainOptions struct {
	DrainTaintName string
}

type nodeDrainer struct {
	client  client.Client
	options DrainOptions
}

func NewNodeDrainer(client client.Client, options *DrainOptions) reconcile.Reconciler {
	result := &nodeDrainer{client: client, options: *options}
	if result.options.DrainTaintName == "" {
		result.options.DrainTaintName = defaultDrainTaintName
	}
	return result
}

func (r *nodeDrainer) haveUnschedulableNodes(ctx context.Context) (bool, error) {
	nodes := corev1.NodeList{}
	if err := r.client.List(ctx, &nodes); err != nil {
		return false, err
	}
	for _, node := range nodes.Items {
		if node.Spec.Unschedulable {
			return true, nil
		}
	}
	return false, nil
}

func (r *nodeDrainer) nodeHasDrainingTaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == r.options.DrainTaintName && taint.Effect == "NoSchedule" {
			return true
		}
	}
	return false
}

func (r *nodeDrainer) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	node := corev1.Node{}
	err := r.client.Get(ctx, request.NamespacedName, &node)
	if errors.IsNotFound(err) {
		log.Error(nil, "Could not find Node")
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could nod fetch node: %+v", err)
	}

	log.V(2).Info("Reconciling Node")

	if !r.nodeHasDrainingTaint(&node) {
		return reconcile.Result{}, nil
	}

	if node.Spec.Unschedulable {
		return r.evictNodePods(ctx, &node)
	} else {
		haveUnschedulableNodes, err := r.haveUnschedulableNodes(ctx)
		if err != nil {
			log.Error(err, "Unable to list nodes")
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if haveUnschedulableNodes {
			log.Info("Have other cordoned nodes, will postpone cordoning this one")
			return reconcile.Result{RequeueAfter: 15 * time.Minute}, nil
		}
		log.Info("Cordoning node")
		node.Spec.Unschedulable = true

		if err := r.client.Update(ctx, &node); err != nil {
			log.Error(err, "Could not update node", "reason", "marking unschedulable")
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *nodeDrainer) evictNodePods(ctx context.Context, node *corev1.Node) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	pods := corev1.PodList{}
	err := r.client.List(ctx, &pods)
	if err != nil {
		log.Error(err, "Could not retrive pod list")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Pending" {
			log.V(2).Info("Found pending pod, postponing evictions on node", "pod", pod.Name)
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	podsUnderEviction := make([]*corev1.Pod, 0, len(pods.Items))
	errorCount := 0
	for _, pod := range pods.Items {
		if pod.Spec.NodeName != node.Name {
			continue
		}
		if len(pod.OwnerReferences) > 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
			continue
		}
		podName := getFullPodName(&pod)
		log.Info("Evicting pod from node", "pod", podName)
		err := r.client.SubResource("eviction").Create(
			ctx, &pod, &policyv1.Eviction{DeleteOptions: metav1.NewDeleteOptions(45)})
		if errors.IsNotFound(err) {
			continue
		}
		if errors.IsTooManyRequests(err) {
			return reconcile.Result{RequeueAfter: 10 * time.Second}, err
		}
		if err != nil {
			log.Error(err, "Unable to evict pod", "pod", podName)
			errorCount += 1
		}
		podsUnderEviction = append(podsUnderEviction, &pod)
	}
	if errorCount > 0 {
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	pollInterval := 10 * time.Second
	pollTimeout := 20 * time.Minute
	remainingPods := make([]*corev1.Pod, 0, len(pods.Items))
	err = wait.PollUntilContextTimeout(
		ctx,
		pollInterval,
		pollTimeout,
		true,
		func(ctx context.Context) (bool, error) {
			remainingPods = remainingPods[:0]
			for _, pod := range podsUnderEviction {
				actualPod := corev1.Pod{}
				err := r.client.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, &actualPod)
				if errors.IsNotFound(err) || (err == nil && actualPod.Spec.NodeName != node.Name) {
					continue // The pod has left the building.
				}
				if err != nil {
					log.Error(err, "Unable to retrieve pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
				}
				remainingPods = append(remainingPods, pod)
			}
			return len(remainingPods) == 0, nil
		})
	if err != nil {
		log.Error(err, "Error waiting for pod eviction")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	if len(remainingPods) > 0 {
		log.Error(nil, "Pods remain on node after timeout", "pods", remainingPods)
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	return reconcile.Result{}, nil
}
