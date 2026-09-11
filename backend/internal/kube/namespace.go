package kube

import (
	"context"
	"fmt"

	"github.com/proxlab/backend/internal/domain/repository"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	applynetv1 "k8s.io/client-go/applyconfigurations/networking/v1"
)

var applyOpts = metav1.ApplyOptions{FieldManager: fieldManager, Force: true}

func (r *Runtime) EnsureNamespace(ctx context.Context, ns string, q repository.ResourceQuotaSpec) error {
	nsCfg := applycorev1.Namespace(ns).WithLabels(map[string]string{
		"app.kubernetes.io/managed-by":               "proxlab",
		"pod-security.kubernetes.io/enforce":         "restricted",
		"pod-security.kubernetes.io/enforce-version": "latest",
	})
	if _, err := r.cs.CoreV1().Namespaces().Apply(ctx, nsCfg, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar namespace: %w", err)
	}

	hard := corev1.ResourceList{
		corev1.ResourceCPU:             *resource.NewMilliQuantity(int64(q.CPUMillis), resource.DecimalSI),
		corev1.ResourceMemory:          *resource.NewQuantity(int64(q.MemoryMB)<<20, resource.BinarySI),
		corev1.ResourcePods:            *resource.NewQuantity(int64(q.Pods), resource.DecimalSI),
		corev1.ResourceRequestsStorage: *resource.NewQuantity(int64(q.StorageGB)<<30, resource.BinarySI),
	}
	rq := applycorev1.ResourceQuota("proxlab-quota", ns).
		WithSpec(applycorev1.ResourceQuotaSpec().WithHard(hard))
	if _, err := r.cs.CoreV1().ResourceQuotas(ns).Apply(ctx, rq, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar resourcequota: %w", err)
	}

	def := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("250m"),
		corev1.ResourceMemory: resource.MustParse("256Mi"),
	}
	lr := applycorev1.LimitRange("proxlab-defaults", ns).
		WithSpec(applycorev1.LimitRangeSpec().WithLimits(
			applycorev1.LimitRangeItem().WithType(corev1.LimitTypeContainer).
				WithDefault(def).WithDefaultRequest(def)))
	if _, err := r.cs.CoreV1().LimitRanges(ns).Apply(ctx, lr, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar limitrange: %w", err)
	}

	// Ingress: só do próprio namespace e do Traefik (kube-system). Egress: liberado.
	np := applynetv1.NetworkPolicy("proxlab-isolation", ns).
		WithSpec(applynetv1.NetworkPolicySpec().
			WithPodSelector(applymetav1.LabelSelector()).
			WithPolicyTypes(netv1.PolicyTypeIngress).
			WithIngress(
				applynetv1.NetworkPolicyIngressRule().WithFrom(
					applynetv1.NetworkPolicyPeer().WithPodSelector(applymetav1.LabelSelector()),
					applynetv1.NetworkPolicyPeer().WithNamespaceSelector(
						applymetav1.LabelSelector().WithMatchLabels(map[string]string{"kubernetes.io/metadata.name": "kube-system"})),
				),
			))
	if _, err := r.cs.NetworkingV1().NetworkPolicies(ns).Apply(ctx, np, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar networkpolicy: %w", err)
	}
	return nil
}
