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
	schedulinglister "github.com/angao/coscheduling/pkg/client/listers/scheduling/v1alpha1"
)

// Cache collects pods/nodes information.
type Cache interface {
	// Run start informer
	Run(stopCh <-chan struct{})

	// WaitForCacheSync waits for all cache synced
	WaitForCacheSync(stopCh <-chan struct{}) bool

	PodGroupLister() schedulinglister.PodGroupLister
}
