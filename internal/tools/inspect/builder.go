// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// minCompatVer is the min "go mod" version that includes the "compat" option.
var minCompatVer = semver.New(1, 17, 0, "", "")

// builder builds a Go application into a binary using Docker.
type builder struct {
	log *slog.Logger
	cli *client.Client

	goVer   *semver.Version
	GoImage string
}

// newBuilder returns a builder that will use goVer version of a Go docker
// image to build Go applications. The cli is used to connect to the docker
// interface.
//
// If goVer is nil, the latest version of the Go docker container will be used
// to build applications.
func newBuilder(l *slog.Logger, cli *client.Client, goVer *semver.Version) *builder {
	img := "golang:latest"
	if goVer != nil {
		// Use goVer.String here so 1.12 means 1.12.0. If Original is used, it
		// would mean that the 1.12.17 docker image (which is tagged as the
		// latest "1.12" release) would be used.
		img = "golang:" + goVer.String()
	}
	return &builder{
		log:     l,
		cli:     cli,
		goVer:   goVer,
		GoImage: img,
	}
}

// Build builds the appV version of a Go application located in dir.
func (b *builder) Build(ctx context.Context, dir string, appV *semver.Version, modName string) (string, error) {
	b.log.Debug("building application...", "version", appV, "dir", dir, "image", b.GoImage)

	app := "app" + appV.Original()
	goGetCmd := fmt.Sprintf("go get %s@%s", modName, appV.Original())
	goModTidyCmd := "go mod tidy -compat=1.17"
	var cmd string

	if b.goVer != nil && b.goVer.LessThan(minCompatVer) {
		goModTidyCmd = "go mod tidy"
	}

	if b.goVer == nil {
		cmd = fmt.Sprintf("%s && %s && go build -o %s", goModTidyCmd, goGetCmd, app)
	} else {
		cmd = fmt.Sprintf("%s && go build -o %s", goModTidyCmd, app)
	}

	if err := b.runCmd(ctx, []string{"sh", "-c", cmd}, dir); err != nil {
		return "", err
	}

	b.log.Debug("built application", "version", appV, "dir", dir, "image", b.GoImage)
	return filepath.Join(dir, app), nil
}

func (b *builder) runCmd(ctx context.Context, cmd []string, dir string) error {
	b.log.Debug("running command...", "cmd", cmd, "dir", dir, "image", b.GoImage)

	err := b.pullImage(ctx)
	if err != nil {
		return err
	}

	id, err := b.createContainer(ctx, cmd, dir)
	if err != nil {
		return err
	}

	return b.runContainer(ctx, id)
}

func (b *builder) pullImage(ctx context.Context) error {
	// Do not pull image if already present.
	summaries, err := b.cli.ImageList(ctx, image.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("reference", b.GoImage),
		),
	})
	if err != nil {
		return err
	}
	if len(summaries) > 0 {
		b.log.Debug("using local image", "image", b.GoImage)
		return nil
	}

	pullOpts := image.PullOptions{}
	username := os.Getenv("DOCKER_USERNAME")
	password := os.Getenv("DOCKER_PASSWORD")
	if len(username) > 0 && len(password) > 0 {
		authConfig := registry.AuthConfig{
			Username: username,
			Password: password,
		}
		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			panic(err)
		}
		authStr := base64.URLEncoding.EncodeToString(encodedJSON)
		pullOpts.RegistryAuth = authStr
	}

	rc, err := b.cli.ImagePull(ctx, b.GoImage, pullOpts)
	if err != nil {
		return err
	}
	defer rc.Close()

	out := new(bytes.Buffer)
	_, err = io.Copy(out, rc)
	b.log.Debug("pulling image", "image", b.GoImage, "output", out.String())
	return err
}

func (b *builder) createContainer(ctx context.Context, cmd []string, dir string) (string, error) {
	const appDir = "/usr/src/app"
	resp, err := b.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:      b.GoImage,
			Cmd:        cmd,
			WorkingDir: appDir,
			Tty:        false,
		},
		&container.HostConfig{
			AutoRemove: true,
			Mounts: []mount.Mount{{
				Type:   mount.TypeBind,
				Source: dir,
				Target: appDir,
			}},
		},
		nil,
		nil,
		"",
	)
	return resp.ID, err
}

func (b *builder) runContainer(ctx context.Context, id string) error {
	out, err := b.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}

	err = b.cli.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return err
	}

	statusCh, errCh := b.cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
			_, _ = stdcopy.StdCopy(stdout, stderr, out)
			return &errBuild{
				ReturnCode: status.StatusCode,
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
			}
		}
	}
	return nil
}

type errBuild struct {
	ReturnCode int64
	Stdout     string
	Stderr     string
}

func (e *errBuild) Error() string {
	return fmt.Sprintf("failed to build: (%d) %s", e.ReturnCode, e.Stdout)
}
