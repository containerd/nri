package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
				MountPath: "/var/run/secrets/spiffe",
				CertFileName: "svid.pem",
				KeyFileName: "svid_key.pem",
				BundleFileName: "svid_bundle.pem",
				SpiffeId: "spiffe://example.org/p0/c0",
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
				MountPath: "/var/run/secrets/spiffe",
				CertFileName: "svid.pem",
				KeyFileName: "svid_key.pem",
				BundleFileName: "svid_bundle.pem",
				SpiffeId: "spiffe://example.org/p0/c0",
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

// TODO we can also create test cases for processDelegatedIdentityUpdate()