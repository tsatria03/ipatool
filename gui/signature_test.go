package main

import (
	"archive/zip"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSignature builds an embedded signature with one CodeDirectory per hash type.
func fakeSignature(version uint32, hashTypes ...byte) []byte {
	be := binary.BigEndian
	const cdSize = 44
	header := 12 + 8*len(hashTypes)
	blob := make([]byte, header+cdSize*len(hashTypes))
	be.PutUint32(blob, magicSuperBlob)
	be.PutUint32(blob[4:], uint32(len(blob)))
	be.PutUint32(blob[8:], uint32(len(hashTypes)))
	for i, h := range hashTypes {
		off := header + cdSize*i
		be.PutUint32(blob[12+8*i:], 0x1000*uint32(i)) // slot type (0 = CodeDirectory, 0x1000 = alternate)
		be.PutUint32(blob[16+8*i:], uint32(off))
		be.PutUint32(blob[off:], magicCodeDir)
		be.PutUint32(blob[off+4:], cdSize)
		be.PutUint32(blob[off+8:], version)
		blob[off+37] = h
	}
	return blob
}

// fakeProgram builds a minimal one-architecture Mach-O program with an optional
// code signature. arm64 is 64-bit; anything else is a 32-bit armv7 program.
func fakeProgram(arm64 bool, signature []byte) []byte {
	le := binary.LittleEndian
	headerSize := 28
	magic, cpu := uint32(0xfeedface), uint32(12) // MH_MAGIC, CPU_TYPE_ARM
	if arm64 {
		headerSize, magic, cpu = 32, 0xfeedfacf, 0x0100000c
	}
	ncmds, cmdsSize := 0, 0
	if signature != nil {
		ncmds, cmdsSize = 1, 16
	}
	data := make([]byte, headerSize+cmdsSize)
	le.PutUint32(data, magic)
	le.PutUint32(data[4:], cpu)
	le.PutUint32(data[12:], 2) // MH_EXECUTE
	le.PutUint32(data[16:], uint32(ncmds))
	le.PutUint32(data[20:], uint32(cmdsSize))
	if signature != nil {
		cmd := data[headerSize:]
		le.PutUint32(cmd, loadCodeSignature)
		le.PutUint32(cmd[4:], 16)
		le.PutUint32(cmd[8:], uint32(len(data)))
		le.PutUint32(cmd[12:], uint32(len(signature)))
		data = append(data, signature...)
	}
	return data
}

// fakeFat joins programs into a fat file; the first must be armv7, the second arm64.
func fakeFat(programs ...[]byte) []byte {
	be := binary.BigEndian
	const align = 0x1000
	data := make([]byte, align)
	be.PutUint32(data, 0xcafebabe)
	be.PutUint32(data[4:], uint32(len(programs)))
	for i, p := range programs {
		cpu := uint32(12)
		if i == 1 {
			cpu = 0x0100000c
		}
		entry := data[8+20*i:]
		be.PutUint32(entry, cpu)
		be.PutUint32(entry[8:], uint32(len(data)))
		be.PutUint32(entry[12:], uint32(len(p)))
		be.PutUint32(entry[16:], 12) // 2^12 alignment
		data = append(data, p...)
		for len(data)%align != 0 {
			data = append(data, 0)
		}
	}
	return data
}

// fakeIPA writes an .ipa holding Payload/Test.app with the given main program.
func fakeIPA(t *testing.T, program []byte) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "Test 1.0.ipa")
	out, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(out)
	add := func(name string, data []byte) {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	add("Payload/Test.app/Info.plist", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>TestGame</string></dict></plist>`))
	add("Payload/Test.app/Frameworks/Other.framework/Info.plist", []byte(`<plist version="1.0"><dict/></plist>`))
	if program != nil {
		add("Payload/Test.app/TestGame", program)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	out.Close()
	return file
}

func TestCheckSignature(t *testing.T) {
	sha1, sha256 := byte(1), byte(2)
	tests := []struct {
		name     string
		program  []byte
		has64    bool
		codeDir  uint32
		hashes   string
		warnings []string // phrases, in order
	}{
		// Like Yahoozy 1.2 and Blindie Match 1.40: accepted by iOS 27.
		{"modern fat", fakeFat(fakeProgram(false, fakeSignature(0x20500, sha1, sha256)), fakeProgram(true, fakeSignature(0x20500, sha1, sha256))),
			true, 0x20500, "SHA-1 SHA-256", nil},
		// Like Idle Crafting Kingdom 1.19.208: refused by iOS 27.
		{"old fat", fakeFat(fakeProgram(false, fakeSignature(0x20200, sha1, sha256)), fakeProgram(true, fakeSignature(0x20200, sha1, sha256))),
			true, 0x20200, "SHA-1 SHA-256", []string{"old signature"}},
		{"arm64 only", fakeProgram(true, fakeSignature(0x20400, sha256)), true, 0x20400, "SHA-256", nil},
		// Like Blink Pong and RC Boat Simulator.
		{"32-bit only", fakeProgram(false, fakeSignature(0x20100, sha1)), false, 0x20100, "SHA-1",
			[]string{"32-bit only", "old signature"}},
		{"unsigned", fakeProgram(true, nil), true, 0, "", []string{"no signature"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := checkSignature(fakeIPA(t, tt.program))
			if err != nil {
				t.Fatal(err)
			}
			if info.Program != "TestGame" || info.Has64Bit != tt.has64 || info.CodeDir != tt.codeDir ||
				strings.Join(info.HashTypes, " ") != tt.hashes {
				t.Errorf("got %+v", info)
			}
			w := info.warnings()
			if len(w) != len(tt.warnings) {
				t.Fatalf("warnings = %q, want %d", w, len(tt.warnings))
			}
			for i, phrase := range tt.warnings {
				if !strings.Contains(w[i], phrase) {
					t.Errorf("warning %d = %q, want it to contain %q", i, w[i], phrase)
				}
			}
		})
	}
}

func TestCheckSignatureBadFiles(t *testing.T) {
	notZip := filepath.Join(t.TempDir(), "x.ipa")
	os.WriteFile(notZip, []byte("not a zip"), 0o644)
	if _, err := checkSignature(notZip); err == nil {
		t.Error("a file that isn't a zip should fail")
	}
	if _, err := checkSignature(fakeIPA(t, nil)); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("missing program: err = %v", err)
	}
	if _, err := checkSignature(fakeIPA(t, []byte("garbage"))); err == nil {
		t.Error("a program that isn't Mach-O should fail")
	}
}
