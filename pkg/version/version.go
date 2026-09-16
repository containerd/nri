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
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
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

// majorMinorPatch returns the major.minor.patch prefix of the semantic version v.
func majorMinorPatch(sv *semver.Version) string {
	return strings.TrimSuffix(strings.TrimSuffix(sv.Original(), sv.Metadata()), sv.Prerelease())
}

// FindClosestMatch returns the largest version smaller or equal to a given one.
// "" is returned if no such version if found.
func FindClosestMatch(v string, versions []string) string {
	// Note: A git-described version suffix (-N-gSHA1[.*])) is not semantically
	// semver-correct as semver considers it a prerelease identifier. Therefore
	// semver for instance considers v2.2.0-225-ge9dc15b7a.m < v2.2.0, which is
	// obviously not the case. In lack of a better choice, we strip any such
	// suffix from v before comparison.
	v = stripGitSuffix(v)

	var parsedVersions []*semver.Version
	for _, version := range versions {
		semverVersion, err := semver.NewVersion(version)
		if err != nil {
			continue
		}
		parsedVersions = append(parsedVersions, semverVersion)
	}

	sort.Sort(semver.Collection(parsedVersions))

	semverV, err := semver.NewVersion(v)
	if err != nil {
		return ""
	}

	latest := ""
	for _, pv := range parsedVersions {
		if pv.Compare(semverV) > 0 {
			break
		}
		latest = pv.Original()
	}
	return latest
}

// stripGitSuffix strips any git described suffix from a version string.
// We expect a valid git suffix to be of the form "-N-gSHA1[.m], where
// N is a decimal integer and SHA1 is a hexadecimal integer.
func stripGitSuffix(version string) string {
	sv, err := semver.NewVersion(version)
	if err != nil {
		return version
	}
	mmp := majorMinorPatch(sv)
	pre := sv.Prerelease()

	if len(pre) == 0 {
		return version
	}
	if mmp+pre != version {
		return version
	}

	commits, gsha1, ok := strings.Cut(pre, "-")
	if !ok || len(gsha1) == 0 || gsha1[0] != 'g' {
		return version
	}
	if _, err := strconv.ParseInt(commits, 10, 64); err != nil {
		return version
	}

	sha1, _, _ := strings.Cut(gsha1[1:], ".")
	if _, err := strconv.ParseInt(sha1, 16, 64); err != nil {
		return version
	}

	return strings.TrimSuffix(mmp, "-")
}
