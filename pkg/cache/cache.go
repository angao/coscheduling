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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/angao/coscheduling/pkg/api"
	schedulingv1alpha1 "github.com/angao/coscheduling/pkg/apis/scheduling/v1alpha1"
	schdulingclient "github.com/angao/coscheduling/pkg/client/clientset/versioned"
	schedinformer "github.com/angao/coscheduling/pkg/client/informers/externalversions"
	schedulinginformer "github.com/angao/coscheduling/pkg/client/informers/externalversions/scheduling/v1alpha1"
	schedulinglister "github.com/angao/coscheduling/pkg/client/listers/scheduling/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

type schedulerCache struct {
	sync.Mutex
	schedulerName string

	kubeclient  *kubernetes.Clientset
	schedclient *schdulingclient.Clientset

	podInformer      corev1.PodInformer
	podGroupInformer schedulinginformer.PodGroupInformer

	podLister      corelisters.PodLister
	podGroupLister schedulinglister.PodGroupLister
	// jobs is used to gang scheduling.
	jobs map[api.JobID]*api.Job
}

func NewSchedulerCache(config *rest.Config, schedulerName string) Cache {
	kubeclient := kubernetes.NewForConfigOrDie(config)
	schedclient := schdulingclient.NewForConfigOrDie(config)

	sc := &schedulerCache{
		kubeclient:    kubeclient,
		schedclient:   schedclient,
		schedulerName: schedulerName,
		jobs:          make(map[api.JobID]*api.Job),
	}

	informerFactory := informers.NewSharedInformerFactory(sc.kubeclient, 0)
	sc.podInformer = informerFactory.Core().V1().Pods()
	sc.podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *v1.Pod:
				pod := t
				if schedulerName != pod.Spec.SchedulerName {
					if len(pod.Spec.NodeName) == 0 {
						return false
					}
				}
				return true
			default:
				return false
			}
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.AddPod,
			UpdateFunc: sc.UpdatePod,
			DeleteFunc: sc.DeletePod,
		},
	})

	schedulingFactory := schedinformer.NewSharedInformerFactory(sc.schedclient, 0)
	sc.podGroupInformer = schedulingFactory.Scheduling().V1alpha1().PodGroups()
	sc.podGroupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sc.AddPodGroup,
		UpdateFunc: sc.UpdatePodGroup,
		DeleteFunc: sc.DeletePodGroup,
	})

	sc.podLister = sc.podInformer.Lister()
	sc.podGroupLister = sc.podGroupInformer.Lister()
	return sc
}

func (sc *schedulerCache) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	go sc.podInformer.Informer().Run(stopCh)
	go sc.podGroupInformer.Informer().Run(stopCh)
	go wait.Until(sc.syncPodGroupStatus, time.Second, stopCh)
}

// WaitForCacheSync waits for all cache synced
func (sc *schedulerCache) WaitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh,
		func() []cache.InformerSynced {
			informerSynced := []cache.InformerSynced{
				sc.podInformer.Informer().HasSynced,
				sc.podGroupInformer.Informer().HasSynced,
			}
			return informerSynced
		}()...,
	)
}

func (sc *schedulerCache) PodGroupLister() schedulinglister.PodGroupLister {
	return sc.podGroupLister
}

func (sc *schedulerCache) syncPodGroupStatus() {
	for jobID, job := range sc.jobs {
		namespace, name, err := cache.SplitMetaNamespaceKey(string(jobID))
		if err != nil {
			klog.Errorf("Split PodGroup metadata info failed: %v", err)
			continue
		}

		podGroup, err := sc.podGroupLister.PodGroups(namespace).Get(name)
		if err != nil {
			klog.Errorf("Get PodGroup failed: %v", err)
			continue
		}

		status := podGroup.Status
		if len(job.TaskStatusIndex[api.Unschedulable]) > 0 {
			status.Conditions = removePodGroupCondition(status.Conditions, schedulingv1alpha1.PodGroupUnschedulable)

			var tasks []string
			for _, task := range job.TaskStatusIndex[api.Unschedulable] {
				tasks = append(tasks, task.Name)
			}
			cond := schedulingv1alpha1.PodGroupCondition{
				Type:               schedulingv1alpha1.PodGroupUnschedulable,
				Reason:             "Unschedulable",
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Message:            fmt.Sprintf("There are `%v` tasks which are unschedulable", strings.Join(tasks, ",")),
			}
			status.Conditions = append(status.Conditions, cond)
		}

		if job.Ready() {
			status.Phase = schedulingv1alpha1.PodGroupRunning
			status.Conditions = removePodGroupCondition(status.Conditions, schedulingv1alpha1.PodGroupUnschedulable)
			status.Conditions = removePodGroupCondition(status.Conditions, schedulingv1alpha1.PodGroupScheduled)
			status.Conditions = append(status.Conditions, schedulingv1alpha1.PodGroupCondition{
				Type:               schedulingv1alpha1.PodGroupScheduled,
				Reason:             "Scheduled",
				Status:             v1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Message:            "PodGroup can be scheduled",
			})
		} else {
			status.Phase = schedulingv1alpha1.PodGroupPending
			if len(job.Tasks) == 0 {
				status.Conditions = make([]schedulingv1alpha1.PodGroupCondition, 0)
			}
		}

		status.Running = int32(len(job.TaskStatusIndex[api.Running]))
		status.Succeeded = int32(len(job.TaskStatusIndex[api.Succeeded]))
		status.Failed = int32(len(job.TaskStatusIndex[api.Failed]))

		if !reflect.DeepEqual(status, job.PodGroup.Status) {
			podGroup.Status = status
			_, err = sc.schedclient.SchedulingV1alpha1().PodGroups(job.Namespace).Update(podGroup)
			if err != nil {
				klog.Errorf("Update PodGroup failed: %v", err)
			}
		}
	}
}
