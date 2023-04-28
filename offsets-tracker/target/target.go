// Copyright The OpenTelemetry Authors
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

package target

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/offsets-tracker/binary"
	"go.opentelemetry.io/auto/offsets-tracker/cache"
	"go.opentelemetry.io/auto/offsets-tracker/downloader"
	"go.opentelemetry.io/auto/offsets-tracker/versions"
)

// VersionsStrategy is a strategy used when determining the version of a Go
// module.
type VersionsStrategy int

// BinaryFetchStrategy is a strategy used when fetching executable binaries.
type BinaryFetchStrategy int

// Target parsing strategies.
const (
	GoListVersionsStrategy    VersionsStrategy = 0
	GoDevFileVersionsStrategy VersionsStrategy = 1

	WrapAsGoAppBinaryFetchStrategy         BinaryFetchStrategy = 0
	DownloadPreCompiledBinaryFetchStrategy BinaryFetchStrategy = 1
)

// Result are all the offsets for a module.
type Result struct {
	ModuleName       string
	ResultsByVersion []*VersionedResult
}

// VersionedResult is the offset for a version of a module.
type VersionedResult struct {
	Version    string
	OffsetData *binary.Result
}

// Data represents the target Go module data.
type Data struct {
	name                string
	versionsStrategy    VersionsStrategy
	binaryFetchStrategy BinaryFetchStrategy
	versionConstraint   *version.Constraints
	cache               *cache.Cache
}

// New returns a new Data.
func New(name string, fileName string) *Data {
	return &Data{
		name:                name,
		versionsStrategy:    GoListVersionsStrategy,
		binaryFetchStrategy: WrapAsGoAppBinaryFetchStrategy,
		cache:               cache.NewCache(fileName),
	}
}

// VersionConstraint sets the version constraint used to constraint.
func (t *Data) VersionConstraint(constraint *version.Constraints) *Data {
	t.versionConstraint = constraint
	return t
}

// FindVersionsBy sets the VersionsStrategy used.
func (t *Data) FindVersionsBy(strategy VersionsStrategy) *Data {
	t.versionsStrategy = strategy
	return t
}

// DownloadBinaryBy sets the BinaryFetchStrategy used.
func (t *Data) DownloadBinaryBy(strategy BinaryFetchStrategy) *Data {
	t.binaryFetchStrategy = strategy
	return t
}

// FindOffsets returns all the offsets found based on dm.
func (t *Data) FindOffsets(dm []*binary.DataMember) (*Result, error) {
	fmt.Printf("%s: Discovering available versions\n", t.name)
	vers, err := t.findVersions()
	if err != nil {
		return nil, err
	}

	result := &Result{
		ModuleName: t.name,
	}
	for _, v := range vers {
		if t.cache != nil {
			cachedResults, found := t.cache.IsAllInCache(v, dm)
			if found {
				fmt.Printf("%s: Found all requested offsets in cache for version %s\n", t.name, v)
				result.ResultsByVersion = append(result.ResultsByVersion, &VersionedResult{
					Version: v,
					OffsetData: &binary.Result{
						DataMembers: cachedResults,
					},
				})
				continue
			}
		}

		fmt.Printf("%s: Downloading version %s\n", t.name, v)
		exePath, dir, err := t.downloadBinary(t.name, v)
		if err != nil {
			return nil, err
		}

		fmt.Printf("%s: Analyzing binary for version %s\n", t.name, v)
		res, err := t.analyzeFile(exePath, dm)
		if err == binary.ErrOffsetsNotFound {
			fmt.Printf("%s: could not find offsets for version %s\n", t.name, v)
		} else if err != nil {
			return nil, err
		} else {
			result.ResultsByVersion = append(result.ResultsByVersion, &VersionedResult{
				Version:    v,
				OffsetData: res,
			})
		}

		_ = os.RemoveAll(dir)
	}

	return result, nil
}

func (t *Data) analyzeFile(exePath string, dm []*binary.DataMember) (*binary.Result, error) {
	f, err := os.Open(exePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res, err := binary.FindOffsets(f, dm)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (t *Data) findVersions() ([]string, error) {
	var vers []string
	var err error
	switch t.versionsStrategy {
	case GoListVersionsStrategy:
		vers, err = versions.FindVersionsUsingGoList(t.name)
		if err != nil {
			return nil, err
		}
	case GoDevFileVersionsStrategy:
		vers, err = versions.FindVersionsFromGoWebsite()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported version strategy")
	}

	if t.versionConstraint == nil {
		return vers, nil
	}

	var filteredVers []string
	for _, v := range vers {
		semver, err := version.NewVersion(v)
		if err != nil {
			return nil, err
		}

		if t.versionConstraint.Check(semver) {
			filteredVers = append(filteredVers, v)
		}
	}

	return filteredVers, nil
}

func (t *Data) downloadBinary(modName string, version string) (string, string, error) {
	switch t.binaryFetchStrategy {
	case WrapAsGoAppBinaryFetchStrategy:
		return downloader.DownloadBinary(modName, version)
	case DownloadPreCompiledBinaryFetchStrategy:
		return downloader.DownloadBinaryFromRemote(modName, version)
	}

	return "", "", fmt.Errorf("unsupported binary fetch strategy")
}
