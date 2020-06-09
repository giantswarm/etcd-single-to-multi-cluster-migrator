package migrator

import (
	"fmt"
	"time"

	"github.com/giantswarm/microerror"
	batchapiv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/etcd-cluster-migrator/pkg/project"
)

const (
	runCommandConfigMap     = "etcd-cluster-migrator-cm"
	runCommandDockerImage   = "giantswarm/alpine:3.11.6"
	runCommandNamespace     = apismetav1.NamespaceSystem
	runCommandPriorityClass = "system-cluster-critical"
	runCommandSAName        = "etcd-cluster-migrator-cmd"
	runCommandVolume        = "command-volume"

	nsenterCommand = "nsenter -t 1 -m -u -n -i -- "

	waitJobCompleted = time.Second * 2
)

// runCommandsOnNode will execute command list on the specified node in the host namespace.
func (m *Migrator) runCommandsOnNode(nodeName string, commands []string) error {
	// configmap for the job where commands will be stored in a single bash file
	{
		cm := buildConfigMapFile(commands)
		// ensure there is no configmap present
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
	// run command on the node
	{
		job := buildCommandJob(nodeName, m.dockerRegistry)
		err := m.k8sClient.BatchV1().Jobs(runCommandNamespace).Delete(job.Name, &apismetav1.DeleteOptions{})
		if k8serrors.IsNotFound(err) {
			// It is fine as its is just safe check before creating.
		} else if err != nil {
			return microerror.Mask(err)
		}

		job, err = m.k8sClient.BatchV1().Jobs(runCommandNamespace).Create(job)
		if err != nil {
			return microerror.Mask(err)
		}

		for {
			fmt.Printf("Waiting for job %s to be completed\n", job.Name)
			time.Sleep(waitJobCompleted)

			job, err := m.k8sClient.BatchV1().Jobs(runCommandNamespace).Get(job.Name, apismetav1.GetOptions{})
			if err != nil {
				return microerror.Mask(err)
			}

			if isJobCompleted(job) {
				fmt.Printf("Job %s was completed.\n", job.Name)

				err := m.k8sClient.BatchV1().Jobs(runCommandNamespace).Delete(job.Name, &apismetav1.DeleteOptions{})
				if err != nil {
					return microerror.Mask(err)
				}
				break
			}
		}
	}

	return nil
}

func isJobCompleted(j *batchapiv1.Job) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == "Complete" && c.Status == apiv1.ConditionTrue {
			return true
		}
	}
	return false
}

// buildCommandJob return job that will execute commands on a node in host namespace.
// The executed file is taken from configmap which is mounted to the pod.
func buildCommandJob(nodeName string, dockerRegistry string) *batchapiv1.Job {
	activeDeadlineSeconds := int64(120)
	backOffLimit := int32(10)
	completions := int32(1)
	privileged := true
	priority := int32(2000000000)
	parallelism := int32(1)
	jobName := fmt.Sprintf("%s-command", project.Name())
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
								Privileged:               &privileged,
								AllowPrivilegeEscalation: &privileged,
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
						{
							Key:    "node-role.kubernetes.io/master",
							Effect: "NoSchedule",
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

// buildConfigMapFile return configmap which has content of the bash file which has the commands.
// This configmap should be used as volume for the pod where it will be executed.
func buildConfigMapFile(cmds []string) *apiv1.ConfigMap {
	configMapContent := `#/bin/bash
set -xe
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
