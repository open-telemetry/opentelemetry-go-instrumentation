package process

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"github.com/keyval-dev/opentelemetry-go-instrumentation/pkg/log"
	"strconv"
	"strings"
)

// The build info blob left by the linker is identified by
// a 16-byte header, consisting of buildInfoMagic (14 bytes),
// the binary's pointer size (1 byte),
// and whether the binary is big endian (1 byte).
var buildInfoMagic = []byte("\xff Go buildinf:")

func (a *processAnalyzer) isRegistersABI(f *elf.File) (bool, error) {
	goVersion, err := getGoVersion(f)
	if err != nil {
		return false, err
	}

	log.Logger.V(1).Info("go version detected", "version", goVersion)
	minor, err := getMinorVersion(goVersion)
	if err != nil {
		return false, err
	}

	// Go is using register based ABI since go 1.17
	return minor >= 17, nil
}

func getMinorVersion(goVersion string) (int, error) {
	parts := strings.Split(goVersion, ".")
	if len(parts) < 2 {
		return 0, fmt.Errorf("could not parse go version %s", goVersion)
	}

	num, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	return num, nil
}

func getGoVersion(f *elf.File) (string, error) {
	// Read the first 64kB of text to find the build info blob.
	text := dataStart(f)
	data, err := readData(f, text, 64*1024)
	if err != nil {
		return "", err
	}
	for ; !bytes.HasPrefix(data, buildInfoMagic); data = data[32:] {
		if len(data) < 32 {
			return "", fmt.Errorf("could not detect go version")
		}
	}

	// Decode the blob.
	ptrSize := int(data[14])
	bigEndian := data[15] != 0
	var bo binary.ByteOrder
	if bigEndian {
		bo = binary.BigEndian
	} else {
		bo = binary.LittleEndian
	}

	var readPtr func([]byte) uint64
	if ptrSize == 4 {
		readPtr = func(b []byte) uint64 { return uint64(bo.Uint32(b)) }
	} else {
		readPtr = bo.Uint64
	}
	vers := readString(f, ptrSize, readPtr, readPtr(data[16:]))
	if vers == "" {
		return "", fmt.Errorf("could not detect go version")
	}

	return vers, nil
}

func dataStart(f *elf.File) uint64 {
	for _, s := range f.Sections {
		if s.Name == ".go.buildinfo" {
			return s.Addr
		}
	}
	for _, p := range f.Progs {
		if p.Type == elf.PT_LOAD && p.Flags&(elf.PF_X|elf.PF_W) == elf.PF_W {
			return p.Vaddr
		}
	}
	return 0
}

func readData(f *elf.File, addr, size uint64) ([]byte, error) {
	for _, prog := range f.Progs {
		if prog.Vaddr <= addr && addr <= prog.Vaddr+prog.Filesz-1 {
			n := prog.Vaddr + prog.Filesz - addr
			if n > size {
				n = size
			}
			data := make([]byte, n)
			_, err := prog.ReadAt(data, int64(addr-prog.Vaddr))
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("address not mapped")
}

// readString returns the string at address addr in the executable x.
func readString(f *elf.File, ptrSize int, readPtr func([]byte) uint64, addr uint64) string {
	hdr, err := readData(f, addr, uint64(2*ptrSize))
	if err != nil || len(hdr) < 2*ptrSize {
		return ""
	}
	dataAddr := readPtr(hdr)
	dataLen := readPtr(hdr[ptrSize:])
	data, err := readData(f, dataAddr, dataLen)
	if err != nil || uint64(len(data)) < dataLen {
		return ""
	}
	return string(data)
}
