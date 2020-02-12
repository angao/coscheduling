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

package gang

import (
	"context"
	"fmt"
	"time"

	schedulingv1alpha1 "github.com/angao/coscheduling/pkg/apis/scheduling/v1alpha1"
	schedulinglister "github.com/angao/coscheduling/pkg/client/listers/scheduling/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

const (
	// Plugin name
	Name = "Gang"

	preFilterKey = "preFilter" + Name
)

var _ framework.PreFilterPlugin = &Gang{}
var _ framework.PermitPlugin = &Gang{}

type Gang struct {
	PodGroupLister schedulinglister.PodGroupLister
	Handle         framework.FrameworkHandle
}

type preFilterStateData struct {
	*schedulingv1alpha1.PodGroup
}

func (p *preFilterStateData) Clone() framework.StateData {
	return p
}

func NewGang(pgl schedulinglister.PodGroupLister, h framework.FrameworkHandle) framework.Plugin {
	return &Gang{
		PodGroupLister: pgl,
		Handle:         h,
	}
}

func (g *Gang) Name() string {
	return Name
}

func (g *Gang) PreFilter(_ context.Context, cycleState *framework.CycleState, pod *corev1.Pod) *framework.Status {
	if len(pod.Labels) == 0 {
		return nil
	}
	if pgName, found := pod.Labels[schedulingv1alpha1.GroupNameLabelKey]; found && len(pgName) != 0 {
		pg, err := g.getPodGroup(pod.Namespace, pgName)
		if err != nil {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
		}
		if g.calcTotalPods(pg.Namespace, pg.Name) < pg.Spec.MinMember {
			return framework.NewStatus(framework.UnschedulableAndUnresolvable,
				fmt.Sprintf("The total pods of PodGroup %s less than minMember %v", pgName, pg.Spec.MinMember))
		}
		cycleState.Write(preFilterKey, &preFilterStateData{pg})
	}
	return nil
}

func (g *Gang) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (g *Gang) Permit(_ context.Context, cycleState *framework.CycleState, pod *corev1.Pod, _ string) (*framework.Status, time.Duration) {
	if g.Skip(pod) {
		return nil, 0
	}

	stateData, err := getPreFilterStateData(cycleState)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error()), 0
	}

	waitingPods := g.calcWaitingPods(stateData.Namespace, stateData.Name)
	runningPods := g.calcRunningPods(stateData.Namespace, stateData.Name)

	klog.V(4).Infof("waitingPods: %v, runningPods: %v podGroup minMember: %v", waitingPods, runningPods, stateData.Spec.MinMember)

	if waitingPods+runningPods < stateData.Spec.MinMember {
		return framework.NewStatus(framework.Wait, ""), 10 * time.Second
	}

	g.Handle.IterateOverWaitingPods(func(p framework.WaitingPod) {
		pod := p.GetPod()
		if pod.Labels[schedulingv1alpha1.GroupNameLabelKey] == stateData.Name {
			p.Allow(Name)
		}
	})
	return nil, 0
}

func getPreFilterStateData(cycleState *framework.CycleState) (*preFilterStateData, error) {
	stateData, err := cycleState.Read(preFilterKey)
	if err != nil {
		return nil, err
	}

	s, ok := stateData.(*preFilterStateData)
	if !ok {
		return nil, fmt.Errorf("%+v convert to *preFilterStateData error", stateData)
	}
	return s, nil
}

func (g *Gang) Skip(pod *corev1.Pod) bool {
	_, found := pod.Labels[schedulingv1alpha1.GroupNameLabelKey]
	return !found
}

func (g *Gang) getPodGroup(namespace, name string) (*schedulingv1alpha1.PodGroup, error) {
	return g.PodGroupLister.PodGroups(namespace).Get(name)
}

func (g *Gang) calcWaitingPods(namespace, name string) int32 {
	replica := int32(1)
	g.Handle.IterateOverWaitingPods(func(p framework.WaitingPod) {
		pod := p.GetPod()
		if pod.Namespace == namespace && pod.Labels[schedulingv1alpha1.GroupNameLabelKey] == name {
			replica++
		}
	})
	return replica
}

func (g *Gang) calcRunningPods(namespace, name string) int32 {
	selector := labels.SelectorFromSet(labels.Set{schedulingv1alpha1.GroupNameLabelKey: name})

	pods, err := g.Handle.SnapshotSharedLister().Pods().FilteredList(func(pod *corev1.Pod) bool {
		return pod.Namespace == namespace && (pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded)
	}, selector)
	if err != nil {
		klog.Errorf("FilteredList pod failed: %v", err)
		return 0
	}
	return int32(len(pods))
}

func (g *Gang) calcTotalPods(namespace, name string) int32 {
	selector := labels.SelectorFromSet(labels.Set{schedulingv1alpha1.GroupNameLabelKey: name})
	field := fields.ParseSelectorOrDie("metadata.namespace=" + namespace)

	pods, err := g.Handle.ClientSet().CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: selector.String(),
		FieldSelector: field.String(),
	})
	if err != nil {
		klog.Errorf("List pod failed: %v", err)
		return 0
	}
	return int32(len(pods.Items))
}
