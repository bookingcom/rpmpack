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
	"reflect"
	"sort"
	"time"
)

const (
	signatures = 0x3e
	immutable  = 0x3f

	typeInt16       = 0x03
	typeInt32       = 0x04
	typeString      = 0x06
	typeBinary      = 0x07
	typeStringArray = 0x08
	typei18nString  = 0x09
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

func (e IndexEntry) toString() (string, error) {
	if e.rpmtype != typeString && e.rpmtype != typei18nString {
		return "", fmt.Errorf("rpmtype %d is not a string type", e.rpmtype)
	}

	return string(e.data[:len(e.data)-1]), nil
}

var IndexEntryToString = IndexEntry.toString

func (e IndexEntry) toUint16() (uint16, error) {
	if e.rpmtype != typeInt16 {
		return 0, fmt.Errorf("rpmtype %d is not a uint16 type", e.rpmtype)
	}

	b := &bytes.Buffer{}
	b.Write(e.data)
	value := uint16(0)
	binary.Read(b, binary.BigEndian, &value)

	return value, nil
}

func (e IndexEntry) toUint32() (uint32, error) {
	if e.rpmtype != typeInt32 {
		return 0, fmt.Errorf("rpmtype %d is not a uint32 type", e.rpmtype)
	}

	b := &bytes.Buffer{}
	b.Write(e.data)
	value := uint32(0)
	binary.Read(b, binary.BigEndian, &value)

	return value, nil
}

func (e *index) toRelations(nameTag int, versionTag int, flagsTag int) (Relations, error) {
	names, err := popTag(e.entries, nameTag, IndexEntry.toStringArray)
	if err != nil {
		return nil, fmt.Errorf("failed to find name tag %d %w", nameTag, err)
	}

	versions, err := popTag(e.entries, versionTag, IndexEntry.toStringArray)
	if err != nil {
		return nil, fmt.Errorf("failed to find versions tag %d %w", versionTag, err)
	}

	flags, err := popTag(e.entries, flagsTag, IndexEntry.toInt32Array)
	if err != nil {
		return nil, fmt.Errorf("failed to find flags tag %d %w", flagsTag, err)
	}

	if (names == nil && versions == nil && flags == nil) {
		return nil, nil
	}

	if (names == nil || versions == nil || flags == nil) {
		return nil, fmt.Errorf("one of names versions or flags is nil %v %v %v", names, versions, flags)
	}

	if len(names) != len(versions) || len(names) != len(flags) {
		return nil, fmt.Errorf("missmatch in counts %d %d %d", len(names), len(versions), len(flags))
	}


	out := make(Relations, len(names))
	for i := range names {
		sense, err := SenseFromFlag(flags[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse sense at %d: %w", i, err)
		}
		out[i] = &Relation{
			Name: names[i],
			Version: versions[i],
			Sense: sense,
		}
	}

	return out, nil
}

func (e IndexEntry) toTime() (time.Time, error) {
	val, err := e.toUint32()
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(int64(val), 0), nil
}

func (e IndexEntry) toStringArray() ([]string, error) {
	if e.rpmtype != typeStringArray {
		return nil, fmt.Errorf("rpmtype %d is not a string array type", e.rpmtype)
	}

	data := e.data

	out := make([]string, e.count)
	for i := 0; i < e.count; i++ {
		end := bytes.IndexByte(data, '\x00')
		if  end > -1 {
			out[i] = string(data[:end])
			data = data[end+1:]
		} else {
			out[i] = string(data)
			data = nil
		}
	}

	return out, nil
}

func (e IndexEntry) toInt32Array() ([]int32, error) {
	if e.rpmtype != typeInt32 {
		return nil, fmt.Errorf("rpmtype %d is not an int type", e.rpmtype)
	}
	out := make([]int32, e.count)
	b := &bytes.Buffer{}
	b.Write(e.data)
	binary.Read(b, binary.BigEndian, &out)

	return out, nil
}

func (e IndexEntry) toUint32Array() ([]uint32, error) {
	if e.rpmtype != typeInt32 {
		return nil, fmt.Errorf("rpmtype %d is not an int type", e.rpmtype)
	}
	out := make([]uint32, e.count)
	b := &bytes.Buffer{}
	b.Write(e.data)
	binary.Read(b, binary.BigEndian, &out)

	return out, nil
}

func (e IndexEntry) toUint16Array() ([]uint16, error) {
	if e.rpmtype != typeInt16 {
		return nil, fmt.Errorf("rpmtype %d is not an int type", e.rpmtype)
	}
	out := make([]uint16, e.count)
	b := &bytes.Buffer{}
	b.Write(e.data)
	binary.Read(b, binary.BigEndian, &out)

	return out, nil
}

func (e IndexEntry) toInt16Array() ([]int16, error) {
	if e.rpmtype != typeInt16 {
		return nil, fmt.Errorf("rpmtype %d is not an int type", e.rpmtype)
	}
	out := make([]int16, e.count)
	b := &bytes.Buffer{}
	b.Write(e.data)
	binary.Read(b, binary.BigEndian, &out)

	return out, nil
}
func (e *IndexEntry) setData(data []byte) {
	e.data = data
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

func (i *index) Equals(o *index) bool {
	return i.h == o.h &&
		reflect.DeepEqual(i.entries, o.entries)
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
		return 1
	case typeBinary:
		return 1
	case typeStringArray:
		return 1
	case typei18nString:
		return 1
	}

	return -1
}

func readIndexEntry(entry IndexEntry, data []byte, offset int) ([]byte, error) {
	size := indexEntrySize(entry.rpmtype)
	if size < 1 {
		return nil, fmt.Errorf("can't handle %d data type yet", entry.rpmtype)
	}
	if len(data) < offset + (size * entry.count) {
		return nil, fmt.Errorf("buffer is too small size: %d, offset: %d, size: %d, count: %d", len(data), offset, size, entry.count)
	}
	if entry.rpmtype == typeInt16 || entry.rpmtype == typeInt32 {
		return data[offset:offset + ( size * entry.count )], nil
	}
	if entry.rpmtype == typeString || entry.rpmtype == typei18nString {
		data = 	data[offset:]
		end := bytes.IndexByte(data, '\x00')
		if  end > -1 {
			return data[:end+1], nil
		}
		return data, nil
	}
	if entry.rpmtype == typeStringArray {
		data = data[offset:]
		out := []byte{}
		offset = 0
		for i := 0 ; i < entry.count ; i++ {
			offset = bytes.IndexByte(data, '\x00')
			out = append(out, data[:offset+1]...)
			data = data[offset+1:]
		}
		return out, nil
	}
	if entry.rpmtype == typeBinary {
		data = data[offset:]
		return data[:entry.count], nil
	}
	return nil, fmt.Errorf("not implemented")

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
		entry := out.entries[tag]
		entry.setData(buf)
		out.entries[tag] = entry
	}

	return out, nil
}

type Lead struct {
    magic [4]byte
	major, minor uint8
	typeFile uint16
	archNum uint16
	name string
	nameFromBinary bool
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
	if r.nameFromBinary {
		fullVersion = ""
	}
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

func popTag[A any](m map[int]IndexEntry, key int, chain func(IndexEntry) (A, error)) (A, error) {
	v, ok := m[key]
	if !ok {
		var defaultValue A
		return defaultValue, fmt.Errorf("key %d not found", key)
	}

	delete(m, key)
	return chain(v)
}
