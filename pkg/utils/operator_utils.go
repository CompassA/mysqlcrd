/*
 * @Author: Tomato
 * @Date: 2026-04-23 21:58:33
 */
package utils

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// --- Deployment ---
func DeploymentReady(deploy *appsv1.Deployment) (bool, string) {
	for _, cond := range deploy.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			if cond.Status == corev1.ConditionTrue {
				return true, ""
			}
			return false, cond.Message
		}
		// 捕捉失败进展
		if cond.Type == appsv1.DeploymentProgressing &&
			cond.Status == corev1.ConditionFalse &&
			cond.Reason == "ProgressDeadlineExceeded" {
			return false, cond.Message
		}
	}
	return false, "deployment conditions not yet observed"
}

// --- StatefulSet ---
func StatefulSetReady(sts *appsv1.StatefulSet) (bool, string) {
	if sts.Status.Replicas != sts.Status.ReadyReplicas {
		return false, fmt.Sprintf("replicas %d, ready %d", sts.Status.Replicas, sts.Status.ReadyReplicas)
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType &&
		sts.Spec.Replicas != nil && sts.Status.UpdatedReplicas != *sts.Spec.Replicas {
		return false, "updating"
	}
	if sts.Generation > sts.Status.ObservedGeneration {
		return false, "spec changes not yet processed"
	}
	return true, ""
}

// --- Pod ---
func PodReady(pod *corev1.Pod) (bool, string) {
	if pod.Status.Phase == corev1.PodRunning {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true, ""
			}
		}
	}
	if pod.Status.Phase == corev1.PodFailed {
		return false, pod.Status.Message
	}
	return false, fmt.Sprintf("pod phase %s", pod.Status.Phase)
}

// --- PVC ---
func PvcReady(pvc *corev1.PersistentVolumeClaim) (bool, string) {
	if pvc.Status.Phase == corev1.ClaimBound {
		return true, ""
	}
	return false, fmt.Sprintf("pvc phase %s", pvc.Status.Phase)
}

// --- Job ---
func JobReady(job *batchv1.Job) (bool, string) {
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return true, ""
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return false, cond.Message
		}
	}
	return false, "job still running"
}
