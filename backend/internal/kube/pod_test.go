package kube

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(name string, phase corev1.PodPhase, age time.Duration) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: map[string]string{"app": "web"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-age))},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func TestFindRunningPod_PicksNewestRunning(t *testing.T) {
	cs := fake.NewClientset(
		pod("web-old", corev1.PodRunning, 2*time.Hour),
		pod("web-new", corev1.PodRunning, time.Minute),
		pod("web-pending", corev1.PodPending, 0),
	)
	rt := NewWithClientset(cs)
	p, err := rt.findRunningPod(context.Background(), "ns", "web")
	require.NoError(t, err)
	assert.Equal(t, "web-new", p.Name)
}

func TestFindRunningPod_NoneRunning(t *testing.T) {
	cs := fake.NewClientset(pod("web-pending", corev1.PodPending, 0))
	rt := NewWithClientset(cs)
	_, err := rt.findRunningPod(context.Background(), "ns", "web")
	assert.ErrorIs(t, err, errPodNotRunning)
}
