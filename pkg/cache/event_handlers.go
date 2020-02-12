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

package cache

import (
	"fmt"

	"github.com/angao/coscheduling/pkg/api"
	schedulingv1 "github.com/angao/coscheduling/pkg/apis/scheduling/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// AddPod add pod to scheduler cache
func (sc *schedulerCache) AddPod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert to *v1.Pod: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.addPod(pod); err != nil {
		klog.Errorf("Failed to add pod <%s/%s> into cache: %v", pod.Namespace, pod.Name, err)
		return
	}
	klog.V(4).Infof("Added pod <%s/%s> into cache.", pod.Namespace, pod.Name)
}

func (sc *schedulerCache) UpdatePod(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	err := sc.updatePod(oldPod, newPod)
	if err != nil {
		klog.Errorf("Failed to update pod %v in cache: %v", oldPod.Name, err)
		return
	}

	klog.V(4).Infof("Updated pod <%s/%s> in cache.", oldPod.Namespace, oldPod.Name)
}

// DeletePod delete pod from scheduler cache
func (sc *schedulerCache) DeletePod(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			klog.Errorf("Cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1.Pod: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.deletePod(pod); err != nil {
		klog.Errorf("Failed to delete pod <%s/%s> from cache: %v", pod.Namespace, pod.Name, err)
		return
	}

	klog.V(4).Infof("Deleted pod <%s/%s> from cache.", pod.Namespace, pod.Name)
}

func (sc *schedulerCache) AddPodGroup(obj interface{}) {
	podGroup, ok := obj.(*schedulingv1.PodGroup)
	if !ok {
		klog.Errorf("Cannot convert to *schedulingv1.PodGroup: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.setPodGroup(podGroup)
	klog.V(4).Infof("Added PodGroup <%s/%s> into cache", podGroup.Namespace, podGroup.Name)
}

func (sc *schedulerCache) UpdatePodGroup(oldObj, newObj interface{}) {
	oldPg, ok := oldObj.(*schedulingv1.PodGroup)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *schedulingv1.PodGroup: %v", oldObj)
		return
	}

	newPg, ok := newObj.(*schedulingv1.PodGroup)
	if !ok {
		klog.Errorf("Cannot convert newObj to *schedulingv1.PodGroup: %v", oldObj)
		return
	}

	if oldPg.ResourceVersion == newPg.ResourceVersion {
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.setPodGroup(newPg)
	klog.V(4).Infof("Updated PodGroup <%s/%s> in cache", newPg.Namespace, newPg.Name)
}

func (sc *schedulerCache) DeletePodGroup(obj interface{}) {
	var podGroup *schedulingv1.PodGroup
	switch t := obj.(type) {
	case *schedulingv1.PodGroup:
		podGroup = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		podGroup, ok = t.Obj.(*schedulingv1.PodGroup)
		if !ok {
			klog.Errorf("Cannot convert to *schedulingv1.PodGroup: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *schedulingv1.PodGroup: %v", t)
		return
	}
	jobID := getJobID(podGroup)

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.deletePodGroup(jobID); err != nil {
		klog.Errorf("Failed to delete PodGroup <%s/%s> from cache: %v", podGroup.Namespace, podGroup.Name, err)
	}
	klog.V(4).Infof("Deleted PodGroup <%s/%s> in cache", podGroup.Namespace, podGroup.Name)
}

func (sc *schedulerCache) addPod(pod *v1.Pod) error {
	task := api.NewTask(pod)
	return sc.addTask(task)
}

func (sc *schedulerCache) addTask(task *api.Task) error {
	job := sc.getOrCreateJob(task)
	if job != nil {
		job.AddTask(task)
	}
	return nil
}

func (sc *schedulerCache) getOrCreateJob(task *api.Task) *api.Job {
	if len(task.Job) == 0 {
		if task.Pod.Spec.SchedulerName != sc.schedulerName {
			klog.V(4).Infof("Pod %s/%s will not not scheduled by %s, skip creating PodGroup and Job for it",
				task.Pod.Namespace, task.Pod.Name, sc.schedulerName)
		}
		return nil
	}

	if _, found := sc.jobs[task.Job]; !found {
		sc.jobs[task.Job] = api.NewJob(task.Job)
	}

	return sc.jobs[task.Job]
}

func (sc *schedulerCache) updatePod(oldPod *v1.Pod, newPod *v1.Pod) error {
	if err := sc.deletePod(oldPod); err != nil {
		return err
	}

	err := sc.addPod(newPod)
	if err != nil {
		return err
	}
	return nil
}

// Assumes that lock is already acquired.
func (sc *schedulerCache) deletePod(pod *v1.Pod) error {
	task := api.NewTask(pod)

	if job, found := sc.jobs[task.Job]; found {
		if t, found := job.Tasks[task.UID]; found {
			task = t
		}
	}

	if err := sc.deleteTask(task); err != nil {
		return err
	}
	return nil
}

func (sc *schedulerCache) deleteTask(task *api.Task) error {
	if len(task.Job) != 0 {
		if job, found := sc.jobs[task.Job]; found {
			job.DeleteTask(task)
			return nil
		}
		return fmt.Errorf("failed to find Job <%v> for Task %v/%v", task.Job, task.Namespace, task.Name)
	}
	return nil
}

func (sc *schedulerCache) setPodGroup(podGroup *schedulingv1.PodGroup) {
	job := getJobID(podGroup)
	if _, found := sc.jobs[job]; !found {
		sc.jobs[job] = api.NewJob(job)
	}
	sc.jobs[job].SetPodGroup(podGroup)
}

func (sc *schedulerCache) deletePodGroup(jobID api.JobID) error {
	job, found := sc.jobs[jobID]
	if !found {
		return fmt.Errorf("cannot found job: %v", jobID)
	}
	job.UnsetPodGroup()
	return nil
}

func getJobID(podGroup *schedulingv1.PodGroup) api.JobID {
	return api.JobID(fmt.Sprintf("%s/%s", podGroup.Namespace, podGroup.Name))
}

func removePodGroupCondition(conditions []schedulingv1.PodGroupCondition, conditionType schedulingv1.PodGroupConditionType) []schedulingv1.PodGroupCondition {
	res := make([]schedulingv1.PodGroupCondition, 0)
	for _, cond := range conditions {
		if cond.Type == conditionType {
			continue
		}
		res = append(res, cond)
	}
	return res
}
