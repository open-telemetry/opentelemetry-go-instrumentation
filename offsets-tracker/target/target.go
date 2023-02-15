package target

import (
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/keyval-dev/offsets-tracker/binary"
	"github.com/keyval-dev/offsets-tracker/cache"
	"github.com/keyval-dev/offsets-tracker/downloader"
	"github.com/keyval-dev/offsets-tracker/versions"
	"os"
)

type VersionsStrategy int
type BinaryFetchStrategy int

const (
	GoListVersionsStrategy    VersionsStrategy = 0
	GoDevFileVersionsStrategy VersionsStrategy = 1

	WrapAsGoAppBinaryFetchStrategy         BinaryFetchStrategy = 0
	DownloadPreCompiledBinaryFetchStrategy BinaryFetchStrategy = 1
)

type Result struct {
	ModuleName       string
	ResultsByVersion []*VersionedResult
}

type VersionedResult struct {
	Version    string
	OffsetData *binary.Result
}

type targetData struct {
	name                string
	VersionsStrategy    VersionsStrategy
	BinaryFetchStrategy BinaryFetchStrategy
	versionConstraint   *version.Constraints
	Cache               *cache.Cache
}

func New(name string, fileName string) *targetData {
	return &targetData{
		name:                name,
		VersionsStrategy:    GoListVersionsStrategy,
		BinaryFetchStrategy: WrapAsGoAppBinaryFetchStrategy,
		Cache:               cache.NewCache(fileName),
	}
}

func (t *targetData) VersionConstraint(constraint *version.Constraints) *targetData {
	t.versionConstraint = constraint
	return t
}

func (t *targetData) FindVersionsBy(strategy VersionsStrategy) *targetData {
	t.VersionsStrategy = strategy
	return t
}

func (t *targetData) DownloadBinaryBy(strategy BinaryFetchStrategy) *targetData {
	t.BinaryFetchStrategy = strategy
	return t
}

func (t *targetData) FindOffsets(dm []*binary.DataMember) (*Result, error) {
	fmt.Printf("%s: Discovering available versions\n", t.name)
	vers, err := t.findVersions()
	if err != nil {
		return nil, err
	}

	result := &Result{
		ModuleName: t.name,
	}
	for _, v := range vers {
		if t.Cache != nil {
			cachedResults, found := t.Cache.IsAllInCache(t.name, v, dm)
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

		os.RemoveAll(dir)
	}

	return result, nil
}

func (t *targetData) analyzeFile(exePath string, dm []*binary.DataMember) (*binary.Result, error) {
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

func (t *targetData) findVersions() ([]string, error) {
	var vers []string
	var err error
	if t.VersionsStrategy == GoListVersionsStrategy {
		vers, err = versions.FindVersionsUsingGoList(t.name)
		if err != nil {
			return nil, err
		}
	} else if t.VersionsStrategy == GoDevFileVersionsStrategy {
		vers, err = versions.FindVersionsFromGoWebsite()
		if err != nil {
			return nil, err
		}
	} else {
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

func (t *targetData) downloadBinary(modName string, version string) (string, string, error) {
	if t.BinaryFetchStrategy == WrapAsGoAppBinaryFetchStrategy {
		return downloader.DownloadBinary(modName, version)
	} else if t.BinaryFetchStrategy == DownloadPreCompiledBinaryFetchStrategy {
		return downloader.DownloadBinaryFromRemote(modName, version)
	}

	return "", "", fmt.Errorf("unsupported binary fetch strategy")
}
