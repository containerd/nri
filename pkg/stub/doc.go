/*
   Copyright The containerd Authors.

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

//go:generate go build -C ../../hack/gen-pkg-alias
//go:generate ../../hack/gen-pkg-alias/gen-pkg-alias -src ../../pkg/stub/v1alpha1 -dst ../../pkg/stub -rm -out stub-v1alpha1.go -l ../../hack/license-header

package stub
