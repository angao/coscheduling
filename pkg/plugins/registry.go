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

package plugins

import (
	"github.com/angao/coscheduling/pkg/cache"
	"github.com/angao/coscheduling/pkg/plugins/gang"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

func NewCommand(c cache.Cache) *cobra.Command {
	return app.NewSchedulerCommand(
		app.WithPlugin(gang.Name, func(_ *runtime.Unknown, h framework.FrameworkHandle) (framework.Plugin, error) {
			return gang.NewGang(c.PodGroupLister(), h), nil
		}),
	)
}
