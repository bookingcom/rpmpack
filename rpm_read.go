package rpmpack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cavaliergopher/cpio"
	"github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

func readSignatures(file *os.File, out *RPM) error {
	signatures, err := ReadHeader(file, signatures)
	if err != nil {
		return err
	}

	if signatures == nil {
		return fmt.Errorf("signatures header is nil")
	}

	out.signatures = signatures
	return nil
}

func seekFile(file *os.File) error {
	offset, err := file.Seek(0, 1) // advance 0 from current position

	if err != nil {
		return fmt.Errorf("failed to get offset from begging of file: %w", err)
	}

	if offset % 8 == 0 {
		return nil
	}

	newOffset, err := file.Seek(offset % 8, 1) // advance padding from current position
	if err != nil {
		return fmt.Errorf("failed to advance file by %d bytes: %w", offset % 8, err)
	}

	targetOffset := int64(offset / 8) * 8 + 8
	if newOffset != targetOffset {
		return fmt.Errorf("new offset is not matching %d expected %d", newOffset, targetOffset)
	}

	return nil
}

func readHeaders(file *os.File, out *RPM) error {
	headers, err := ReadHeader(file, immutable)
	if err != nil {
		return err
	}

	if headers == nil {
		return fmt.Errorf("immutable headers is nil")
	}
	out.headers = headers
	return nil
}

func readGenIndexes(out *RPM) {
	out.Name, _ = popTag(out.headers.entries, tagName, IndexEntry.toString)
	out.Summary, _ = popTag(out.headers.entries, tagSummary, IndexEntry.toString)
	out.Description, _ = popTag(out.headers.entries, tagDescription, IndexEntry.toString)
	out.Version, _ = popTag(out.headers.entries, tagVersion, IndexEntry.toString)
	out.Release, _ = popTag(out.headers.entries, tagRelease, IndexEntry.toString)
	out.Arch, _ = popTag(out.headers.entries, tagArch, IndexEntry.toString)
	out.OS, _ = popTag(out.headers.entries, tagOS, IndexEntry.toString)
	out.Vendor, _ = popTag(out.headers.entries, tagVendor, IndexEntry.toString)
	out.URL, _ = popTag(out.headers.entries, tagURL, IndexEntry.toString)
	out.Packager, _ = popTag(out.headers.entries, tagPackager, IndexEntry.toString)
	out.Group, _ = popTag(out.headers.entries, tagGroup, IndexEntry.toString)
	out.Licence, _ = popTag(out.headers.entries, tagLicence, IndexEntry.toString)
	out.BuildHost, _ = popTag(out.headers.entries, tagBuildHost, IndexEntry.toString)
	out.Compressor, _ = popTag(out.headers.entries, tagPayloadCompressor, IndexEntry.toString)
	out.Epoch, _ = popTag(out.headers.entries, tagEpoch, IndexEntry.toUint32)
	out.BuildTime, _ = popTag(out.headers.entries, tagBuildTime, IndexEntry.toTime)
	out.Prefixes, _ = popTag(out.headers.entries, tagPrefixes, IndexEntry.toStringArray)
	out.SourcePackage, _ = popTag(out.headers.entries, tagSourceRPM, IndexEntry.toString)

	out.Provides, _ = out.headers.toRelations(tagProvides, tagProvideVersion, tagProvideFlags)
	out.Obsoletes, _ = out.headers.toRelations(tagObsoletes, tagObsoleteVersion, tagObsoleteFlags)
	out.Suggests, _ = out.headers.toRelations(tagSuggests, tagSuggestVersion, tagSuggestFlags)
	out.Recommends, _ = out.headers.toRelations(tagRecommends, tagRecommendVersion, tagRecommendFlags)
	out.Requires, _ = out.headers.toRelations(tagRequires, tagRequireVersion, tagRequireFlags)
	out.Conflicts, _ = out.headers.toRelations(tagConflicts, tagConflictVersion, tagConflictFlags)
	out.Changelog, _ = out.headers.toChangelog()
}

func readFileIndexes(out *RPM) {
	out.basenames, _ = popTag(out.headers.entries, tagBasenames, IndexEntry.toStringArray)
	out.dirindexes, _ = popTag(out.headers.entries, tagDirindexes, IndexEntry.toUint32Array)
	out.di = newDirIndex()
	out.di.l, _ = popTag(out.headers.entries, tagDirnames, IndexEntry.toStringArray)

	out.filesizes, _ = popTag(out.headers.entries, tagFileSizes, IndexEntry.toUint32Array)
	out.filemodes, _ = popTag(out.headers.entries, tagFileModes, IndexEntry.toUint16Array)
	out.fileowners, _ = popTag(out.headers.entries, tagFileUserName, IndexEntry.toStringArray)
	out.filegroups, _ = popTag(out.headers.entries, tagFileGroupName, IndexEntry.toStringArray)
	out.filemtimes, _ = popTag(out.headers.entries, tagFileMTimes, IndexEntry.toUint32Array)
	out.filedigests, _ = popTag(out.headers.entries, tagFileDigests, IndexEntry.toStringArray)
	out.filelinktos, _ = popTag(out.headers.entries, tagFileLinkTos, IndexEntry.toStringArray)
	out.fileflags, _ = popTag(out.headers.entries, tagFileFlags, IndexEntry.toUint32Array)

	popTag(out.headers.entries, tagFileINodes, IndexEntry.toInt32Array)
	popTag(out.headers.entries, tagFileDigestAlgo, IndexEntry.toInt32Array)
	popTag(out.headers.entries, tagFileVerifyFlags, IndexEntry.toInt32Array)
	popTag(out.headers.entries, tagFileRDevs, IndexEntry.toInt16Array)
	popTag(out.headers.entries, tagFileLangs, IndexEntry.toStringArray)
}

func readScript(data *RPM, tagScript int, tagProgram int, name string) (string, error){
	script, _ := popTag(data.headers.entries, tagPretrans, IndexEntry.toString)
	pretransProg, _ := popTag(data.headers.entries, tagPretransProg, IndexEntry.toString)

	if script != "" && pretransProg != "/bin/sh" {
		return "", fmt.Errorf("%s script %s does not match expected script %s", name, pretransProg, "/bin/sh")
	}
	return script, nil
}

func readScripts(out *RPM) error {
	var err error
	out.pretrans, err = readScript(out, tagPretrans, tagPretransProg, "pretrans")
	if err != nil {
		return err
	}

	out.prein, err = readScript(out, tagPrein, tagPreinProg, "prein")
	if err != nil {
		return err
	}

	out.postin, err = readScript(out, tagPostin, tagPostinProg, "postin")
	if err != nil {
		return err
	}

	out.preun, err = readScript(out, tagPreun, tagPreunProg, "preun")
	if err != nil {
		return err
	}

	out.postun, err = readScript(out, tagPostun, tagPostunProg, "postun")
	if err != nil {
		return err
	}

	out.posttrans, err = readScript(out, tagPosttrans, tagPosttransProg, "posttrans")
	if err != nil {
		return err
	}

	return nil
}

func ReadLead(inp io.Reader) (*Lead, error) {
	out := &Lead{}

	if val, err := readExactly(inp, 4); err != nil {
		return nil, fmt.Errorf("failed to read magic: %v", err)
	} else {
		out.magic = ([4]byte)(val)
	}

	if string(out.magic[:]) != "\xed\xab\xee\xdb" {
		return nil, fmt.Errorf("not a valid RPM file")
	}

	version, err := io.ReadAll(io.LimitReader(inp, 2))
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %v", err)
	}
	out.major = uint8(version[0])
	out.minor = uint8(version[1])
	if out.major == 0 && out.minor == 0 {
		return nil, fmt.Errorf("unsupported rpm version %d.%d", out.major, out.minor)
	}

	if out.typeFile, err = readUint16(inp); err != nil {
		return nil, fmt.Errorf("failed to read typeFile: %v", err)
	}

	if out.archNum, err = readUint16(inp); err != nil {
		return nil, fmt.Errorf("failed to read archNum: %v", err)
	}

	if out.name, err = readString(inp, 66); err != nil {
		return nil, fmt.Errorf("failed to read name: %v", err)
	}

	out.nameFromBinary = true

	if out.osnum, err = readUint16(inp); err != nil {
		return nil, fmt.Errorf("failed to read osnum: %v", err)
	}

	if out.signatureType, err = readUint16(inp); err != nil {
		return nil, fmt.Errorf("failed to read signatureType: %v", err)
	}

	var reserved []byte
	if reserved, err = readExactly(inp, 16); err != nil {
		return nil, fmt.Errorf("failed to read reserved: %v", err)
	}
	out.reserved = ([16]byte)(reserved)

	for _, b := range out.reserved {
		if b != 0 {
			return nil, fmt.Errorf("reserved bytes not zero")
		}
	}

	return out, nil
}

func setupDecompressor(compressorSetting string, r io.Reader) (rc io.Reader, err error) {

	parts := strings.Split(compressorSetting, ":")
	if len(parts) > 2 {
		return nil, fmt.Errorf("malformed compressor setting: %s", compressorSetting)
	}

	compressorType := parts[0]

	switch compressorType {
	case "":
		compressorType = "gzip"
		fallthrough
	case "gzip":
		rc, err = gzip.NewReader(r)
	case "lzma":
		rc, err = lzma.NewReader(r)
	case "xz":
		rc, err = xz.NewReader(r)
	case "zstd":
		rc, err = zstd.NewReader(r)
	default:
		return nil, fmt.Errorf("unknown compressor type: %s", compressorType)
	}

	return rc, err
}

func readFile(r *cpio.Reader, i int, out *RPM) (error) {
	h, err := r.Next()
	if err != nil {
		return err
	}
	ret := RPMFile{
		Name: h.Name,
		Mode: uint(out.filemodes[i]),
		Owner: out.fileowners[i],
		Group: out.filegroups[i],
		MTime: uint32(h.ModTime.Unix()),
		Type: FileType(out.fileflags[i]),
	}

	buff := make([]byte, h.Size)
	count, err := r.Read(buff)
	if count != int(h.Size) {
		return fmt.Errorf("failed to read %s %w", h.Name, err)
	}
	ret.Body = buff

	if h.Linkname != "" {
		ret.Body = []byte(h.Linkname)
	}

	out.files[h.Name] = ret

	return nil
}

func readFiles(out *RPM, file *os.File) error {
	payload := bytes.NewBuffer(nil)
	count, err := payload.ReadFrom(file)

	if err != nil {
		return fmt.Errorf("failed to read payload: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("read 0 bytes as payload")
	}

	rc, err := setupDecompressor(out.Compressor, payload)

	if err != nil {
		return err
	}

	decompressPayload := bytes.NewBuffer(nil)
	count, err = decompressPayload.ReadFrom(rc)

	if err != nil {
		return fmt.Errorf("failed to decompress payload: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("read 0 bytes as decompressed payload")
	}

	out.files = map[string]RPMFile{}
	r := cpio.NewReader(decompressPayload)
	i := 0
	for {
		err = readFile(r, i, out)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		i++
	}
	out.dirindexes = make([]uint32, 0)
	out.basenames = make([]string, 0)
	out.fileowners = make([]string, 0)
	out.filegroups = make([]string, 0)
	out.filemtimes = make([]uint32, 0)
	out.fileflags = make([]uint32, 0)
	out.filesizes = make([]uint32, 0)
	out.filedigests = make([]string, 0)
	out.filelinktos = make([]string, 0)
	out.filemodes = make([]uint16, 0)
	return nil
}

func ReadRPMFile(p string) (*RPM, error) {
	file, err := os.Open(p)

	if err != nil {
		return nil, err
	}
	defer file.Close()

	lead, err := ReadLead(file)
	if err != nil {
		return nil, err
	}

	out := &RPM{}
	out.lead = lead

	err = readSignatures(file, out)

	if err != nil {
		return nil, err
	}

	err = seekFile(file)
	if err != nil {
		return nil, err
	}

	err = readHeaders(file, out)
	if err != nil {
		return nil, err
	}

	readGenIndexes(out)

	readFileIndexes(out)

	popTag(out.headers.entries, tagHeaderI18NTable, IndexEntry.toString)
	popTag(out.headers.entries, tagPayloadFormat, IndexEntry.toString)
	popTag(out.headers.entries, tagPayloadFlags, IndexEntry.toString)
	popTag(out.headers.entries, tagPayloadDigest, IndexEntry.toStringArray)
	popTag(out.headers.entries, tagPayloadDigestAlgo, IndexEntry.toUint32)

	err = readScripts(out)
	if err != nil {
		return nil, err
	}

	payloadSize, _ := popTag(out.headers.entries, tagSize, IndexEntry.toUint32)
	out.payloadSize = uint(payloadSize)

	out.customTags = out.headers.entries
	out.headers.h = 0

	err = readFiles(out, file)

	if err != nil {
		return nil, err
	}

	out.cpio = cpio.NewWriter(out.payload)

	buff := &bytes.Buffer{}

	z, _, err := setupCompressor(out.Compressor, buff)
	if err != nil {
		return nil, err
	}

	out.payload = buff
	out.compressedPayload = z
	out.cpio = cpio.NewWriter(z)

	return out, err
}
