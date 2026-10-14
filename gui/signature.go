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
	loadEncryption    = 0x21       // LC_ENCRYPTION_INFO
	loadEncryption64  = 0x2c       // LC_ENCRYPTION_INFO_64
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
	AppleID   string   // the Apple ID that downloaded the app, from iTunesMetadata.plist; "" if none
	// Encrypted is true for App Store apps: FairPlay encrypts their main program.
	Encrypted bool
	// HasProfile is true when the app carries a provisioning profile
	// (embedded.mobileprovision), which builds signed for test devices need.
	HasProfile bool
}

// msgNotSigned explains apps that aren't signed for any phone, such as games
// from an engine that doesn't sign them. The phone refuses them ("No code
// signature found").
const msgNotSigned = "This app isn't signed for your iPhone. Apps you build yourself must be signed first, " +
	"for example with Sideloadly."

// msgOtherAppleID warns that an app may not open: the phone needs the keys of
// the Apple ID that downloaded it.
const msgOtherAppleID = "This app was downloaded with a different Apple ID. It will only open on your iPhone " +
	"if that Apple ID has been signed in on it."

// otherAppleID reports whether an app downloaded with fileID belongs to a
// different account than signedIn. Unknown on either side means no.
func otherAppleID(fileID, signedIn string) bool {
	fileID, signedIn = strings.TrimSpace(fileID), strings.TrimSpace(signedIn)
	return fileID != "" && signedIn != "" && !strings.EqualFold(fileID, signedIn)
}

// warnings returns plain-language warnings, or nil when nothing looks wrong.
func (s signatureInfo) warnings() []string {
	var w []string
	if !s.Has64Bit {
		w = append(w, "This app is 32-bit only. iPhones can't run 32-bit apps since iOS 11.")
	}
	switch {
	case !s.Signed, !s.Encrypted && !s.HasProfile:
		// Not from the App Store and not signed for any test device: built by
		// someone and never signed for a phone (or signed only for a Mac).
		w = append(w, msgNotSigned)
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
	var data, metadata []byte
	hasProfile := false
	for _, f := range z.File {
		switch f.Name {
		case app + "embedded.mobileprovision":
			hasProfile = true
		case app + program:
			if data, err = readZipFile(f); err != nil {
				return signatureInfo{}, err
			}
		case "iTunesMetadata.plist":
			metadata, _ = readZipFile(f) // only for the Apple ID; a bad file just means no Apple ID
		}
	}
	if data == nil {
		return signatureInfo{}, fmt.Errorf("the app's main program %q is missing from the .ipa", program)
	}
	info, err := readProgram(data)
	info.Program = program
	info.AppleID = metadataAppleID(metadata)
	info.HasProfile = hasProfile
	return info, err
}

// metadataAppleID returns the Apple ID recorded in an iTunesMetadata.plist
// (binary or XML): ipatool writes "apple-id" and "userName"; iTunes keeps it in
// com.apple.iTunesStore.downloadInfo, accountInfo, AppleID. Self-built apps have
// none.
func metadataAppleID(data []byte) string {
	var meta struct {
		AppleID      string `plist:"apple-id"`
		UserName     string `plist:"userName"`
		DownloadInfo struct {
			AccountInfo struct {
				AppleID string `plist:"AppleID"`
			} `plist:"accountInfo"`
		} `plist:"com.apple.iTunesStore.downloadInfo"`
	}
	if len(data) == 0 {
		return ""
	}
	if _, err := plist.Unmarshal(data, &meta); err != nil {
		return ""
	}
	for _, id := range []string{meta.AppleID, meta.UserName, meta.DownloadInfo.AccountInfo.AppleID} {
		if id = strings.TrimSpace(id); id != "" {
			return id
		}
	}
	return ""
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
		if len(raw) < 16 {
			continue
		}
		switch f.ByteOrder.Uint32(raw) {
		case loadEncryption, loadEncryption64:
			// cryptoff, cryptsize, then cryptid: 0 means not encrypted.
			info.Encrypted = len(raw) >= 20 && f.ByteOrder.Uint32(raw[16:]) != 0
			continue
		case loadCodeSignature:
		default:
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
