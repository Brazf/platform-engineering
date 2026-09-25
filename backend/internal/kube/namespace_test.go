package kube_test

import (
	"context"
	"testing"

	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureNamespace_CreatesQuotaLimitRangeAndPolicy(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewWithClientset(cs)
	ctx := context.Background()
	q := repository.ResourceQuotaSpec{CPUMillis: 2000, MemoryMB: 4096, Pods: 5, StorageGB: 20}

	require.NoError(t, rt.EnsureNamespace(ctx, "u-abc12345", q))
	require.NoError(t, rt.EnsureNamespace(ctx, "u-abc12345", q), "segunda chamada é idempotente")

	ns, err := cs.CoreV1().Namespaces().Get(ctx, "u-abc12345", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "baseline", ns.Labels["pod-security.kubernetes.io/enforce"])
	assert.Equal(t, "proxlab", ns.Labels["app.kubernetes.io/managed-by"])

	rq, err := cs.CoreV1().ResourceQuotas("u-abc12345").Get(ctx, "proxlab-quota", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "2", rq.Spec.Hard.Cpu().String())
	assert.Equal(t, "4Gi", rq.Spec.Hard.Memory().String())
	assert.Equal(t, "5", rq.Spec.Hard.Pods().String())
	assert.Equal(t, "20Gi", rq.Spec.Hard.Name("requests.storage", "").String())

	_, err = cs.CoreV1().LimitRanges("u-abc12345").Get(ctx, "proxlab-defaults", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = cs.NetworkingV1().NetworkPolicies("u-abc12345").Get(ctx, "proxlab-isolation", metav1.GetOptions{})
	require.NoError(t, err)
}

func TestDeleteNamespace_RemovesAndIsIdempotent(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewWithClientset(cs)
	ctx := context.Background()
	require.NoError(t, rt.EnsureNamespace(ctx, "u-abc12345", repository.ResourceQuotaSpec{CPUMillis: 1000, MemoryMB: 1024, Pods: 1, StorageGB: 1}))

	require.NoError(t, rt.DeleteNamespace(ctx, "u-abc12345"))
	require.NoError(t, rt.DeleteNamespace(ctx, "u-abc12345"), "namespace inexistente não é erro")

	_, err := cs.CoreV1().Namespaces().Get(ctx, "u-abc12345", metav1.GetOptions{})
	assert.Error(t, err)
}
