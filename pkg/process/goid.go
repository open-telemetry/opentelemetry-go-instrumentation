package process

import (
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"io"
)

const runtimeG = "runtime.g"

func FindGoIDOffset(f *elf.File) error {
	dwarfData, err := f.DWARF()
	if err != nil {
		return err
	}

	reader := dwarfData.Reader()
	err = findRuntimeG(reader)
	if err != nil {
		return err
	}

	return nil
}

func findRuntimeG(reader *dwarf.Reader) error {
	for {
		// Read all entries in sequence
		entry, err := reader.Next()
		if err == io.EOF || entry == nil {
			// We've reached the end of DWARF entries
			break
		}

		if entry.Tag == dwarf.TagStructType {
			// Go through fields
			for _, field := range entry.Field {
				if field.Attr == dwarf.AttrName {
					str := field.Val.(string)
					if str == runtimeG {
						goidLookup(reader)
					}
				}
			}
		}
	}

	return fmt.Errorf("runtime.g not found")
}

func goidLookup(reader *dwarf.Reader) {
	for {
		entry, err := reader.Next()
		if err == io.EOF || entry == nil {
			// We've reached the end of DWARF entries
			break
		}

		for _, field := range entry.Field {
			fmt.Printf("%s %s\n", field.Attr, field.Val)
		}
	}
}
