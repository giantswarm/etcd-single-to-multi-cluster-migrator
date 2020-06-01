package migrator

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/giantswarm/microerror"
	batchapiv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/etcd-cluster-migrator/pkg/project"
)

const (
	runCommandConfigMap     = "etcd-cluster-migrator-cm"
	runCommandDockerImage   = "giantswarm/alpine:3.11.6"
	runCommandNamespace     = apismetav1.NamespaceSystem
	runCommandPriorityClass = "system-cluster-critical"
	runCommandSAName        = "etcd-cluster-migrator-job"
	runCommandVolume        = "command-volume"
)

var nsenterCommand = "nsenter -t 1 -m -u -n -i "

func (m *Migrator) runCommandsOnNode(nodeName string, commands []string) error {
	// configmap for the job
	{
		cm := buildConfigMapFile(commands)
		err := m.k8sClient.CoreV1().ConfigMaps(runCommandNamespace).Delete(cm.Name, &apismetav1.DeleteOptions{})
		if k8serrors.IsNotFound(err) {
			// It is fine as its is just safe check before creating.
		} else if err != nil {
			return microerror.Mask(err)
		}

		_, err = m.k8sClient.CoreV1().ConfigMaps(runCommandNamespace).Create(cm)
		if err != nil {
			return microerror.Mask(err)
		}
	}
	// the job for running commands
	{
		job := buildCommandJob(nodeName, m.dockerRegistry)
		err := m.k8sClient.BatchV1().Jobs(runCommandNamespace).Delete(job.Name, &apismetav1.DeleteOptions{})
		if k8serrors.IsNotFound(err) {
			// It is fine as its is just safe check before creating.
		} else if err != nil {
			return microerror.Mask(err)
		}

		_, err = m.k8sClient.BatchV1().Jobs(runCommandNamespace).Create(job)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}

func buildCommandJob(nodeName string, dockerRegistry string) *batchapiv1.Job {
	activeDeadlineSeconds := int64(60)
	backOffLimit := int32(1)
	completions := int32(1)
	privileged := true
	priority := int32(2000000000)
	parallelism := int32(1)
	jobName := fmt.Sprintf("%s-command-%s", project.Name(), nodeName)
	cpu := resource.MustParse("50m")
	memory := resource.MustParse("50Mi")

	j := batchapiv1.Job{
		TypeMeta: apismetav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchapiv1.GroupName,
		},
		ObjectMeta: apismetav1.ObjectMeta{
			Name:      jobName,
			Namespace: runCommandNamespace,
			Labels: map[string]string{
				"app":        jobName,
				"created-by": project.Name(),
			},
		},
		Spec: batchapiv1.JobSpec{
			Parallelism:  &parallelism,
			Completions:  &completions,
			BackoffLimit: &backOffLimit,
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "run-command",
							Image: jobDockerImage(dockerRegistry),
							Resources: apiv1.ResourceRequirements{
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    cpu,
									apiv1.ResourceMemory: memory,
								},
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    cpu,
									apiv1.ResourceMemory: memory,
								},
							},
							SecurityContext: &apiv1.SecurityContext{
								Privileged: &privileged,
							},
							Command: []string{
								"/bin/sh",
								"/data/command.sh",
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      runCommandVolume,
									MountPath: "/data/",
									ReadOnly:  true,
								},
							},
						},
					},
					HostPID: true,
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
					RestartPolicy:      apiv1.RestartPolicyNever,
					Priority:           &priority,
					PriorityClassName:  runCommandPriorityClass,
					ServiceAccountName: runCommandSAName,
					Tolerations: []apiv1.Toleration{
						{
							Key:      "node.kubernetes.io/unschedulable",
							Operator: "Exists",
							Effect:   "NoSchedule",
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: runCommandVolume,
							VolumeSource: apiv1.VolumeSource{
								ConfigMap: &apiv1.ConfigMapVolumeSource{
									LocalObjectReference: apiv1.LocalObjectReference{
										Name: runCommandConfigMap,
									},
								},
							},
						},
					},
				},
			},
			ActiveDeadlineSeconds: &activeDeadlineSeconds,
		},
	}
	return &j
}

func buildConfigMapFile(cmds []string) *apiv1.ConfigMap {
	configMapContent := `#/bin/bash
`
	for _, c := range cmds {
		configMapContent += nsenterCommand + c + "\n"
	}

	cm := &apiv1.ConfigMap{
		TypeMeta: apismetav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: apiv1.GroupName,
		},
		ObjectMeta: apismetav1.ObjectMeta{
			Name: runCommandConfigMap,
			Labels: map[string]string{
				"app":        runCommandConfigMap,
				"created-by": project.Name(),
			},
		},
		Data: map[string]string{
			"command.sh": configMapContent,
		},
	}

	return cm
}

func jobDockerImage(registry string) string {
	return fmt.Sprintf("%s/%s", registry, runCommandDockerImage)
}
