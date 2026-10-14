package main

import (
	"archive/zip"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"path/filepath"

	"github.com/danielpaulus/go-ios/ios"
)

// Sending an .ipa to the phone's streaming zip conduit, the service Xcode uses
// to install apps. This follows go-ios's ios/zipconduit package (MIT license,
// github.com/danielpaulus/go-ios) step by step; it is repeated here because
// that package keeps its connection private and can't report how much has been
// sent, which matters for big apps.
//
// The phone wants a zip-like stream it can unpack as it arrives: a plist
// starting the transfer, then every file of the .ipa uncompressed, each after a
// local file header (no data descriptors), a metadata file first, and only the
// central directory's signature at the end. Then the phone answers with
// progress plists until the app is installed or refused.

const zipConduitService = "com.apple.streaming_zip_conduit"

// zipConduitMetadata is META-INF/com.apple.ZipMetadata.plist, sent before the
// app's files: how many records follow and their total size.
type zipConduitMetadata struct {
	StandardDirectoryPerms int
	StandardFilePerms      int
	RecordCount            int
	TotalUncompressedBytes uint64
	Version                int
}

// zipConduitInit starts the transfer, with the values Xcode uses.
type zipConduitInit struct {
	InstallOptionsDictionary    zipConduitOptions
	InstallTransferredDirectory int
	MediaSubdir                 string
	UserInitiatedTransfer       int
}

type zipConduitOptions struct {
	DisableDeltaTransfer int
	InstallDeltaTypeKey  string
	IsUserInitiated      int
	PackageType          string
	PreferWifi           int
}

func newZipConduitInit(file string) zipConduitInit {
	return zipConduitInit{
		InstallTransferredDirectory: 1,
		MediaSubdir:                 "PublicStaging/" + filepath.Base(file),
		InstallOptionsDictionary: zipConduitOptions{
			DisableDeltaTransfer: 1,
			InstallDeltaTypeKey:  "InstallDeltaTypeSparseIPAFiles",
			IsUserInitiated:      1,
			PackageType:          "Customer",
			PreferWifi:           1,
		},
	}
}

// zipLocalHeader is a zip local file header.
type zipLocalHeader struct {
	Signature        uint32
	Version          uint16
	Flags            uint16
	Method           uint16
	ModifiedTime     uint16
	ModifiedDate     uint16
	CRC32            uint32
	CompressedSize   uint32
	UncompressedSize uint32
	NameLength       uint16
	ExtraLength      uint16
}

// zipExtra is the timestamp extra field Xcode sends with every entry.
var zipExtra, _ = hex.DecodeString("55540D0007F3A2EC60F6A2EC60F3A2EC6075780B000104F50100000414000000")

var zipCentralDirectorySignature = []byte{0x50, 0x4b, 0x01, 0x02}

const (
	zipMetadataName = "META-INF/com.apple.ZipMetadata.plist"
	// The permissions Xcode sends: 0o40755 for folders, 0o644 for files.
	zipDirPerms  = 16877
	zipFilePerms = -32348
)

func writeZipEntry(w io.Writer, name string, size, crc uint32) error {
	header := zipLocalHeader{
		Signature: 0x04034b50, Version: 20, ModifiedTime: 0xBDEF, ModifiedDate: 0x52EC,
		CRC32: crc, CompressedSize: size, UncompressedSize: size,
		NameLength: uint16(len(name)), ExtraLength: uint16(len(zipExtra)),
	}
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}
	if _, err := io.WriteString(w, name); err != nil {
		return err
	}
	_, err := w.Write(zipExtra)
	return err
}

// writeZipStream writes the files of z to w in the phone's format. sent is
// called as the files' contents go out, with the bytes sent so far and the
// total.
func writeZipStream(w io.Writer, z *zip.Reader, sent func(done, total uint64)) error {
	var total uint64
	for _, f := range z.File {
		if f.UncompressedSize64 > 0xFFFFFFFF {
			return fmt.Errorf("%s is too big to send (over 4 GB)", f.Name)
		}
		total += f.UncompressedSize64
	}
	metadata := ios.ToPlistBytes(zipConduitMetadata{
		StandardDirectoryPerms: zipDirPerms, StandardFilePerms: zipFilePerms,
		RecordCount: 2 + len(z.File), TotalUncompressedBytes: total, Version: 2,
	})
	if err := writeZipEntry(w, "META-INF/", 0, 0); err != nil {
		return err
	}
	if err := writeZipEntry(w, zipMetadataName, uint32(len(metadata)), crc32.ChecksumIEEE(metadata)); err != nil {
		return err
	}
	if _, err := w.Write(metadata); err != nil {
		return err
	}

	counter := &sentCounter{w: w, total: total, report: sent}
	buffer := make([]byte, 64*1024)
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			if err := writeZipEntry(w, f.Name, 0, 0); err != nil {
				return err
			}
			continue
		}
		if err := writeZipEntry(w, f.Name, uint32(f.UncompressedSize64), f.CRC32); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.CopyBuffer(counter, r, buffer)
		r.Close()
		if err != nil {
			return err
		}
	}
	_, err := w.Write(zipCentralDirectorySignature)
	return err
}

// sentCounter passes writes on and reports the running total.
type sentCounter struct {
	w           io.Writer
	done, total uint64
	report      func(done, total uint64)
}

func (c *sentCounter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.done += uint64(n)
	if c.report != nil {
		c.report(c.done, c.total)
	}
	return n, err
}

// phoneProgress reads one progress plist from the phone: done when the app is
// installed, otherwise the percent and stage, or the phone's error.
func phoneProgress(update map[string]any) (done bool, percent int, status string, err error) {
	if s, ok := update["Status"].(string); ok {
		if s == "DataComplete" {
			return true, 100, s, nil
		}
		return false, 0, "", fmt.Errorf("unexpected install status %q", s)
	}
	dict, ok := update["InstallProgressDict"].(map[string]any)
	if !ok {
		return false, 0, "", fmt.Errorf("unexpected install answer: %v", update)
	}
	if e, ok := dict["Error"]; ok {
		return false, 0, "", fmt.Errorf("the phone refused the app: '%v' errorDescription:'%v'", e, dict["ErrorDescription"])
	}
	switch p := dict["PercentComplete"].(type) {
	case uint64:
		percent = int(p)
	case int64:
		percent = int(p)
	}
	status, _ = dict["Status"].(string)
	return false, percent, status, nil
}

// sendIPA sends the .ipa to the phone, then waits for the phone to install it.
// sent reports the sending; stage reports the phone's install stages after that.
// Closing conn (when cancelling) makes it return.
func sendIPA(conn ios.DeviceConnectionInterface, file string, sent func(done, total uint64), stage func(percent int, status string)) error {
	z, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer z.Close()

	codec := ios.NewPlistCodecReadWriter(conn.Reader(), conn.Writer())
	if err := codec.Write(newZipConduitInit(file)); err != nil {
		return err
	}
	if err := writeZipStream(conn, &z.Reader, sent); err != nil {
		return err
	}
	for {
		var update map[string]any
		if err := codec.Read(&update); err != nil {
			return err
		}
		done, percent, status, err := phoneProgress(update)
		if err != nil || done {
			return err
		}
		if stage != nil {
			stage(percent, status)
		}
	}
}
