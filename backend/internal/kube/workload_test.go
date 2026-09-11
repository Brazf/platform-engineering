package kube_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func sample() *entity.Container {
	return &entity.Container{
		ID: uuid.New(), Name: "meu-pg", Image: "postgres:16",
		CPUMillis: 500, MemoryMB: 512, Port: 5432, Protocol: "tcp", NodePort: 31234,
		Env: entity.EnvMap{"POSTGRES_PASSWORD": "x"}, PersistentGB: 5,
		MountPath: "/var/lib/postgresql/data", Namespace: "u-abc12345",
	}
}

func TestApply_TCPWithVolume(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewWithClientset(cs)
	ctx := context.Background()
	c := sample()

	require.NoError(t, rt.Apply(ctx, c, ""))
	require.NoError(t, rt.Apply(ctx, c, ""), "idempotente")

	dep, err := cs.AppsV1().Deployments(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	require.NoError(t, err)
	ctr := dep.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "postgres:16", ctr.Image)
	assert.Equal(t, "500m", ctr.Resources.Limits.Cpu().String())
	assert.Equal(t, "512Mi", ctr.Resources.Limits.Memory().String())
	assert.Equal(t, "meu-pg-env", ctr.EnvFrom[0].SecretRef.Name)
	assert.Equal(t, "/var/lib/postgresql/data", ctr.VolumeMounts[0].MountPath)

	svc, err := cs.CoreV1().Services(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeNodePort, svc.Spec.Type)
	assert.Equal(t, int32(31234), svc.Spec.Ports[0].NodePort)

	_, err = cs.CoreV1().PersistentVolumeClaims(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = cs.NetworkingV1().Ingresses(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	assert.Error(t, err, "tcp não cria ingress")
}

func TestApply_HTTPWithIngress(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewWithClientset(cs)
	c := sample()
	c.Name, c.Protocol, c.Port, c.NodePort, c.PersistentGB = "site", "http", 80, 0, 0

	require.NoError(t, rt.Apply(context.Background(), c, "lab.example.edu"))
	ing, err := cs.NetworkingV1().Ingresses(c.Namespace).Get(context.Background(), "site", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "site-u-abc12345.lab.example.edu", ing.Spec.Rules[0].Host)
	svc, _ := cs.CoreV1().Services(c.Namespace).Get(context.Background(), "site", metav1.GetOptions{})
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	_, err = cs.CoreV1().PersistentVolumeClaims(c.Namespace).Get(context.Background(), "site", metav1.GetOptions{})
	assert.Error(t, err, "sem volume")
}

func TestApply_NoPortNoService(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewWithClientset(cs)
	c := sample()
	c.Name, c.Port, c.NodePort, c.Command = "shell", 0, 0, "sleep infinity"
	require.NoError(t, rt.Apply(context.Background(), c, "lab.example.edu"))
	_, err := cs.CoreV1().Services(c.Namespace).Get(context.Background(), "shell", metav1.GetOptions{})
	assert.Error(t, err)
	dep, _ := cs.AppsV1().Deployments(c.Namespace).Get(context.Background(), "shell", metav1.GetOptions{})
	assert.Equal(t, []string{"/bin/sh", "-c", "sleep infinity"}, dep.Spec.Template.Spec.Containers[0].Command)
}

func TestDelete_RemovesAll(t *testing.T) {
	cs := fake.NewClientset()
	rt := kube.NewWithClientset(cs)
	c := sample()
	require.NoError(t, rt.Apply(context.Background(), c, ""))
	require.NoError(t, rt.Delete(context.Background(), c.Namespace, c.Name))
	require.NoError(t, rt.Delete(context.Background(), c.Namespace, c.Name), "não existe → sem erro")
	_, err := cs.AppsV1().Deployments(c.Namespace).Get(context.Background(), "meu-pg", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestStatus_FromPods(t *testing.T) {
	cs := fake.NewClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a-1", Namespace: "ns", Labels: map[string]string{"app": "a", "app.kubernetes.io/managed-by": "proxlab"}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b-1", Namespace: "ns", Labels: map[string]string{"app": "b", "app.kubernetes.io/managed-by": "proxlab"}},
			Status: corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "not found"}}}}}},
	)
	rt := kube.NewWithClientset(cs)
	st, err := rt.Status(context.Background(), "ns", []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, repository.ContainerStatus{Phase: "running"}, st["a"])
	assert.Equal(t, "failed", st["b"].Phase)
	assert.Contains(t, st["b"].Message, "ImagePullBackOff")
	assert.Equal(t, "pending", st["c"].Phase, "sem pod ainda → pending")
}
