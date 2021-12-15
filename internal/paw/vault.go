package paw

import (
	"io"
	"sort"
	"strings"
)

type Vault struct {
	key   *Key
	name  string
	Items map[string]Item
}

func NewVault(name string, key *Key) *Vault {
	return &Vault{
		name:  name,
		key:   key,
		Items: make(map[string]Item),
	}
}

func (v *Vault) SetName(name string) {
	v.name = name
}

func (v *Vault) SetKey(key *Key) {
	v.key = key
}

func (v *Vault) Key() *Key {
	return v.key
}

func (v *Vault) Name() string {
	return v.name
}

func (v *Vault) Encrypt(dst io.Writer) (io.WriteCloser, error) {
	return v.key.Encrypt(dst)
}

func (v *Vault) Item(id string) Item {
	return v.Items[id]
}

func (v *Vault) SetItem(item Item) {
	v.Items[item.ID()] = item
}

func (v *Vault) DeleteItem(item Item) {
	delete(v.Items, item.ID())
}

func (v *Vault) FilterItems(opts *VaultFilterOptions) []Item {
	items := []Item{}
	titleFilter := opts.Title
	itemType := opts.ItemType
	for _, item := range v.Items {
		if itemType != 0 && (itemType&item.Type()) == 0 {
			continue
		}

		if titleFilter != "" && !strings.HasPrefix(item.String(), titleFilter) {
			continue
		}

		items = append(items, item)
	}

	sort.Sort(ByString(items))

	return items
}

type VaultFilterOptions struct {
	Title    string
	ItemType ItemType
}
