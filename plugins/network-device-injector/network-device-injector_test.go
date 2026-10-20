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

package main

import (
	"os"
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestParseNetdevicesDefaults(t *testing.T) {
	for _, key := range []string{netdeviceKey, oldNetdeviceKey} {
		t.Run(key, func(t *testing.T) {
			devices, err := parseNetdevices(map[string]string{
				key + "/pod": `
- name: dummy0
- name: dummy1
  address: 192.0.2.1
- name: dummy2
  address: "2001:db8::1"
- name: dummy3
  new_name: eth1
  address: 192.0.2.2
  prefix: 24
  mtu: 1500
`,
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []netdevice{
				{Name: "dummy0", NewName: "dummy0"},
				{Name: "dummy1", NewName: "dummy1", Address: "192.0.2.1", Prefix: 32},
				{Name: "dummy2", NewName: "dummy2", Address: "2001:db8::1", Prefix: 128},
				{Name: "dummy3", NewName: "eth1", Address: "192.0.2.2", Prefix: 24, MTU: 1500},
			}
			if !reflect.DeepEqual(devices, want) {
				t.Fatalf("parsed devices = %#v, want %#v", devices, want)
			}
		})
	}
}

func TestSampleNetworkDeviceAnnotations(t *testing.T) {
	data, err := os.ReadFile("sample-network-device-inject.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var sample struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := yaml.Unmarshal(data, &sample); err != nil {
		t.Fatal(err)
	}
	devices, err := parseNetdevices(sample.Metadata.Annotations)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Name != "dummy0" || devices[0].NewName != "eth33" {
		t.Fatalf("sample must inject dummy0 as eth33, got %#v", devices)
	}
}
