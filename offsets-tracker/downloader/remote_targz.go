package downloader

import (
	"fmt"
	"github.com/keyval-dev/offsets-tracker/utils"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

const (
	urlPattern = "https://go.dev/dl/go%s.linux-amd64.tar.gz"
)

func DownloadBinaryFromRemote(modName string, version string) (string, string, error) {
	dir, err := ioutil.TempDir("", version)
	if err != nil {
		return "", "", err
	}
	dest, err := os.Create(path.Join(dir, "go.tar.gz"))
	if err != nil {
		return "", "", err
	}
	defer dest.Close()

	resp, err := http.Get(fmt.Sprintf(urlPattern, version))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	_, err = io.Copy(dest, resp.Body)
	if err != nil {
		return "", "", err
	}

	err, _, _ = utils.RunCommand("tar -xf go.tar.gz -C .", dir)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s/go/bin/go", dir), dir, nil
}
