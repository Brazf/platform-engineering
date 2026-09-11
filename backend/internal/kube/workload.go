package kube

import (
	"context"
	"fmt"

	"github.com/proxlab/backend/internal/domain/entity"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	applynetv1 "k8s.io/client-go/applyconfigurations/networking/v1"
)

func secretName(name string) string { return name + "-env" }

func labels(name string) map[string]string {
	return map[string]string{"app": name, "app.kubernetes.io/managed-by": "proxlab"}
}

// IngressHost é o host público de um container HTTP: <name>-<ns>.<domain>.
func IngressHost(name, ns, domain string) string { return name + "-" + ns + "." + domain }

func (r *Runtime) Apply(ctx context.Context, c *entity.Container, ingressDomain string) error {
	ns, name := c.Namespace, c.Name

	sec := applycorev1.Secret(secretName(name), ns).WithLabels(labels(name)).WithStringData(c.Env)
	if _, err := r.cs.CoreV1().Secrets(ns).Apply(ctx, sec, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar secret: %w", err)
	}

	if c.PersistentGB > 0 {
		pvc := applycorev1.PersistentVolumeClaim(name, ns).WithLabels(labels(name)).
			WithSpec(applycorev1.PersistentVolumeClaimSpec().
				WithAccessModes(corev1.ReadWriteOnce).
				WithResources(applycorev1.VolumeResourceRequirements().WithRequests(corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(int64(c.PersistentGB)<<30, resource.BinarySI),
				})))
		if _, err := r.cs.CoreV1().PersistentVolumeClaims(ns).Apply(ctx, pvc, applyOpts); err != nil {
			return fmt.Errorf("erro ao aplicar pvc: %w", err)
		}
	}

	res := corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(c.CPUMillis), resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(int64(c.MemoryMB)<<20, resource.BinarySI),
	}
	ctr := applycorev1.Container().WithName(name).WithImage(c.Image).
		WithEnvFrom(applycorev1.EnvFromSource().WithSecretRef(applycorev1.SecretEnvSource().WithName(secretName(name)))).
		WithResources(applycorev1.ResourceRequirements().WithLimits(res).WithRequests(res)).
		WithSecurityContext(applycorev1.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithCapabilities(applycorev1.Capabilities().WithDrop("ALL")).
			WithSeccompProfile(applycorev1.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault)))
	if c.Port > 0 {
		ctr = ctr.WithPorts(applycorev1.ContainerPort().WithContainerPort(int32(c.Port)))
	}
	if c.Command != "" {
		ctr = ctr.WithCommand("/bin/sh", "-c", c.Command)
	}
	// VolumeMounts antes de WithContainers: o builder copia o container por valor.
	if c.PersistentGB > 0 {
		ctr = ctr.WithVolumeMounts(applycorev1.VolumeMount().WithName("data").WithMountPath(c.MountPath))
	}
	podSpec := applycorev1.PodSpec().WithContainers(ctr).
		WithSecurityContext(applycorev1.PodSecurityContext().WithRunAsNonRoot(false)).
		WithAutomountServiceAccountToken(false)
	if c.PersistentGB > 0 {
		podSpec = podSpec.WithVolumes(applycorev1.Volume().WithName("data").
			WithPersistentVolumeClaim(applycorev1.PersistentVolumeClaimVolumeSource().WithClaimName(name)))
	}
	dep := applyappsv1.Deployment(name, ns).WithLabels(labels(name)).
		WithSpec(applyappsv1.DeploymentSpec().WithReplicas(1).
			WithSelector(applymetav1.LabelSelector().WithMatchLabels(map[string]string{"app": name})).
			WithStrategy(applyappsv1.DeploymentStrategy().WithType("Recreate")).
			WithTemplate(applycorev1.PodTemplateSpec().WithLabels(labels(name)).WithSpec(podSpec)))
	if _, err := r.cs.AppsV1().Deployments(ns).Apply(ctx, dep, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar deployment: %w", err)
	}

	if c.Port == 0 {
		return nil
	}

	useIngress := c.Protocol == entity.ProtocolHTTP && ingressDomain != "" && c.NodePort == 0
	port := applycorev1.ServicePort().WithName("main").WithPort(int32(c.Port)).
		WithTargetPort(intstr.FromInt32(int32(c.Port))).WithProtocol(corev1.ProtocolTCP)
	svcType := corev1.ServiceTypeClusterIP
	if !useIngress {
		svcType = corev1.ServiceTypeNodePort
		port = port.WithNodePort(int32(c.NodePort))
	}
	svc := applycorev1.Service(name, ns).WithLabels(labels(name)).
		WithSpec(applycorev1.ServiceSpec().WithType(svcType).
			WithSelector(map[string]string{"app": name}).WithPorts(port))
	if _, err := r.cs.CoreV1().Services(ns).Apply(ctx, svc, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar service: %w", err)
	}

	if useIngress {
		pt := netv1.PathTypePrefix
		ing := applynetv1.Ingress(name, ns).WithLabels(labels(name)).
			WithSpec(applynetv1.IngressSpec().WithRules(
				applynetv1.IngressRule().WithHost(IngressHost(name, ns, ingressDomain)).
					WithHTTP(applynetv1.HTTPIngressRuleValue().WithPaths(
						applynetv1.HTTPIngressPath().WithPath("/").WithPathType(pt).
							WithBackend(applynetv1.IngressBackend().WithService(
								applynetv1.IngressServiceBackend().WithName(name).
									WithPort(applynetv1.ServiceBackendPort().WithNumber(int32(c.Port)))))))))
		if _, err := r.cs.NetworkingV1().Ingresses(ns).Apply(ctx, ing, applyOpts); err != nil {
			return fmt.Errorf("erro ao aplicar ingress: %w", err)
		}
	}
	return nil
}

// Delete remove todos os objetos do container; ausência não é erro.
func (r *Runtime) Delete(ctx context.Context, ns, name string) error {
	del := metav1.DeleteOptions{}
	ignore := func(err error) error {
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := ignore(r.cs.NetworkingV1().Ingresses(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover ingress: %w", err)
	}
	if err := ignore(r.cs.CoreV1().Services(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover service: %w", err)
	}
	if err := ignore(r.cs.AppsV1().Deployments(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover deployment: %w", err)
	}
	if err := ignore(r.cs.CoreV1().Secrets(ns).Delete(ctx, secretName(name), del)); err != nil {
		return fmt.Errorf("erro ao remover secret: %w", err)
	}
	if err := ignore(r.cs.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover pvc: %w", err)
	}
	return nil
}
