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

package api

import "testing"

// SetLinuxCPUPeriod must not silently mangle its value on the way into the
// optional CPU period field: the period is semantically unsigned (the kernel
// accepts [1000, 1000000]), and a negative value used to be converted into a
// huge garbage uint64 that later failed container creation inside runc with
// an error giving no hint about the plugin-side cause.
func TestSetLinuxCPUPeriod(t *testing.T) {
	const period = int64(100000)

	u := &ContainerUpdate{}
	u.SetLinuxCPUPeriod(period)
	if got := u.Linux.Resources.Cpu.Period.GetValue(); got != uint64(period) {
		t.Fatalf("ContainerUpdate: period = %d, want %d", got, period)
	}

	a := &ContainerAdjustment{}
	a.SetLinuxCPUPeriod(period)
	if got := a.Linux.Resources.Cpu.Period.GetValue(); got != uint64(period) {
		t.Fatalf("ContainerAdjustment: period = %d, want %d", got, period)
	}
}

// A negative CPU period is invalid: the setter must ignore it rather than
// record a wrapped-around huge value that fails container creation.
func TestSetLinuxCPUPeriodNegative(t *testing.T) {
	u := &ContainerUpdate{}
	u.SetLinuxCPUPeriod(-1)
	if p := getCPUPeriod(u.Linux.GetResources().GetCpu().GetPeriod()); p != 0 {
		t.Fatalf("ContainerUpdate: negative period recorded: %d", p)
	}

	a := &ContainerAdjustment{}
	a.SetLinuxCPUPeriod(-1)
	if p := getCPUPeriod(a.Linux.GetResources().GetCpu().GetPeriod()); p != 0 {
		t.Fatalf("ContainerAdjustment: negative period recorded: %d", p)
	}
}

func getCPUPeriod(p *OptionalUInt64) uint64 {
	if p == nil {
		return 0
	}
	return p.Value
}
