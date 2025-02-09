// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
)

const (
	repoAPI   = "https://api.github.com/repos/libbpf/libbpf/releases"
	batchSize = 1024 // Number of bytes read of the tarball per batch.
)

// Only these files are extraced to the desitination.
var requiredFiles = []string{
	"LICENSE",
	"LICENSE.BSD-2-Clause",
	"LICENSE.LGPL-2.1",
	"src/bpf_helpers.h",
	"src/bpf_helper_defs.h",
	"src/bpf_tracing.h",
}

func main() {
	ver := flag.String("version", ">= 0.0.0", "Version constraint for libbpf")
	dest := flag.String("dest", "./libbpf", "Destination directory for extracted files")
	flag.Parse()

	constraint, err := version.NewConstraint(*ver)
	if err != nil {
		log.Fatalf("Invalid version %q: %s", *ver, err)
	}

	// Ensure output destination is a directory and exists.
	err = os.MkdirAll(*dest, os.ModePerm)
	if err != nil {
		log.Fatalf("Invalid destination directory %q: %s", *dest, err)
	}

	release, err := latestRelease(constraint)
	if err != nil {
		log.Fatal("Error fetching release:", err)
	}

	log.Println("Downloading:", release.TarballURL)
	if err := download(release.TarballURL, *dest); err != nil {
		log.Fatal("Error extracting files:", err)
	}

	log.Println("Files extracted successfully to", *dest)
}

// Release is a partial set of GitHub release info.
type Release struct {
	TagName    *version.Version `json:"tag_name"`
	TarballURL string           `json:"tarball_url"`
}

func latestRelease(constraint version.Constraints) (*Release, error) {
	resp, err := http.Get(repoAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch releases: %s", resp.Status)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	var matchingReleases []Release
	for i := range releases {
		if constraint.Check(releases[i].TagName) {
			matchingReleases = append(matchingReleases, releases[i])
		}
	}

	if len(matchingReleases) == 0 {
		return nil, errors.New("no matching release found")
	}

	sort.Slice(matchingReleases, func(i, j int) bool {
		return matchingReleases[i].TagName.GreaterThan(matchingReleases[j].TagName)
	})

	return &matchingReleases[0], nil
}

func download(url, dest string) error {
	resp, err := http.Get(url) // nolint: gosec  // Variable URL from GitHub API.
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download tarball: %s", resp.Status)
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		for _, file := range requiredFiles {
			if strings.HasSuffix(hdr.Name, file) {
				// Flatten directory structure.
				path := filepath.Join(dest, filepath.Base(file))
				if err := write(path, tarReader); err != nil {
					return err
				}
				log.Printf("Extracted %q to %q", file, path)
			}
		}
	}

	return nil
}

func write(dst string, src io.Reader) error {
	outFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for {
		// Batch read.
		_, err := io.CopyN(outFile, src, batchSize)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}
