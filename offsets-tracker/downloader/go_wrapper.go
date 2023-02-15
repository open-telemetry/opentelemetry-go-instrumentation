package downloader

import (
	_ "embed"
	"fmt"
	"github.com/keyval-dev/offsets-tracker/utils"
	"io/fs"
	"io/ioutil"
	"path"
)

const appName = "testapp"

var (
	//go:embed wrapper/go.mod.txt
	goMod string

	//go:embed wrapper/main.go.txt
	goMain string
)

func DownloadBinary(modName string, version string) (string, string, error) {
	dir, err := ioutil.TempDir("", appName)
	if err != nil {
		return "", "", err
	}

	goModContent := fmt.Sprintf(goMod, modName, version)
	err = ioutil.WriteFile(path.Join(dir, "go.mod"), []byte(goModContent), fs.ModePerm)
	if err != nil {
		return "", "", err
	}

	goMainContent := fmt.Sprintf(goMain, modName)
	err = ioutil.WriteFile(path.Join(dir, "main.go"), []byte(goMainContent), fs.ModePerm)
	if err != nil {
		return "", "", err
	}

	err, _, _ = utils.RunCommand("go mod tidy -compat=1.17", dir)
	if err != nil {
		return "", "", err
	}

	err, _, _ = utils.RunCommand("GOOS=linux GOARCH=amd64 go build", dir)
	if err != nil {
		return "", "", err
	}

	return path.Join(dir, appName), dir, nil
}
