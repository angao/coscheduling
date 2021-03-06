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

package app

import (
	"github.com/angao/coscheduling/pkg/api"
	internalcache "github.com/angao/coscheduling/pkg/cache"
	"github.com/angao/coscheduling/pkg/plugins"
	"github.com/angao/coscheduling/pkg/utils"
	"github.com/angao/coscheduling/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

func NewSchedulerCommand() *cobra.Command {
	c := buildCache()

	cmd := plugins.NewCommand(c)
	cmd.Use = api.SchedulerName
	cmd.Long = "The coscheduling is a scheduler for Kubernetes and support gang scheduling."

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		go c.Run(wait.NeverStop)
		c.WaitForCacheSync(wait.NeverStop)
	}
	cmd.AddCommand(&cobra.Command{
		Use:  "version",
		Long: "Print version information and quit",
		Run: func(cmd *cobra.Command, args []string) {
			version.PrintVersion()
		},
	})
	return cmd
}

func buildCache() internalcache.Cache {
	config, err := utils.GetConfig("", "")
	if err != nil {
		klog.Fatal(err)
	}
	return internalcache.NewSchedulerCache(config, api.SchedulerName)
}
