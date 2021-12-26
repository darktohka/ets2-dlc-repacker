package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math"
	"regexp"
	"strings"

	"os"
	"path"

	"github.com/Luzifer/rconfig/v2"
	"github.com/darktohka/ets2-dlc-repacker/scs"
	log "github.com/sirupsen/logrus"
)

var (
	cfg = struct {
		LogLevel       string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	Version = "v0.1.0"

	DLCPrefix       = "dlc_"
	ExtensionSuffix = ".scs"
	ManifestSuffix  = ".manifest.sii"
	DLCFiles        = []string{"pcg", "rocket_league", "metallics", "phys_flags", "rims", "hs_schoch", "toys", "oversize"}

	ZlibBestCompression = []byte{0x78, 0xDA}
	NameRegexp          = regexp.MustCompile("display_name:\\s+[\"']([^\"']+)[\"']")
	VersionRegexp       = regexp.MustCompile("(package|compatible)_version(s)?(\\[\\])?:\\s+[\"']([^\"']+)[\"']")
)

func init() {
	if err := rconfig.ParseAndValidate(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("ets2-dlc-repacker %s by Disyer\n", Version)
		os.Exit(0)
	}

	if l, err := log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}
}

func IsInputPiped() bool {
	stdin, err := os.Stdin.Stat()

	return err != nil || (stdin.Mode()&os.ModeCharDevice) == 0
}

func WaitForKeyboard() {
	if !IsInputPiped() {
		log.Info("Press any key to close this window...")
		fmt.Scanln()
	}

	os.Exit(0)
}

func FindVersionFromManifest(manifest []byte) []byte {
	matches := VersionRegexp.FindSubmatch(manifest)

	if matches == nil || len(matches) < 5 {
		log.Fatal("No version found in DLC file")
	}

	return matches[4]
}

func FindDLCNameFromManifest(manifest []byte) string {
	matches := NameRegexp.FindSubmatch(manifest)

	if matches == nil || len(matches) < 2 {
		return "Unknown DLC"
	}

	return string(matches[1])
}

func FindDLCVersion(folder string, dlcName string) []byte {
	f, err := os.Open(path.Join(folder, DLCPrefix+dlcName+ExtensionSuffix))

	if err != nil {
		log.WithError(err).Fatal("Unable to open input DLC file")
	}

	defer f.Close()

	reader, err := scs.NewReader(f, 0)

	if err != nil {
		log.WithError(err).Fatal("Unable to read SCS file headers")
	}

	for _, file := range reader.Files {
		if !strings.HasSuffix(file.Name, ManifestSuffix) {
			continue
		}

		src, err := file.Open()

		if err != nil {
			log.WithError(err).Fatal("Unable to open file from archive")
		}

		defer src.Close()

		manifest := make([]byte, file.Size)

		if _, err := src.Read(manifest); err != nil && !errors.Is(err, io.EOF) {
			log.WithError(err).Fatal("Unable to copy byte array into memory")
		}

		return FindVersionFromManifest(manifest)
	}

	log.Fatal("Unable to find manifest in example DLC file")
	return nil
}

func FindDLCVersionFromFolder(folder string) []byte {
	for _, dlcName := range DLCFiles {
		var dlcFilename = DLCPrefix + dlcName + ExtensionSuffix

		if _, err := os.Stat(path.Join(folder, dlcFilename)); errors.Is(err, os.ErrNotExist) {
			continue
		}

		return FindDLCVersion(folder, dlcName)
	}

	log.Warn("No DLC files have been found in " + folder + ". Are you sure this is a valid game installation?")
	WaitForKeyboard()
	return nil
}

func FindLargestOffset(reader *scs.Reader) int64 {
	var largestOffset int64 = math.MinInt64

	for _, file := range reader.Files {
		if file.Offset > largestOffset {
			largestOffset = file.Offset
		}
	}

	return largestOffset
}

func DeflateBytes(data []byte) ([]byte, error) {
	var compressedBuffer bytes.Buffer
	compressedBuffer.Write(ZlibBestCompression)

	writer, err := flate.NewWriter(&compressedBuffer, flate.BestCompression)

	if err != nil {
		return nil, err
	}

	_, err = io.Copy(writer, bytes.NewReader(data))
	writer.Flush()
	writer.Close()
	return compressedBuffer.Bytes(), err
}

func RepackDLC(filename string, targetVersion []byte) {
	f, err := os.Open(filename)

	if err != nil {
		log.WithError(err).Fatal("Unable to open input file")
	}

	defer f.Close()

	reader, err := scs.NewReader(f, 0)

	if err != nil {
		log.WithField("filename", filename).WithError(err).Fatal("Unable to read SCS file headers")
	}

	for _, file := range reader.Files {
		if !strings.HasSuffix(file.Name, ManifestSuffix) {
			continue
		}

		src, err := file.Open()

		if err != nil {
			log.WithError(err).Fatal("Unable to open file from archive")
		}

		defer src.Close()

		manifest := make([]byte, file.Size)

		if _, err := src.Read(manifest); err != nil && !errors.Is(err, io.EOF) {
			log.WithError(err).Fatal("Unable to copy byte array into memory")
		}

		version := FindVersionFromManifest(manifest)

		if bytes.Equal(version, targetVersion) {
			return
		}

		dlcName := FindDLCNameFromManifest(manifest)
		log.Info("Updating " + dlcName + " (" + path.Base(filename) + ") from version " + string(version) + " to version " + string(targetVersion) + "...")

		manifest = bytes.ReplaceAll(manifest, version, targetVersion)
		compressed, err := DeflateBytes(manifest)

		if err != nil {
			log.WithError(err).Fatal("Failed to compress")
		}

		if file.ZSize < int32(len(compressed)) && file.Offset != FindLargestOffset(reader) {
			stat, err := os.Stat(filename)

			if err != nil {
				log.WithError(err).Fatal("Failed to find the length of " + filename)
			}

			file.CatalogEntry.Offset = stat.Size()
		}

		file.CatalogEntry.CRC = crc32.ChecksumIEEE(manifest)
		file.CatalogEntry.Size = int32(len(manifest))
		file.CatalogEntry.ZSize = int32(len(compressed))
		file.CatalogEntry.Type = scs.EntryTypeCompressedFileCopy

		var headerBuffer bytes.Buffer
		binary.Write(&headerBuffer, binary.LittleEndian, file.CatalogEntry)
		header := headerBuffer.Bytes()

		if len(header) != 32 {
			log.Fatal("Generated catalog header size is not 32 bytes")
		}

		f, err := os.OpenFile(filename, os.O_RDWR, 0644)

		if err != nil {
			log.WithError(err).Fatal("Unable to open file for writing: " + filename)
		}

		n, err := f.WriteAt(compressed, file.CatalogEntry.Offset)

		if err != nil {
			log.WithError(err).Fatal("Error appending new manifest")
		}

		if n != len(compressed) {
			log.WithField("expected", len(compressed)).WithField("wrote", n).Fatal("Did not write enough bytes for new manifest")
		}

		n, err = f.WriteAt(header, file.CatalogOffset)

		if err != nil {
			log.WithError(err).Fatal("Unable to patch manifest header")
		}

		if n != len(header) {
			log.WithField("expected", len(header)).WithField("wrote", n).Fatal("Did not write enough bytes for new manifest")
		}

		break
	}
}

func RepackDLCsInFolder(folder string) {
	if _, err := os.Stat(folder); errors.Is(err, os.ErrNotExist) {
		log.Fatal("Folder does not exist")
	}

	log.Info("Repacking DLC files in " + folder + "...")

	version := FindDLCVersionFromFolder(folder)

	if version == nil {
		return
	}

	files, err := ioutil.ReadDir(folder)

	if err != nil {
		log.WithError(err).Fatal("Could not enumerate files in directory")
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()

		if !strings.HasPrefix(name, DLCPrefix) || !strings.HasSuffix(name, ExtensionSuffix) {
			continue
		}

		RepackDLC(path.Join(folder, name), version)
	}

	log.Info("Updated all DLC files to version " + string(version) + ".")
	WaitForKeyboard()
}

func main() {
	var folder string

	switch len(rconfig.Args()) {
	case 0:
	case 1:
		// No positional arguments
		dir, err := os.Getwd()

		if err != nil {
			log.WithError(err).Fatal("Unable to find current working directory")
		}

		folder = dir
	default:
		folder = rconfig.Args()[1]
	}

	RepackDLCsInFolder(folder)
}
