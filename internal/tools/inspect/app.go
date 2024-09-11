// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"context"
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/hashicorp/go-version"

	"go.opentelemetry.io/auto/internal/pkg/structfield"
)

// app holds a built Go application.
type app struct {
	Renderer Renderer
	Builder  *builder
	AppVer   *version.Version
	Fields   []structfield.ID

	log    *slog.Logger
	tmpDir string
	exec   string
	data   *dwarf.Data
}

// newApp builds and returns a new app.
//
// The new app is built in a temp directory. It is up to the caller to ensure
// the returned app's Close method is called when it is no longer needed so
// all temp directory resources are cleaned up.
func newApp(ctx context.Context, l *slog.Logger, j job) (*app, error) {
	a := &app{
		Renderer: j.Renderer,
		Builder:  j.Builder,
		AppVer:   j.AppVer,
		Fields:   j.Fields,
		log:      l,
	}

	var err error
	a.tmpDir, err = os.MkdirTemp("", "inspect-*")
	if err != nil {
		return nil, err
	}

	data := struct{ Version string }{
		Version: "v" + a.AppVer.String(),
	}
	if err = j.Renderer.Render(a.tmpDir, data); err != nil {
		return nil, err
	}

	if len(j.Fields) == 0 {
		return nil, errors.New("no fields to analyze")
	}
	modName := j.Fields[0].ModPath

	a.exec, err = j.Builder.Build(ctx, a.tmpDir, a.AppVer, modName)
	if err != nil {
		return nil, err
	}

	elfF, err := elf.Open(a.exec)
	if err != nil {
		return nil, err
	}
	defer elfF.Close()

	a.data, err = elfF.DWARF()
	if err != nil {
		return nil, err
	}

	a.log.Debug("built app", "binary", a.exec)
	return a, nil
}

// GetOffset returnst the struct field offset for sf. It uses the DWARF data
// of the app's built binary to find this value.
func (a *app) GetOffset(id structfield.ID) (uint64, bool) {
	a.log.Debug("analyzing binary...", "id", id, "binary", a.exec)

	strct := fmt.Sprintf("%s.%s", id.PkgPath, id.Struct)
	r := a.data.Reader()
	if !gotoEntry(r, dwarf.TagStructType, strct) {
		return 0, false
	}

	e, err := findEntryInChildren(r, dwarf.TagMember, id.Field)
	if err != nil {
		return 0, false
	}

	f, ok := entryField(e, dwarf.AttrDataMemberLoc)
	if !ok {
		return 0, false
	}

	return uint64(f.Val.(int64)), true
}

// Close closes the app, releasing all held resources.
func (a *app) Close() error {
	return os.RemoveAll(a.tmpDir)
}

// gotoEntry reads from r until the entry with a tag equal to name is found.
// True is returned if the entry is found, otherwise false is returned.
func gotoEntry(r *dwarf.Reader, tag dwarf.Tag, name string) bool {
	_, err := findEntry(r, tag, name)
	return err == nil
}

// findEntry returns the DWARF entry with a tag equal to name read from r. An
// error is returned if the entry cannot be found.
func findEntry(r *dwarf.Reader, tag dwarf.Tag, name string) (*dwarf.Entry, error) {
	for {
		entry, err := r.Next()
		if err == io.EOF || entry == nil {
			break
		}

		if entry.Tag == tag {
			if f, ok := entryField(entry, dwarf.AttrName); ok {
				if name == f.Val.(string) {
					return entry, nil
				}
			}
		}
	}
	return nil, errors.New("not found")
}

// findEntryInChildren returns the DWARF entry with a tag equal to name read from r, only
// considering the children of the current entry. An error is returned if the entry cannot be found.
func findEntryInChildren(r *dwarf.Reader, tag dwarf.Tag, name string) (*dwarf.Entry, error) {
	for {
		entry, err := r.Next()
		if err == io.EOF || entry == nil || entry.Tag == 0 {
			break
		}

		if entry.Tag == tag {
			if f, ok := entryField(entry, dwarf.AttrName); ok {
				if name == f.Val.(string) {
					return entry, nil
				}
			}
		}
	}
	return nil, errors.New("not found")
}

// entryField returns the DWARF field from DWARF entry e that has the passed
// DWARF attribute a.
func entryField(e *dwarf.Entry, a dwarf.Attr) (dwarf.Field, bool) {
	for _, f := range e.Field {
		if f.Attr == a {
			return f, true
		}
	}
	return dwarf.Field{}, false
}
