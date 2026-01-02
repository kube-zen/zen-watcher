// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// main is required for package main but unused in this example plugin
// The actual hook registration happens via init() functions in the other files.
func main() {
	// This is an example hooks package. In production, hooks are registered
	// via init() functions and compiled into the main zen-watcher binary.
	// This main() function exists only to satisfy Go's package main requirement.
}

