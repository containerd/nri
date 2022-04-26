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

package adaptation

import (
	"fmt"
	"os"
	"testing"
)

func tempFile(dir, pattern string, content []byte) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	if content != nil {
		if _, err = f.Write(content); err != nil {
			f.Close()
			return nil, fmt.Errorf("failed to write temporary content: %w", err)
		}
	}
	return f, nil
}

func TestNonExistentConfiguration(t *testing.T) {
	_, err := ReadConfig("/.no-such-dir/.no-such-file.conf")
	if err == nil {
		t.Errorf("NRI should fail to parse non-existent configuration")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	configContent := `fooBar:
  - none
`
	f, err := tempFile("", "test.cfg", []byte(configContent))
	if err != nil {
		t.Errorf("failed to create temporary config file: %v", err)
		return
	}
	_, err = ReadConfig(f.Name())
	if err == nil {
		t.Errorf("NRI should fail to parse invalid configuration")
	}
}

func TestValidConfiguration(t *testing.T) {
	configContent := `disableConnections: false
`
	f, err := tempFile("", "test.cfg", []byte(configContent))
	if err != nil {
		t.Errorf("failed to create temporary config file: %v", err)
		return
	}
	_, err = ReadConfig(f.Name())
	if err != nil {
		t.Errorf("NRI should parse invalid configuration without errors (%v)", err)
	}
}
