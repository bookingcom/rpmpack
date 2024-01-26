// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpmpack

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
)

const (
	signatures = 0x3e
	immutable  = 0x3f

	typeInt16       = 0x03
	typeInt32       = 0x04
	typeString      = 0x06
	typeBinary      = 0x07
	typeStringArray = 0x08
)

// Only integer types are aligned. This is not just an optimization - some versions
// of rpm fail when integers are not aligned. Other versions fail when non-integers are aligned.
var boundaries = map[int]int{
	typeInt16: 2,
	typeInt32: 4,
}

type IndexEntry struct {
	rpmtype, count int
	data           []byte
}

func (e IndexEntry) indexBytes(tag, contentOffset int) []byte {
	b := &bytes.Buffer{}
	if err := binary.Write(b, binary.BigEndian, []int32{int32(tag), int32(e.rpmtype), int32(contentOffset), int32(e.count)}); err != nil {
		// binary.Write can fail if the underlying Write fails, or the types are invalid.
		// bytes.Buffer's write never error out, it can only panic with OOM.
		panic(err)
	}
	return b.Bytes()
}

func readIndex(inp io.Reader) (int32, int32, *IndexEntry, error) {
	indexData := []int32{0, 0, 0, 0} // tag rpmtype contentOffset count
	err := binary.Read(inp, binary.BigEndian, indexData)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read index data: %w", err)
	}
	out := IndexEntry{
		rpmtype: int(indexData[1]),
		count: int(indexData[3]),
	}
	return indexData[0], indexData[2], &out, nil
}

func intEntry(rpmtype, size int, value interface{}) IndexEntry {
	b := &bytes.Buffer{}
	if err := binary.Write(b, binary.BigEndian, value); err != nil {
		// binary.Write can fail if the underlying Write fails, or the types are invalid.
		// bytes.Buffer's write never error out, it can only panic with OOM.
		panic(err)
	}
	return IndexEntry{rpmtype, size, b.Bytes()}
}

func EntryInt16(value []int16) IndexEntry {
	return intEntry(typeInt16, len(value), value)
}
func EntryUint16(value []uint16) IndexEntry {
	return intEntry(typeInt16, len(value), value)
}
func EntryInt32(value []int32) IndexEntry {
	return intEntry(typeInt32, len(value), value)
}
func EntryUint32(value []uint32) IndexEntry {
	return intEntry(typeInt32, len(value), value)
}
func EntryString(value string) IndexEntry {
	return IndexEntry{typeString, 1, append([]byte(value), byte(00))}
}
func EntryBytes(value []byte) IndexEntry {
	return IndexEntry{typeBinary, len(value), value}
}

func EntryStringSlice(value []string) IndexEntry {
	b := [][]byte{}
	for _, v := range value {
		b = append(b, []byte(v))
	}
	bb := append(bytes.Join(b, []byte{00}), byte(00))
	return IndexEntry{typeStringArray, len(value), bb}
}

type index struct {
	entries map[int]IndexEntry
	h       int
}

func newIndex(h int) *index {
	return &index{entries: make(map[int]IndexEntry), h: h}
}
func (i *index) Add(tag int, e IndexEntry) {
	i.entries[tag] = e
}
func (i *index) AddEntries(m map[int]IndexEntry) {
	for t, e := range m {
		i.Add(t, e)
	}
}

func (i *index) sortedTags() []int {
	t := []int{}
	for k := range i.entries {
		t = append(t, k)
	}
	sort.Ints(t)
	return t
}

func pad(w *bytes.Buffer, rpmtype, offset int) {
	// We need to align integer entries...
	if b, ok := boundaries[rpmtype]; ok && offset%b != 0 {
		if _, err := w.Write(make([]byte, b-offset%b)); err != nil {
			// binary.Write can fail if the underlying Write fails, or the types are invalid.
			// bytes.Buffer's write never error out, it can only panic with OOM.
			panic(err)
		}
	}
}

// Bytes returns the bytes of the index.
func (i *index) Bytes() ([]byte, error) {
	w := &bytes.Buffer{}
	// Even the header has three parts: The lead, the index entries, and the entries.
	// Because of alignment, we can only tell the actual size and offset after writing
	// the entries.
	entryData := &bytes.Buffer{}
	tags := i.sortedTags()
	offsets := make([]int, len(tags))
	for ii, tag := range tags {
		e := i.entries[tag]
		pad(entryData, e.rpmtype, entryData.Len())
		offsets[ii] = entryData.Len()
		entryData.Write(e.data)
	}
	entryData.Write(i.eigenHeader().data)

	// 4 magic and 4 reserved
	w.Write([]byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0})
	// 4 count and 4 size
	// We add the pseudo-entry "eigenHeader" to count.
	if err := binary.Write(w, binary.BigEndian, []int32{int32(len(i.entries)) + 1, int32(entryData.Len())}); err != nil {
		return nil, fmt.Errorf("failed to write eigenHeader: %w", err)
	}
	// Write the eigenHeader index entry
	w.Write(i.eigenHeader().indexBytes(i.h, entryData.Len()-0x10))
	// Write all of the other index entries
	for ii, tag := range tags {
		e := i.entries[tag]
		w.Write(e.indexBytes(tag, offsets[ii]))
	}
	w.Write(entryData.Bytes())
	return w.Bytes(), nil
}

// the eigenHeader is a weird entry. Its index entry is sorted first, but its content
// is last. The content is a 16 byte index entry, which is almost the same as the index
// entry except for the offset. The offset here is ... minus the length of the index entry region.
// Which is always 0x10 * number of entries.
// I kid you not.
func (i *index) eigenHeader() IndexEntry {
	b := &bytes.Buffer{}
	if err := binary.Write(b, binary.BigEndian, []int32{int32(i.h), int32(typeBinary), -int32(0x10 * (len(i.entries) + 1)), int32(0x10)}); err != nil {
		// binary.Write can fail if the underlying Write fails, or the types are invalid.
		// bytes.Buffer's write never error out, it can only panic with OOM.
		panic(err)
	}

	return EntryBytes(b.Bytes())
}

func indexEntrySize(rpmtype int) int {
	switch rpmtype {
	case typeInt16:
		return 2
	case typeInt32:
		return 4
	case typeString:
	case typeBinary:
	case typeStringArray:
		return 1
	}
	return -1
}

func readIndexEntry(entry IndexEntry, data []byte, offset int) ([]byte, error) {
	size := indexEntrySize(entry.rpmtype)
	if size < 1 {
		return nil, fmt.Errorf("can't handle %d data type yet", entry.rpmtype)
	}
	if len(data) < offset+size {
		return nil, fmt.Errorf("buffer is too small size: %d, offset: %d, size: %d", len(data), offset, size)
	}
	return data[offset:offset+size], nil
}

func readHeaderIndex(inp io.Reader, countEntries int, expectedHeaderType int, size int32) (*index, map[int]int, error) {
	out := index {
		entries: make(map[int]IndexEntry, countEntries),
	}

	offsets := make(map[int]int, countEntries)


	for i := 0; i < int(countEntries); i++ {
		tag, contentOffset, indexEntry, err := readIndex(inp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read index entry at %d: %w", i, err)
		}
		fmt.Fprintf(os.Stderr, "got tag %x offset %x type %x count %x\n", tag, contentOffset, indexEntry.rpmtype, indexEntry.count)
		if i == 0 {
			out.h = int(tag)
			if out.h != expectedHeaderType {
				return nil, nil, fmt.Errorf("missmatch type of header expected %x but got %x", expectedHeaderType, out.h)
			}
			if contentOffset + 0x10 != size {
				return nil, nil, fmt.Errorf("failed to read eigen header offset not matching: %x %x", contentOffset, size)
			}
			continue
		}
		out.entries[int(tag)] = *indexEntry
		offsets[int(tag)] = int(contentOffset)
	}

	return &out, offsets, nil
}

func ReadHeader(inp io.Reader, expectedHeaderType int) (*index, error) {
	data, err := readExactly(inp, 8)
	if err != nil {
		return nil, err
	}

	if [8]byte(data) != [8]byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0} {
		return nil, fmt.Errorf("header lead doesn't match expected: %v", data)
	}

	countEntries, err := readInt32(inp)
	if err != nil {
		return nil, fmt.Errorf("failed to read amount of entries: %w", err)
	}

	size, err := readInt32(inp)
	if err != nil {
		return nil, fmt.Errorf("failed to read length of entries: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Will be reading %d bytes\n", size)

	out, offsets, err := readHeaderIndex(inp, int(countEntries), expectedHeaderType, size)

	if err != nil {
		return nil, fmt.Errorf("failed to read header index: %w", err)
	}

	body, err := readExactly(inp, int64(size))
	if err != nil {
		return nil, fmt.Errorf("failed to read")
	}

	for tag := range(offsets) {
		buf, err := readIndexEntry(out.entries[tag], body, offsets[tag])
		if err != nil {
			return nil, fmt.Errorf("failed to extract data for %x: %w", tag, err)
		}
		copy(buf[:], out.entries[tag].data)
	}

	return out, nil
}

type Lead struct {
    magic [4]byte
	major, minor uint8
	typeFile uint16
	archNum uint16
	name string
	osnum, signatureType uint16
	reserved [16]uint8;
} ;

func NewLead(data RPMMetaData) *Lead {
	out := &Lead{}
	out.magic = [4]byte{'\xed', '\xab', '\xee', '\xdb'}
	out.major = 0x03
	out.minor = 0x00
	out.typeFile = 0 // assume binary
	out.archNum = 0  // i386? lead is ignored so it doesn't matter
	out.name = data.Name
	out.osnum = 0x01 // linux?
	out.signatureType = 0x05
	out.reserved = [16]uint8{}

	return out
}

func computeName(name string, version string) []byte {
	if name == "" {
		return make([]byte, 66)
	}

	if version != "" {
		name = fmt.Sprintf("%s-%s", name, version)
	}

	if len(name) > 65 {
		return []byte(name[:65])
	}

	var n []byte = []byte(name[:])
	n = append(n, make([]byte, 66-len(n))...)
	return n
}

func (r *Lead) toArray(fullVersion string) ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Write(r.magic[:])
	buf.Write([]byte{r.major, r.minor})
	binary.Write(buf, binary.BigEndian, r.typeFile)
	binary.Write(buf, binary.BigEndian, r.archNum)
	buf.Write(computeName(r.name, fullVersion))
	binary.Write(buf, binary.BigEndian, r.osnum)
	binary.Write(buf, binary.BigEndian, r.signatureType)
	buf.Write(r.reserved[:])
	out := buf.Bytes()
	if len(out) != 96 {
		return nil, fmt.Errorf("invalid lead length expected 96, got %d", len(out))
	}
	return out, nil
}

func (r *Lead) ToString() string {
	return fmt.Sprintf(`magic: %s
major: %d
minor: %d
file type: %d
arch: %d
name: %s
os number: %d
signature type: %d
`,
	r.magic, r.major, r.minor, r.typeFile, r.archNum, r.name, r.osnum, r.signatureType)
}

func readExactly(inp io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(inp, limit))
}

func readUint16(inp io.Reader) (uint16, error) {
	if val, err := readExactly(inp, 2); err != nil {
		return 0, fmt.Errorf("failed to read uint16: %v", err)
	} else {
		return binary.BigEndian.Uint16(val), nil
	}
}

func readInt32(inp io.Reader) (int32, error) {
	if val, err := readExactly(inp, 4); err != nil {
		return 0, fmt.Errorf("failed to read int32: %v", err)
	} else {
		return int32(binary.BigEndian.Uint32(val)), nil
	}
}

func readString(inp io.Reader, length int64) (string, error) {
	if val, err := readExactly(inp, length); err != nil {
		return "", fmt.Errorf("failed to read string: %s", err)
	} else {
		return string(bytes.Trim(val, "\x00")), nil
	}
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

func (r *Lead) Equals(o *Lead) bool {
	return r.magic == o.magic &&
		r.major == o.major &&
		r.minor == o.minor &&
		r.typeFile == o.typeFile &&
		r.archNum == o.archNum &&
		r.name == o.name &&
		r.osnum == o.osnum &&
		r.signatureType == o.signatureType
}
