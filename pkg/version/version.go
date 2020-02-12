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

package version

import (
	"fmt"
	"runtime"
)

var (
	// Version shows the version of storm.
	Version = "Not provided."
	// GitSHA shows the git commit id of storm.
	GitSHA = "Not provided."
	// Built shows the built time of the binary.
	BuiltDate = "Not provided."
)

// PrintVersion prints versions from the array returned by Info() and exit
func PrintVersion() {
	for _, i := range Info() {
		fmt.Printf("%s\n", i)
	}
}

// Info returns an array of various service versions
func Info() []string {
	return []string{
		fmt.Sprintf("Version: %s", Version),
		fmt.Sprintf("Git SHA: %s", GitSHA),
		fmt.Sprintf("Built At: %s", BuiltDate),
		fmt.Sprintf("Go Version: %s", runtime.Version()),
		fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
