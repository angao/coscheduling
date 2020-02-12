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
	"fmt"

	schedulingv1alpha1 "github.com/angao/coscheduling/pkg/apis/scheduling/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type TaskID types.UID

// Task will have all info about the task.
type Task struct {
	UID TaskID

	Job JobID

	Name      string
	Namespace string
	Status    TaskStatus
	NodeName  string

	Pod *v1.Pod
}

func NewTask(pod *v1.Pod) *Task {
	jobID := GetJobID(pod)
	return &Task{
		UID:       TaskID(pod.UID),
		Job:       jobID,
		Name:      pod.Name,
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Status:    getTaskStatus(pod),
		Pod:       pod,
	}
}

// String returns the taskInfo details in a string
func (t Task) String() string {
	return fmt.Sprintf("Task (%v:%v/%v): job %v, status %v", t.UID, t.Namespace, t.Name, t.Job, t.Status)
}

func GetJobID(pod *v1.Pod) JobID {
	if len(pod.Labels) != 0 {
		if gn, found := pod.Labels[schedulingv1alpha1.GroupNameLabelKey]; found && len(gn) != 0 {
			// Make sure Pod and PodGroup belong to the same namespace.
			jobID := fmt.Sprintf("%s/%s", pod.Namespace, gn)
			return JobID(jobID)
		}
	}
	return ""
}

type JobID types.UID

type taskMap map[TaskID]*Task

type Job struct {
	UID JobID

	Name            string
	Namespace       string
	TaskStatusIndex map[TaskStatus]taskMap
	Tasks           taskMap

	MinAvailable int32

	CreationTimestamp metav1.Time
	PodGroup          *schedulingv1alpha1.PodGroup
}

func NewJob(uid JobID, tasks ...*Task) *Job {
	job := &Job{
		UID:             uid,
		MinAvailable:    0,
		TaskStatusIndex: make(map[TaskStatus]taskMap),
		Tasks:           make(taskMap),
	}

	for _, task := range tasks {
		job.AddTask(task)
	}
	return job
}

func (job *Job) addTaskIndex(task *Task) {
	if _, found := job.TaskStatusIndex[task.Status]; !found {
		job.TaskStatusIndex[task.Status] = make(taskMap)
	}

	job.TaskStatusIndex[task.Status][task.UID] = task
}

func (job *Job) AddTask(task *Task) {
	job.Tasks[task.UID] = task
	job.addTaskIndex(task)
}

func (job *Job) UpdateTaskStatus(task *Task, status TaskStatus) {
	job.DeleteTask(task)
	task.Status = status
	job.AddTask(task)
}

func (job *Job) DeleteTask(task *Task) {
	if t, found := job.Tasks[task.UID]; found {
		delete(job.Tasks, task.UID)
		job.deleteTaskIndex(t)
	}
}

func (job *Job) deleteTaskIndex(task *Task) {
	if tasks, found := job.TaskStatusIndex[task.Status]; found {
		delete(tasks, task.UID)
		if len(tasks) == 0 {
			delete(job.TaskStatusIndex, task.Status)
		}
	}
}

// SetPodGroup sets podGroup details to a job
func (job *Job) SetPodGroup(pg *schedulingv1alpha1.PodGroup) {
	job.Name = pg.Name
	job.Namespace = pg.Namespace
	job.MinAvailable = pg.Spec.MinMember
	job.CreationTimestamp = pg.GetCreationTimestamp()

	job.PodGroup = pg
}

// UnsetPodGroup removes podGroup details from a job
func (job *Job) UnsetPodGroup() {
	job.PodGroup = nil
}

// ValidTaskNum returns the number of tasks that are valid
func (job *Job) ValidTaskNum() int32 {
	occupied := 0
	for status, tasks := range job.TaskStatusIndex {
		if AllocatedStatus(status) || status == Succeeded || status == Pending {
			occupied = occupied + len(tasks)
		}
	}
	return int32(occupied)
}

// ReadyTaskNum returns the number of tasks that are ready.
func (job *Job) ReadyTaskNum() int32 {
	occupied := 0
	for status, tasks := range job.TaskStatusIndex {
		if AllocatedStatus(status) || status == Succeeded {
			occupied = occupied + len(tasks)
		}
	}
	return int32(occupied)
}

// Ready returns whether job is ready for run
func (job *Job) Ready() bool {
	occupied := job.ReadyTaskNum()
	return occupied >= job.MinAvailable
}

// String returns a jobInfo object in string format
func (job Job) String() string {
	res := ""

	i := 0
	for _, task := range job.Tasks {
		res = res + fmt.Sprintf("\n\t %d: %v", i, task)
		i++
	}

	return fmt.Sprintf("Job (%v): namespace %v (%v), minAvailable %d, podGroup %+v",
		job.UID, job.Namespace, job.Name, job.MinAvailable, job.PodGroup) + res
}
