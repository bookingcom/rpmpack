package rpmpack

import (
	"cmp"
	"slices"
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

	slices.SortFunc(items, func(a, b ChangelogEntry) int {
		if a.Time != b.Time {
			if a.Name != b.Name {
				return cmp.Compare(a.Text, b.Text)
			}
			return cmp.Compare(a.Name, b.Name)
		}
		return cmp.Compare(b.Time, a.Time)
	})

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
