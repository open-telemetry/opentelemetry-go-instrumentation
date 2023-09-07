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
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
)

type builder struct {
	log logr.Logger
	cli *client.Client
	Go  *version.Version
}

func newBuilder(l logr.Logger, cli *client.Client, goVer *version.Version) *builder {
	return &builder{log: l, cli: cli, Go: goVer}
}

func (b *builder) image() string {
	return fmt.Sprintf("golang:%s", b.Go.Original())
}

func (b *builder) Build(ctx context.Context, dir string, appV *version.Version) (*app, error) {
	goModTidy := []string{"go", "mod", "tidy", "-compat=1.17"}
	if err := b.run(ctx, goModTidy, dir); err != nil {
		return nil, err
	}

	app := fmt.Sprintf("app%s", appV.Original())
	build := []string{"go", "build", "-o", app}
	if err := b.run(ctx, build, dir); err != nil {
		return nil, err
	}

	return newApp(b.log, appV, filepath.Join(dir, app))
}

func (b *builder) run(ctx context.Context, cmd []string, dir string) error {
	img := b.image()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	b.log.Info("running command", "cmd", cmd, "dir", dir, "image", img)

	rc, err := b.cli.ImagePull(ctx, img, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	rc.Close()

	const appDir = "/usr/src/app"
	resp, err := b.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:      img,
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
	if err != nil {
		return err
	}

	out, err := b.cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return err
	}
	stdcopy.StdCopy(stdout, stderr, out)

	err = b.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	statusCh, errCh := b.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
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

type app struct {
	log  logr.Logger
	ver  *version.Version
	exec string
	bin  *elf.File
	data *dwarf.Data
}

func newApp(l logr.Logger, v *version.Version, exec string) (*app, error) {
	l.Info("loading", "bin", exec, "version", v)
	elfF, err := elf.Open(exec)
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	dwarfData, err := elfF.DWARF()
	if err != nil {
		return nil, err
	}

	return &app{log: l, ver: v, exec: exec, bin: elfF, data: dwarfData}, nil
}

func (a *app) Analyze(sf StructField) structFieldOffset {
	a.log.Info("analyzing app binary", "package", sf.Package, "binary", a.exec, "version", a.ver)
	return sf.offset(a.ver, a.data)
}

func (a *app) Close() error {
	return a.bin.Close()
}
