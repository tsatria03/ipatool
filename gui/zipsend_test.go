package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"strings"
	"testing"

	"howett.net/plist"
)

// streamEntry is one entry read back from a stream written by writeZipStream.
type streamEntry struct {
	name string
	crc  uint32
	data []byte
}

// readStream splits the phone stream into entries and checks it ends with the
// central directory signature and nothing after it.
func readStream(t *testing.T, stream []byte) []streamEntry {
	t.Helper()
	le := binary.LittleEndian
	var entries []streamEntry
	i := 0
	for i+4 <= len(stream) && le.Uint32(stream[i:]) == 0x04034b50 {
		if le.Uint16(stream[i+8:]) != 0 || le.Uint16(stream[i+6:]) != 0 {
			t.Fatalf("entry at %d is compressed or uses data descriptors", i)
		}
		crc, size := le.Uint32(stream[i+14:]), int(le.Uint32(stream[i+22:]))
		if int(le.Uint32(stream[i+18:])) != size {
			t.Fatalf("entry at %d: compressed and uncompressed sizes differ", i)
		}
		nameLen, extraLen := int(le.Uint16(stream[i+26:])), int(le.Uint16(stream[i+28:]))
		start := i + 30 + nameLen + extraLen
		entries = append(entries, streamEntry{string(stream[i+30 : i+30+nameLen]), crc, stream[start : start+size]})
		i = start + size
	}
	if !bytes.Equal(stream[i:], zipCentralDirectorySignature) {
		t.Fatalf("stream ends with %x, want only the central directory signature", stream[i:])
	}
	return entries
}

func TestWriteZipStream(t *testing.T) {
	var ipa bytes.Buffer
	z := zip.NewWriter(&ipa)
	big := bytes.Repeat([]byte("game data "), 30000) // 300 KB, several buffer copies
	files := []struct {
		name string
		data []byte
	}{{"Payload/", nil}, {"Payload/Test.app/", nil}, {"Payload/Test.app/Info.plist", []byte("<plist/>")},
		{"Payload/Test.app/TestGame", big}, {"iTunesMetadata.plist", []byte("x")}}
	for _, f := range files {
		w, err := z.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(f.data)
	}
	z.Close()
	r, err := zip.NewReader(bytes.NewReader(ipa.Bytes()), int64(ipa.Len()))
	if err != nil {
		t.Fatal(err)
	}

	var reports [][2]uint64
	var stream bytes.Buffer
	if err := writeZipStream(&stream, r, func(done, total uint64) {
		reports = append(reports, [2]uint64{done, total})
	}); err != nil {
		t.Fatal(err)
	}
	entries := readStream(t, stream.Bytes())

	wantNames := []string{"META-INF/", zipMetadataName}
	for _, f := range files {
		wantNames = append(wantNames, f.name)
	}
	if len(entries) != len(wantNames) {
		t.Fatalf("got %d entries, want %d", len(entries), len(wantNames))
	}
	for i, e := range entries {
		if e.name != wantNames[i] {
			t.Errorf("entry %d is %q, want %q", i, e.name, wantNames[i])
		}
		if e.crc != crc32.ChecksumIEEE(e.data) {
			t.Errorf("%s: CRC doesn't match its data", e.name)
		}
		if i >= 2 && !bytes.Equal(e.data, files[i-2].data) {
			t.Errorf("%s: data changed on the way", e.name)
		}
	}

	var meta zipConduitMetadata
	if _, err := plist.Unmarshal(entries[1].data, &meta); err != nil {
		t.Fatal(err)
	}
	total := uint64(len("<plist/>") + len(big) + 1)
	if meta.RecordCount != 2+len(files) || meta.TotalUncompressedBytes != total || meta.Version != 2 {
		t.Errorf("metadata = %+v", meta)
	}

	if len(reports) < 3 {
		t.Fatalf("only %d progress reports", len(reports))
	}
	for i, rep := range reports {
		if rep[1] != total || (i > 0 && rep[0] < reports[i-1][0]) {
			t.Fatalf("report %d = %v, want rising counts of %d", i, rep, total)
		}
	}
	if last := reports[len(reports)-1]; last[0] != total {
		t.Errorf("last report = %v, want all %d bytes", last, total)
	}
}

func TestPhoneProgress(t *testing.T) {
	done, percent, status, err := phoneProgress(map[string]any{
		"InstallProgressDict": map[string]any{"PercentComplete": uint64(40), "Status": "VerifyingApplication"}})
	if done || percent != 40 || status != "VerifyingApplication" || err != nil {
		t.Errorf("progress = %v %d %q %v", done, percent, status, err)
	}
	if done, _, _, err := phoneProgress(map[string]any{"Status": "DataComplete"}); !done || err != nil {
		t.Errorf("DataComplete = %v %v, want done", done, err)
	}
	// The refusal iOS 27 sent for Idle Crafting Kingdom 1.19.208.
	_, _, _, err = phoneProgress(map[string]any{"InstallProgressDict": map[string]any{
		"Error": "ApplicationVerificationFailed",
		"ErrorDescription": "Failed to verify code signature of /var/installd/.../CraftingKingdom.app : " +
			"0xe8008029 (The code signature version is no longer supported.)"}})
	if err == nil || !strings.Contains(installErrorText(err), "signature is too old") {
		t.Errorf("refusal: err = %v, message = %q", err, installErrorText(err))
	}
	if _, _, _, err := phoneProgress(map[string]any{"Other": 1}); err == nil {
		t.Error("an unexpected answer should be an error")
	}
}

func TestInstallProgressText(t *testing.T) {
	tests := []struct {
		r    installProgress
		want string
	}{
		{installProgress{Sending: true, Percent: 45, Sent: 120_400_000, Total: 260_000_000},
			"Sending to Alex's iPhone: 45 percent, 120 of 260 MB."},
		{installProgress{Sending: true, Percent: 3, Sent: 1_260_000, Total: 45_400_000},
			"Sending to Alex's iPhone: 3 percent, 1.3 of 45 MB."},
		{installProgress{Percent: 40, Stage: "Checking the app's signature"},
			"Installing on Alex's iPhone: 40 percent, checking the app's signature."},
	}
	for _, tt := range tests {
		if got := installProgressText("Alex's iPhone", tt.r); got != tt.want {
			t.Errorf("installProgressText(%+v) = %q, want %q", tt.r, got, tt.want)
		}
	}
}
