package storage

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func newTestFileStorage(t *testing.T) *FileStorage {
	t.Helper()
	store, err := NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

// TestEncryptRejectsUnsupportedJSONValue checks encoding rejects values JSON cannot represent.
func TestEncryptRejectsUnsupportedJSONValue(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	err = encrypt(key, &bytes.Buffer{}, make(chan int))
	require.ErrorContains(t, err, "unsupported type")
}

// TestFilesystemIDRoundTrip checks filesystem names preserve canonical IDs.
func TestFilesystemIDRoundTrip(t *testing.T) {
	bytes := []byte{0xfb, 0xff, 0x00, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	id := base64.RawURLEncoding.EncodeToString(bytes)
	require.True(t, strings.HasPrefix(id, "-_"))

	name := filesystemID(id)
	require.Len(t, name, 26)
	require.Equal(t, strings.ToLower(name), name)
	require.NotContains(t, name, "-")
	require.NotContains(t, name, "_")
	decodedID, ok := idFromFilesystemName(name)
	require.True(t, ok)
	require.Equal(t, id, decodedID)
	_, ok = idFromFilesystemName(strings.ToUpper(name))
	require.False(t, ok)
}

// TestDeleteVaultRejectsPathTraversal checks deletion cannot escape the vault root.
func TestDeleteVaultRejectsPathTraversal(t *testing.T) {
	store, _ := newItemStoreVault(t)
	directory := filepath.Join(store.Root(), "victim")
	require.NoError(t, os.Mkdir(directory, 0700))
	sentinel := filepath.Join(directory, vaultMetadataFileName)
	require.NoError(t, os.WriteFile(sentinel, []byte("Preserve this file"), 0600))
	err := store.DeleteVault("../victim")
	require.ErrorContains(t, err, "invalid vault ID")
	data, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	require.Equal(t, "Preserve this file", string(data))
}

func TestDeleteVaultRemovesDirectory(t *testing.T) {
	store, vault := newItemStoreVault(t)
	id := vault.Accounts[0].ID
	require.NoError(t, store.DeleteVault(id))
	require.NoDirExists(t, vaultRootPath(store, id))
}

// TestVaultsExcludesBackups checks invalid and backup directories are not listed.
func TestVaultsExcludesBackups(t *testing.T) {
	fileStore := newTestFileStorage(t)
	for _, name := range []string{"Personal", "Work", "Personal.261023_120000.bck", "Work.261023_130000.bck"} {
		require.NoError(t, os.Mkdir(filepath.Join(storageRootPath(fileStore), name), 0700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(storageRootPath(fileStore), "unrelated.txt"), nil, 0600))
	vaults, err := fileStore.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
	_, err = fileStore.LoadVaultKey("Personal", "password")
	require.ErrorContains(t, err, "invalid vault ID")
}

func TestVaultsAreSortedByDirectoryName(t *testing.T) {
	store := newTestFileStorage(t)
	for i, name := range []string{"Charlie", "Alpha", "Bravo"} {
		idData := make([]byte, 16)
		idData[15] = byte(i + 1)
		id := base64.RawURLEncoding.EncodeToString(idData)
		require.NoError(t, store.storeVaultInfo(model.VaultInfo{ID: id, Name: name}))
	}
	vaults, err := store.Vaults()
	require.NoError(t, err)
	for i := 1; i < len(vaults); i++ {
		previous := filesystemID(vaults[i-1].ID)
		current := filesystemID(vaults[i].ID)
		require.LessOrEqual(t, previous, current)
	}
}

func TestVaultMetadataAppearanceDefaultsAndValidation(t *testing.T) {
	id := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	for _, testCase := range []struct {
		name string
		data string
		want model.VaultAppearance
	}{
		{name: "missing appearance", data: `{"name":"Personal"}`, want: model.DefaultVaultAppearance()},
		{name: "unsupported identifiers", data: `{"name":"Personal","icon":"upload","color":"pink"}`, want: model.DefaultVaultAppearance()},
		{name: "supported identifiers", data: `{"name":"Personal","icon":"folder","color":"purple"}`, want: model.VaultAppearance{Icon: "folder", Color: "purple"}},
		{name: "no color", data: `{"name":"Personal","icon":"paw","color":"none"}`, want: model.VaultAppearance{Icon: "paw", Color: "none"}},
		{name: "preserve configured vault icon", data: `{"name":"Personal","icon":"vault","color":"red"}`, want: model.VaultAppearance{Icon: "vault", Color: "red"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			info, err := vaultInfoFromMetadata(id, []byte(testCase.data))
			require.NoError(t, err)
			require.Equal(t, testCase.want, info.Appearance)
		})
	}
}

func TestRenameVaultPreservesAppearance(t *testing.T) {
	backend, vault := newItemStoreVault(t)
	want := model.VaultAppearance{Icon: "laptop", Color: "purple"}
	require.NoError(t, backend.storeVaultInfo(model.VaultInfo{ID: vault.Accounts[0].ID, Name: "Work", Appearance: want}))
	keyString := vault.Key().String()
	item, err := model.NewNote("Test", "note")
	require.NoError(t, err)
	require.NoError(t, backend.StoreItem(vault, item))
	updatedAppearance := model.VaultAppearance{Icon: "house", Color: "orange"}
	require.NoError(t, backend.UpdateVaultIdentity(vault, "Renamed work", updatedAppearance))
	vaults, err := backend.Vaults()
	require.NoError(t, err)
	info, err := model.FindVaultInfo(vaults, vault.Accounts[0].ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed work", info.Name)
	require.Equal(t, updatedAppearance, info.Appearance)
	require.Equal(t, "Renamed work", vault.Metadata.VaultName)
	reloaded, err := backend.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	require.Equal(t, "Renamed work", reloaded.Metadata.VaultName)
	require.Equal(t, keyString, reloaded.Key().String())
	loaded, err := backend.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, item.ID, loaded.ID)
	require.Equal(t, item.Credentials, loaded.Credentials)
	require.NoError(t, backend.RenameVault(vault, "Renamed again"))
	vaults, err = backend.Vaults()
	require.NoError(t, err)
	info, err = model.FindVaultInfo(vaults, vault.Accounts[0].ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed again", info.Name)
	require.Equal(t, updatedAppearance, info.Appearance)
}

// TestVaultsHidesUnpublishedIDDirectories checks missing metadata keeps new folders hidden.
func TestVaultsHidesUnpublishedIDDirectories(t *testing.T) {
	fileStore := newTestFileStorage(t)
	id := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	require.NoError(t, os.Mkdir(vaultRootPath(fileStore, id), 0700))
	vaults, err := fileStore.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
}

// TestVaultsRejectsMalformedMetadata checks corrupt published metadata is reported.
func TestVaultsRejectsMalformedMetadata(t *testing.T) {
	fileStore := newTestFileStorage(t)
	id := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	dir := vaultRootPath(fileStore, id)
	require.NoError(t, os.Mkdir(dir, 0700))
	require.NoError(t, os.WriteFile(vaultMetadataPath(fileStore, id), []byte(`{"name":`), 0600))
	_, err := fileStore.Vaults()
	require.ErrorContains(t, err, "invalid metadata")
}

// TestDefaultAppState checks settings are available before the first settings save.
func TestDefaultAppState(t *testing.T) {
	fileStore := newTestFileStorage(t)
	state, err := fileStore.LoadAppState()
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Settings)
	require.False(t, state.Modified.IsZero())
	require.False(t, state.Settings.FaviconDownloader.Disabled)
	require.Equal(t, paw.RandomPasswordDefaultLength, state.Settings.Password.Random.DefaultLength)
	require.Equal(t, paw.RandomPasswordDefaultFormat, state.Settings.Password.Random.DefaultFormat)
	require.Equal(t, paw.PassphrasePasswordDefaultLength, state.Settings.Password.Passphrase.DefaultLength)
	require.Equal(t, paw.PinPasswordDefaultLength, state.Settings.Password.Pin.DefaultLength)
}
