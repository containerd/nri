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

package version

import (
	"runtime/debug"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

var (
	// Build returns the build suffix for a version string.
	Build = semver.Build
	// Compare compares to version strings, returning -1, 0, or 1 according to their
	// semantic version precedence.
	Compare = semver.Compare
	// IsValid checks a version string for validity.
	IsValid = semver.IsValid
	// Major returns the major version prefix of a semantic version string.
	Major = semver.Major
	// MajorMinor returns the major.minor version prefix of a semantic version string.
	MajorMinor = semver.MajorMinor
	// Prerelease returns the prerelease suffix for a version string.
	Prerelease = semver.Prerelease
	// Sort sorts a slice of version strings in increasing order.
	Sort = semver.Sort
)

const (
	// UnknownVersion is reported for failed version detection.
	UnknownVersion = "0.0.0-unknown"
	// DevelVersion is what we get from debug/build info when building
	// plugins within the NRI repository.
	DevelVersion = "(devel)"
	// nriModulePath is the module we look for to discover the NRI version.
	nriModulePath = "github.com/containerd/nri"
)

// GetFromBuildInfo returns the locally used NRI version. This
// is taken either from the debug/build info provided by the
// golang runtime, or for plugins hosted in the NRI repository
// from a git-described version generated at build time.
func GetFromBuildInfo() string {
	version := UnknownVersion

	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, mod := range bi.Deps {
			if mod.Path != nriModulePath {
				continue
			}

			if mod.Replace != nil && mod.Replace.Version != DevelVersion {
				version = mod.Replace.Version
			} else {
				version = mod.Version
			}
		}
	}

	if version == DevelVersion {
		return fallbackVersion()
	}

	return version
}

// MajorMinorPatch returns the major.minor.patch prefix of the semantic version v.
func MajorMinorPatch(v string) string {
	return strings.TrimSuffix(strings.TrimSuffix(v, Build(v)), Prerelease(v))
}

// FindClosestMatch returns the largest version smaller or equal to a given one.
// "" is returned if no such version if found.
func FindClosestMatch(v string, versions []string) string {
	// Note: A git-described version suffix (-N-gSHA1[.*])) is not semantically
	// semver-correct as semver considers it a prerelease identifier. Therefore
	// semver for instance considers v2.2.0-225-ge9dc15b7a.m < v2.2.0, which is
	// obviously not the case. In lack of a better choice, we strip any such
	// suffix from v before comparison.
	v = StripGitSuffix(v)
	Sort(versions)

	latest := ""
	for _, ver := range versions {
		if Compare(ver, v) > 0 {
			break
		}
		latest = ver
	}
	return latest
}

// StripGitSuffix strips any git described suffix from a version string.
// We expect a valid git suffix to be of the form "-N-gSHA1[.m], where
// N is an decimal integer and SHA1 is a hexadecimal integer.
func StripGitSuffix(version string) string {
	mmp := MajorMinorPatch(version)
	pre := Prerelease(version)
	if mmp+pre != version {
		return version
	}

	if pre == "" || pre[0] != '-' {
		return version
	}

	commits, gsha1, ok := strings.Cut(pre[1:], "-")
	if !ok || gsha1 == "" || gsha1[0] != 'g' {
		return version
	}
	if _, err := strconv.ParseInt(commits, 10, 64); err != nil {
		return version
	}

	sha1, _, _ := strings.Cut(gsha1[1:], ".")
	if _, err := strconv.ParseInt(sha1, 16, 64); err != nil {
		return version
	}

	return mmp
}
