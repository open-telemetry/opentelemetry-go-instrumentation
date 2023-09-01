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

package inspect

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
)

const goBinaryURL = "https://go.dev/dl/go%s.linux-amd64.tar.gz"

type storage struct {
	log logr.Logger

	root string
	sdk  string
}

func newStorage(l logr.Logger, path string) (*storage, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	s := &storage{
		log:  l,
		root: path,
		sdk:  filepath.Join(path, "sdk"),
	}

	err = os.MkdirAll(s.sdk, 0755)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *storage) getGo(ver string) (string, error) {
	goVer := fmt.Sprintf("go%s", ver)
	dest := filepath.Join(s.sdk, goVer)
	bin := filepath.Join(dest, "bin/go")
	if _, err := os.Stat(bin); errors.Is(err, os.ErrNotExist) {
		s.log.Info("Go version not found locally", "version", ver, "path", bin)
		err = s.download(dest, ver)
		if err != nil {
			return "", err
		}
	} else {
		s.log.Info("Go version found locally", "version", ver, "path", bin)
	}

	return bin, nil

}

func (s *storage) download(dest, ver string) error {
	err := os.MkdirAll(dest, 0755)
	if err != nil {
		return err
	}

	s.log.Info("downloading Go binary", "version", ver)
	url := fmt.Sprintf(goBinaryURL, ver)
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Add("Accept-Encoding", "gzip")

	s.log.Info("downloading Go binary", "version", ver, "url", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		r, err = gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer r.Close()
	default:
		r = resp.Body
	}

	return s.untar(tar.NewReader(r), dest, 1)
}

func (s *storage) untar(r *tar.Reader, dest string, strip int) error {
	name := func(s string) string {
		for i := 0; i < strip; i++ {
			if i := strings.IndexByte(s, filepath.Separator); i >= 0 {
				s = s[i+1:]
			}
		}
		return filepath.Join(dest, s)
	}

	flag := os.O_RDWR | os.O_CREATE | os.O_TRUNC
	for {
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		n := name(h.Name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(n, h.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			d, _ := filepath.Split(n)
			if err := os.MkdirAll(d, 0755); err != nil {
				return err
			}

			err = func() error {
				f, err := os.OpenFile(n, flag, h.FileInfo().Mode())
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(f, r)
				return err
			}()
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown tar type: %s (%v)", h.Name, h.Typeflag)
		}
	}
}
