package main

import (
	"archive/zip"
	"bytes"
	"debug/macho"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"howett.net/plist"
)

// Before installing, the GUI looks at the app's main program inside the .ipa to
// warn about two things the phone would refuse: apps that are 32-bit only, and
// code signatures in a format newer iOS versions no longer accept. Only the
// file is read; nothing is sent to Apple or to the phone.

const (
	loadCodeSignature = 0x1d       // LC_CODE_SIGNATURE
	magicSuperBlob    = 0xfade0cc0 // CSMAGIC_EMBEDDED_SIGNATURE
	magicCodeDir      = 0xfade0c02 // CSMAGIC_CODEDIRECTORY

	// CodeDirectory version 0x20400 added the executable segment fields. iOS 27
	// refused an app signed with 0x20200 and accepted ones with 0x20500;
	// 0x20300 and 0x20400 are untested, so older than 0x20400 counts as old.
	minCodeDirVersion = 0x20400
)

// signatureInfo is what the check found in an app's main program.
type signatureInfo struct {
	Program   string   // the main program's name
	Has64Bit  bool     // has an arm64 part, so it can run on iOS 11 and later
	Signed    bool     // the 64-bit part (or the only part) has a code signature
	CodeDir   uint32   // newest CodeDirectory version in that signature
	HashTypes []string // hash types in that signature, such as SHA-1 and SHA-256
}

// warnings returns plain-language warnings, or nil when nothing looks wrong.
func (s signatureInfo) warnings() []string {
	var w []string
	if !s.Has64Bit {
		w = append(w, "This app is 32-bit only. iPhones can't run 32-bit apps since iOS 11.")
	}
	switch {
	case !s.Signed:
		w = append(w, "This app has no signature, so your iPhone will probably refuse it.")
	case s.CodeDir < minCodeDirVersion:
		w = append(w, "This app has an old signature. Newer iOS versions refuse some old signatures, so your iPhone may refuse it.")
	}
	return w
}

// checkSignature reads the main program of the .ipa at file.
func checkSignature(file string) (signatureInfo, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return signatureInfo{}, err
	}
	defer z.Close()

	app, program, err := mainProgram(&z.Reader)
	if err != nil {
		return signatureInfo{}, err
	}
	var data []byte
	for _, f := range z.File {
		if f.Name == app+program {
			if data, err = readZipFile(f); err != nil {
				return signatureInfo{}, err
			}
			break
		}
	}
	if data == nil {
		return signatureInfo{}, fmt.Errorf("the app's main program %q is missing from the .ipa", program)
	}
	info, err := readProgram(data)
	info.Program = program
	return info, err
}

// mainProgram finds the app folder (Payload/Name.app/) and the program named in
// its Info.plist.
func mainProgram(z *zip.Reader) (app, program string, err error) {
	for _, f := range z.File {
		dir, name := path.Split(f.Name)
		if name != "Info.plist" || strings.Count(dir, "/") != 2 ||
			!strings.HasPrefix(dir, "Payload/") || !strings.HasSuffix(dir, ".app/") {
			continue
		}
		data, err := readZipFile(f)
		if err != nil {
			return "", "", err
		}
		var info struct {
			Executable string `plist:"CFBundleExecutable"`
		}
		if _, err := plist.Unmarshal(data, &info); err != nil {
			return "", "", fmt.Errorf("reading %s: %w", f.Name, err)
		}
		if info.Executable == "" {
			return "", "", fmt.Errorf("%s doesn't name the app's main program", f.Name)
		}
		return dir, info.Executable, nil
	}
	return "", "", errors.New("this file has no app in it (no Payload/*.app/Info.plist)")
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// readProgram reads a Mach-O program, either one architecture or a fat file
// holding several. The signature reported is the arm64 one when there is one,
// since that is the part a modern iPhone runs.
func readProgram(data []byte) (signatureInfo, error) {
	if fat, err := macho.NewFatFile(bytes.NewReader(data)); err == nil {
		defer fat.Close()
		var info signatureInfo
		found := false
		for _, arch := range fat.Arches {
			slice := data[arch.Offset : arch.Offset+arch.Size]
			s, err := readSlice(arch.File, slice)
			if err != nil {
				return signatureInfo{}, err
			}
			if arch.Cpu == macho.CpuArm64 || !found {
				info = s
				found = true
			}
			if arch.Cpu == macho.CpuArm64 {
				info.Has64Bit = true
			}
		}
		return info, nil
	} else if !errors.Is(err, macho.ErrNotFat) {
		return signatureInfo{}, err
	}
	f, err := macho.NewFile(bytes.NewReader(data))
	if err != nil {
		return signatureInfo{}, err
	}
	defer f.Close()
	return readSlice(f, data)
}

// readSlice reads one architecture. slice holds that architecture's bytes; the
// code signature's offset is counted from its start.
func readSlice(f *macho.File, slice []byte) (signatureInfo, error) {
	info := signatureInfo{Has64Bit: f.Cpu == macho.CpuArm64}
	for _, l := range f.Loads {
		raw := l.Raw()
		if len(raw) < 16 || f.ByteOrder.Uint32(raw) != loadCodeSignature {
			continue
		}
		off, size := f.ByteOrder.Uint32(raw[8:]), f.ByteOrder.Uint32(raw[12:])
		if uint64(off)+uint64(size) > uint64(len(slice)) {
			return info, errors.New("the app's code signature is cut off")
		}
		info.Signed = true
		info.CodeDir, info.HashTypes = readSuperBlob(slice[off : off+size])
	}
	return info, nil
}

// readSuperBlob reads the CodeDirectories in an embedded signature (all values
// big-endian). Apps signed since 2016 carry one with SHA-1 and an alternate one
// with SHA-256.
func readSuperBlob(blob []byte) (version uint32, hashes []string) {
	be := binary.BigEndian
	if len(blob) < 12 || be.Uint32(blob) != magicSuperBlob {
		return 0, nil
	}
	count := be.Uint32(blob[8:])
	for i := uint32(0); i < count; i++ {
		entry := 12 + int(i)*8
		if entry+8 > len(blob) {
			break
		}
		off := int(be.Uint32(blob[entry+4:]))
		if off+38 > len(blob) || be.Uint32(blob[off:]) != magicCodeDir {
			continue
		}
		if v := be.Uint32(blob[off+8:]); v > version {
			version = v
		}
		hashes = append(hashes, hashName(blob[off+37]))
	}
	return version, hashes
}

func hashName(t byte) string {
	switch t {
	case 1:
		return "SHA-1"
	case 2:
		return "SHA-256"
	case 3:
		return "SHA-256 (truncated)"
	case 4:
		return "SHA-384"
	}
	return fmt.Sprintf("hash type %d", t)
}
