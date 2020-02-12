/*
Copyright 2019 The Caicloud Authors.

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

package api

import (
	v1 "k8s.io/api/core/v1"
)

func getTaskStatus(pod *v1.Pod) TaskStatus {
	switch pod.Status.Phase {
	case v1.PodRunning:
		if pod.DeletionTimestamp != nil {
			return Releasing
		}
		return Running
	case v1.PodPending:
		if pod.DeletionTimestamp != nil {
			return Releasing
		}
		if len(pod.Spec.NodeName) == 0 {
			if isPodUnschedulable(pod.Status.Conditions) {
				return Unschedulable
			}
			return Pending
		}
		return Bound
	case v1.PodUnknown:
		return Unknown
	case v1.PodSucceeded:
		return Succeeded
	case v1.PodFailed:
		return Failed
	}
	return Unknown
}

func isPodUnschedulable(conditions []v1.PodCondition) bool {
	for _, cond := range conditions {
		if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse &&
			cond.Reason == v1.PodReasonUnschedulable {
			return true
		}
	}
	return false
}

// AllocatedStatus checks whether the tasks has AllocatedStatus
func AllocatedStatus(status TaskStatus) bool {
	switch status {
	case Bound, Running:
		return true
	default:
		return false
	}
}
