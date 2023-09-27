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
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
)

// minCompatVer is the min "go mod" version that includes the "compat" option.
var minCompatVer = version.Must(version.NewVersion("1.17.0"))

type builder struct {
	log logr.Logger
	cli *client.Client

	goVer   *version.Version
	GoImage string
}

func newBuilder(l logr.Logger, cli *client.Client, goVer *version.Version) *builder {
	img := "golang:latest"
	if goVer != nil {
		img = fmt.Sprintf("golang:%s", goVer.Original())
	}
	return &builder{
		log:     l.WithName("builder"),
		cli:     cli,
		goVer:   goVer,
		GoImage: img,
	}
}

func (b *builder) Build(ctx context.Context, dir string, appV *version.Version) (string, error) {
	b.log.V(2).Info("building application...", "version", appV, "dir", dir, "image", b.GoImage)

	app := fmt.Sprintf("app%s", appV.Original())
	var cmd string
	if b.goVer.LessThan(minCompatVer) {
		cmd = "go mod tidy && go build -o " + app
	} else {
		cmd = "go mod tidy -compat=1.17 && go build -o " + app
	}
	if err := b.runCmd(ctx, []string{"sh", "-c", cmd}, dir); err != nil {
		return "", err
	}

	b.log.V(1).Info("built application", "version", appV, "dir", dir, "image", b.GoImage)
	return filepath.Join(dir, app), nil
}

func (b *builder) runCmd(ctx context.Context, cmd []string, dir string) error {
	b.log.V(2).Info("running command...", "cmd", cmd, "dir", dir, "image", b.GoImage)

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
	summaries, err := b.cli.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("reference", b.GoImage),
		),
	})
	if err != nil {
		return err
	}
	if len(summaries) > 0 {
		b.log.V(2).Info("using local image", "image", b.GoImage)
		return nil
	}

	rc, err := b.cli.ImagePull(ctx, b.GoImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()

	out := new(bytes.Buffer)
	_, err = io.Copy(out, rc)
	b.log.V(1).Info("pulling image", "image", b.GoImage, "output", out.String())
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
	out, err := b.cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}

	err = b.cli.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	statusCh, errCh := b.cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return ctx.Err()
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
