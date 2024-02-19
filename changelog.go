package rpmpack

import (
	"fmt"
)

type ChangelogEntry struct {
	Time    uint32
	Name    string
	Text    string
}

func (c *ChangelogEntry) Equal(o *ChangelogEntry) bool {
	return c.Time == o.Time && c.Name == o.Name && c.Text == o.Text
}

type Changelog []*ChangelogEntry

func (r *RPM) GetChangelog() Changelog {
	return r.Changelog
}

func (c *Changelog) AddToIndex(h *index) error {
	num := len(*c)
	if num == 0 {
		return nil
	}

	items    := make([]ChangelogEntry, num)
	times    := make([]uint32, num)
	names    := make([]string, num)
	texts    := make([]string, num)

	for i := range(items) {
		items[i] = *(*c)[i]
	}

	for idx, entry := range items {
		times[idx] = uint32(entry.Time)
		names[idx] = entry.Name
		texts[idx] = entry.Text
	}

	h.Add(tagChangelogTime, EntryUint32(times))
	h.Add(tagChangelogName, EntryStringSlice(names))
	h.Add(tagChangelogText, EntryStringSlice(texts))

	return nil
}

func (i *index) toChangelog() (Changelog, error) {
	times, err := popTag(i.entries, tagChangelogTime, IndexEntry.toInt32Array)
	if err != nil {
		return nil, fmt.Errorf("failed to find tagChangelogTime: %w", err)
	}

	names, err := popTag(i.entries, tagChangelogName, IndexEntry.toStringArray)
	if err != nil {
		return nil, fmt.Errorf("failed to find tagChangelogName: %w", err)
	}

	texts, err := popTag(i.entries, tagChangelogText, IndexEntry.toStringArray)
	if err != nil {
		return nil, fmt.Errorf("failed to find tagChangelogText: %w", err)
	}

	if (names == nil && texts == nil && times == nil) {
		return nil, nil
	}

	if (names == nil || texts == nil || times == nil) {
		return nil, fmt.Errorf("one of names text or times is nil %v %v %v", names, texts, times)
	}

	if len(names) != len(texts) || len(names) != len(times) {
		return nil, fmt.Errorf("missmatch in counts %d %d %d", len(names), len(texts), len(times))
	}

	out := make(Changelog, len(times))

	for i := range times {
		out[i] = &ChangelogEntry{
			Time: uint32(times[i]),
			Name: names[i],
			Text: texts[i],
		}
	}

	return out, nil
}
