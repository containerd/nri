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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestMain initialises the package-level logger so that functions under test
// that call log.Warnf / log.Infof / etc. do not panic with a nil receiver.
func TestMain(m *testing.M) {
	log = logrus.New()
	log.SetOutput(os.Stderr)
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Existing tests (unchanged)
// ---------------------------------------------------------------------------

func TestParseIdentityAnnotations(t *testing.T) {
	type testCase struct {
		name        string
		annotations map[string]string
		result      *identityConfig
	}

	for _, tc := range []*testCase{
		{
			name: "no identity annotations",
			annotations: map[string]string{
				"foo": "bar",
			},
			result: nil,
		},
		{
			name: "identity annotated",
			annotations: map[string]string{
				"identity.noderesource.dev/container.c0": `
mount_path: /var/run/secrets/spiffe
host_mount_path: /var/run/secrets/
cert_file_name: svid.pem
key_file_name: svid_key.pem
bundle_file_name: svid_bundle.pem
spiffe_id: spiffe://example.org/p0/c0
`,
			},
			result: &identityConfig{
				MountPath:      "/var/run/secrets/spiffe",
				CertFileName:   "svid.pem",
				KeyFileName:    "svid_key.pem",
				BundleFileName: "svid_bundle.pem",
				SpiffeId:       "spiffe://example.org/p0/c0",
			},
		},
		{
			name: "container name mismatch",
			annotations: map[string]string{
				"identity.noderesource.dev/container.c1": `
mount_path: /var/run/secrets/spiffe
cert_file_name: svid.pem
key_file_name: svid_key.pem
bundle_file_name: svid_bundle.pem
spiffe_id: spiffe://example.org/p0/c0
`,
			},
			result: nil,
		},
		{
			name: "no container name",
			annotations: map[string]string{
				"identity.noderesource.dev": `
mount_path: /var/run/secrets/spiffe
cert_file_name: svid.pem
key_file_name: svid_key.pem
bundle_file_name: svid_bundle.pem
spiffe_id: spiffe://example.org/p0/c0
`,
			},
			result: &identityConfig{
				MountPath:      "/var/run/secrets/spiffe",
				CertFileName:   "svid.pem",
				KeyFileName:    "svid_key.pem",
				BundleFileName: "svid_bundle.pem",
				SpiffeId:       "spiffe://example.org/p0/c0",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := parseIdentityConfig("c0", tc.annotations)
			require.Nil(t, err, "config parsing error")
			require.Equal(t, tc.result, config, "parsed config")
		})
	}

}

// ---------------------------------------------------------------------------
// TestParseIdentityConfig_Defaults
// ---------------------------------------------------------------------------

// TestParseIdentityConfig_Defaults verifies that omitting individual fields
// from the annotation YAML causes parseIdentityConfig to fill in the package-
// level defaults for MountPath, CertFileName, KeyFileName, and BundleFileName.
func TestParseIdentityConfig_Defaults(t *testing.T) {
	annotations := map[string]string{
		// Completely empty body — all fields omitted.
		identityKey: `{}`,
	}

	cfg, err := parseIdentityConfig("c0", annotations)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, defaultMountPath, cfg.MountPath, "default MountPath")
	require.Equal(t, defaultCertFileName, cfg.CertFileName, "default CertFileName")
	require.Equal(t, defaultKeyFileName, cfg.KeyFileName, "default KeyFileName")
	require.Equal(t, defaultBundleFileName, cfg.BundleFileName, "default BundleFileName")
}

// TestParseIdentityConfig_PartialDefaults checks that an annotation which
// sets only some fields still gets the remaining fields defaulted correctly.
func TestParseIdentityConfig_PartialDefaults(t *testing.T) {
	annotations := map[string]string{
		identityKey: `cert_file_name: leaf.pem`,
	}

	cfg, err := parseIdentityConfig("any", annotations)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Explicitly set field must be preserved.
	require.Equal(t, "leaf.pem", cfg.CertFileName)
	// Unset fields must get their defaults.
	require.Equal(t, defaultMountPath, cfg.MountPath)
	require.Equal(t, defaultKeyFileName, cfg.KeyFileName)
	require.Equal(t, defaultBundleFileName, cfg.BundleFileName)
}

// ---------------------------------------------------------------------------
// TestParseIdentityConfig_InvalidFilenames
// ---------------------------------------------------------------------------

// TestParseIdentityConfig_InvalidFilenames ensures that cert/key/bundle
// filenames that are not simple base-names are rejected with an error.
func TestParseIdentityConfig_InvalidFilenames(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
	}{
		{
			name: "absolute cert path",
			yaml: `cert_file_name: /etc/svid.pem`,
		},
		{
			name: "absolute key path",
			yaml: `key_file_name: /etc/key.pem`,
		},
		{
			name: "absolute bundle path",
			yaml: `bundle_file_name: /etc/bundle.pem`,
		},
		{
			name: "dotdot traversal in cert",
			yaml: `cert_file_name: ../svid.pem`,
		},
		{
			name: "dotdot traversal in key",
			yaml: `key_file_name: ../../key.pem`,
		},
		{
			name: "dotdot traversal in bundle",
			yaml: `bundle_file_name: ../bundle.pem`,
		},
		{
			name: "single dot cert",
			yaml: `cert_file_name: "."`,
		},
		{
			name: "single dot key",
			yaml: `key_file_name: "."`,
		},
		{
			name: "single dot bundle",
			yaml: `bundle_file_name: "."`,
		},
		{
			name: "slash prefix in cert",
			yaml: `cert_file_name: subdir/svid.pem`,
		},
		{
			name: "slash prefix in key",
			yaml: `key_file_name: subdir/key.pem`,
		},
		{
			name: "slash prefix in bundle",
			yaml: `bundle_file_name: subdir/bundle.pem`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			annotations := map[string]string{identityKey: tc.yaml}
			cfg, err := parseIdentityConfig("c0", annotations)
			require.Error(t, err, "expected validation error for %q", tc.yaml)
			require.Nil(t, cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// TestGetAnnotation
// ---------------------------------------------------------------------------

// TestGetAnnotation covers the three-tier lookup precedence implemented in
// getAnnotation: container-specific key > pod-level key > global key.
func TestGetAnnotation(t *testing.T) {
	const (
		mainKey   = "identity.noderesource.dev"
		ctr       = "mycontainer"
		globalVal = "global"
		podVal    = "pod"
		ctrVal    = "container"
	)

	t.Run("container-specific wins over pod and global", func(t *testing.T) {
		annotations := map[string]string{
			mainKey + "/container." + ctr: ctrVal,
			mainKey + "/pod":              podVal,
			mainKey:                       globalVal,
		}
		got := getAnnotation(annotations, mainKey, ctr)
		require.Equal(t, []byte(ctrVal), got)
	})

	t.Run("pod-level wins over global when no container key", func(t *testing.T) {
		annotations := map[string]string{
			mainKey + "/pod": podVal,
			mainKey:          globalVal,
		}
		got := getAnnotation(annotations, mainKey, ctr)
		require.Equal(t, []byte(podVal), got)
	})

	t.Run("global key used when no other keys match", func(t *testing.T) {
		annotations := map[string]string{
			mainKey: globalVal,
		}
		got := getAnnotation(annotations, mainKey, ctr)
		require.Equal(t, []byte(globalVal), got)
	})

	t.Run("returns nil when no matching annotation", func(t *testing.T) {
		annotations := map[string]string{
			"unrelated.key": "value",
		}
		got := getAnnotation(annotations, mainKey, ctr)
		require.Nil(t, got)
	})

	t.Run("returns nil for nil annotation map", func(t *testing.T) {
		got := getAnnotation(nil, mainKey, ctr)
		require.Nil(t, got)
	})

	t.Run("container-specific key for different container is not matched", func(t *testing.T) {
		annotations := map[string]string{
			mainKey + "/container.other": ctrVal,
		}
		got := getAnnotation(annotations, mainKey, ctr)
		require.Nil(t, got)
	})
}

// ---------------------------------------------------------------------------
// TestEnsureUnixPrefix
// ---------------------------------------------------------------------------

func TestEnsureUnixPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare path gets prefix",
			input: "/run/spire/admin.sock",
			want:  "unix:///run/spire/admin.sock",
		},
		{
			name:  "already prefixed path unchanged",
			input: "unix:///run/spire/admin.sock",
			want:  "unix:///run/spire/admin.sock",
		},
		{
			name:  "relative path gets prefix",
			input: "spire.sock",
			want:  "unix://spire.sock",
		},
		{
			name:  "empty string gets prefix",
			input: "",
			want:  "unix://",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ensureUnixPrefix(tc.input))
		})
	}
}

// ---------------------------------------------------------------------------
// TestGetHostDir
// ---------------------------------------------------------------------------

func TestGetHostDir(t *testing.T) {
	tests := []struct {
		name          string
		hostMountPath string
		podUUID       string
		containerName string
		want          string
	}{
		{
			name:          "standard components are joined",
			hostMountPath: "/var/run/spiffe/secrets",
			podUUID:       "abc-123",
			containerName: "mycontainer",
			want:          filepath.Join("/var/run/spiffe/secrets", "abc-123", "mycontainer"),
		},
		{
			name:          "trailing slash on hostMountPath is handled",
			hostMountPath: "/var/run/spiffe/secrets/",
			podUUID:       "pod-uid",
			containerName: "ctr",
			want:          filepath.Join("/var/run/spiffe/secrets/", "pod-uid", "ctr"),
		},
		{
			name:          "empty hostMountPath produces relative path",
			hostMountPath: "",
			podUUID:       "pod-uid",
			containerName: "ctr",
			want:          filepath.Join("pod-uid", "ctr"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, getHostDir(tc.hostMountPath, tc.podUUID, tc.containerName))
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers shared by file-writing tests
// ---------------------------------------------------------------------------

// selfSignedCert generates an ephemeral self-signed x509 certificate for
// use in tests. It uses ECDSA P-256 to keep generation fast.
func selfSignedCert(t *testing.T) *x509.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert
}

// ecPrivateKeyDER returns a PKCS#8-encoded ECDSA P-256 private key for tests.
func ecPrivateKeyDER(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	return der
}

// ---------------------------------------------------------------------------
// TestWriteCerts
// ---------------------------------------------------------------------------

func TestWriteCerts(t *testing.T) {
	t.Run("single certificate is written as valid PEM", func(t *testing.T) {
		dir := t.TempDir()
		cert := selfSignedCert(t)
		file := filepath.Join(dir, "svid.pem")

		require.NoError(t, writeCerts(file, []*x509.Certificate{cert}))

		data, err := os.ReadFile(file)
		require.NoError(t, err)

		block, rest := pem.Decode(data)
		require.NotNil(t, block, "expected a PEM block")
		require.Equal(t, "CERTIFICATE", block.Type)
		require.Equal(t, cert.Raw, block.Bytes)
		require.Empty(t, rest, "no trailing data expected after a single cert")
	})

	t.Run("multiple certificates are concatenated in PEM", func(t *testing.T) {
		dir := t.TempDir()
		c1 := selfSignedCert(t)
		c2 := selfSignedCert(t)
		file := filepath.Join(dir, "chain.pem")

		require.NoError(t, writeCerts(file, []*x509.Certificate{c1, c2}))

		data, err := os.ReadFile(file)
		require.NoError(t, err)

		// Decode both PEM blocks.
		b1, rest := pem.Decode(data)
		require.NotNil(t, b1)
		require.Equal(t, "CERTIFICATE", b1.Type)

		b2, remaining := pem.Decode(rest)
		require.NotNil(t, b2)
		require.Equal(t, "CERTIFICATE", b2.Type)
		require.Empty(t, remaining)
	})

	t.Run("empty cert slice writes empty file", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "empty.pem")

		require.NoError(t, writeCerts(file, []*x509.Certificate{}))

		data, err := os.ReadFile(file)
		require.NoError(t, err)
		require.Empty(t, data)
	})
}

// ---------------------------------------------------------------------------
// TestWritePrivateKey
// ---------------------------------------------------------------------------

func TestWritePrivateKey(t *testing.T) {
	t.Run("private key is written as PKCS8 PEM with mode 0600", func(t *testing.T) {
		dir := t.TempDir()
		keyDER := ecPrivateKeyDER(t)
		file := filepath.Join(dir, "key.pem")

		require.NoError(t, writePrivateKey(file, keyDER))

		data, err := os.ReadFile(file)
		require.NoError(t, err)

		block, rest := pem.Decode(data)
		require.NotNil(t, block, "expected a PEM block")
		require.Equal(t, "PRIVATE KEY", block.Type)
		require.Equal(t, keyDER, block.Bytes)
		require.Empty(t, rest)

		// Verify the file permissions are restricted (owner-read/write only).
		info, err := os.Stat(file)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}

// ---------------------------------------------------------------------------
// TestWriteX509Content
// ---------------------------------------------------------------------------

func TestWriteX509Content(t *testing.T) {
	t.Run("both cert and key files are written", func(t *testing.T) {
		dir := t.TempDir()
		cert := selfSignedCert(t)
		keyDER := ecPrivateKeyDER(t)

		cfg := &identityConfig{
			CertFileName: "svid.pem",
			KeyFileName:  "key.pem",
		}

		require.NoError(t, writeX509Content(dir, cfg, []*x509.Certificate{cert}, keyDER))

		// Cert file must exist and contain a valid CERTIFICATE block.
		certData, err := os.ReadFile(filepath.Join(dir, cfg.CertFileName))
		require.NoError(t, err)
		certBlock, _ := pem.Decode(certData)
		require.NotNil(t, certBlock)
		require.Equal(t, "CERTIFICATE", certBlock.Type)

		// Key file must exist and contain a valid PRIVATE KEY block.
		keyData, err := os.ReadFile(filepath.Join(dir, cfg.KeyFileName))
		require.NoError(t, err)
		keyBlock, _ := pem.Decode(keyData)
		require.NotNil(t, keyBlock)
		require.Equal(t, "PRIVATE KEY", keyBlock.Type)
	})
}

// ---------------------------------------------------------------------------
// TestProcessBundleUpdate
// ---------------------------------------------------------------------------

func TestProcessBundleUpdate(t *testing.T) {
	// processBundleUpdate is a method on *plugin but uses no plugin state —
	// all I/O goes through the provided hostDir and config parameters.
	p := &plugin{}

	t.Run("empty caCertificates map is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &identityConfig{BundleFileName: "bundle.pem"}

		err := p.processBundleUpdate("test/ctr", dir, cfg, map[string][]byte{})
		require.NoError(t, err)

		// No bundle file should have been created.
		_, statErr := os.Stat(filepath.Join(dir, cfg.BundleFileName))
		require.True(t, os.IsNotExist(statErr), "bundle file should not exist for empty update")
	})

	t.Run("valid CA certificate DER is written as PEM bundle", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &identityConfig{BundleFileName: "bundle.pem"}

		ca := selfSignedCert(t)
		// x509.ParseCertificates (used internally) expects raw concatenated DER.
		caCerts := map[string][]byte{
			"spiffe://example.org": ca.Raw,
		}

		err := p.processBundleUpdate("test/ctr", dir, cfg, caCerts)
		require.NoError(t, err)

		data, readErr := os.ReadFile(filepath.Join(dir, cfg.BundleFileName))
		require.NoError(t, readErr)

		block, _ := pem.Decode(data)
		require.NotNil(t, block, "expected a PEM CERTIFICATE block in bundle")
		require.Equal(t, "CERTIFICATE", block.Type)
	})

	t.Run("invalid DER in caCertificates returns error", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &identityConfig{BundleFileName: "bundle.pem"}

		caCerts := map[string][]byte{
			"spiffe://bad.org": []byte("not-valid-DER"),
		}

		err := p.processBundleUpdate("test/ctr", dir, cfg, caCerts)
		require.Error(t, err, "expected error for invalid DER")
	})

	t.Run("multiple trust domains are concatenated into one bundle file", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &identityConfig{BundleFileName: "bundle.pem"}

		ca1 := selfSignedCert(t)
		ca2 := selfSignedCert(t)
		caCerts := map[string][]byte{
			"spiffe://domain1.org": ca1.Raw,
			"spiffe://domain2.org": ca2.Raw,
		}

		err := p.processBundleUpdate("test/ctr", dir, cfg, caCerts)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, cfg.BundleFileName))
		require.NoError(t, err)

		// Count the number of PEM blocks; should equal the number of CA certs.
		count := 0
		rest := data
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			require.Equal(t, "CERTIFICATE", block.Type)
			count++
		}
		require.Equal(t, 2, count, "expected one PEM block per trust domain CA cert")
	})
}
