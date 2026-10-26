package storage

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

type failingWriter struct {
	err error
}

// Write accepts bytes until the test sets an error.
func (w *failingWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}

type marshalFunc func() ([]byte, error)

// MarshalJSON lets a test fail writes after the age header has been written.
func (f marshalFunc) MarshalJSON() ([]byte, error) {
	return f()
}

// TestEncryptReturnsWriterFailure checks errors from the age writer reach callers.
func TestEncryptReturnsWriterFailure(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	wantErr := errors.New("disk full")

	err = encrypt(key, &failingWriter{err: wantErr}, map[string]string{"note": "value"})
	require.ErrorIs(t, err, wantErr)
}

// TestEncryptReturnsCloseFailure checks a failed final age chunk is reported.
func TestEncryptReturnsCloseFailure(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	wantErr := errors.New("disk full")
	w := &failingWriter{}
	value := marshalFunc(func() ([]byte, error) {
		w.err = wantErr
		// A small JSON value stays buffered until the age writer closes.
		return []byte(`"note"`), nil
	})

	err = encrypt(key, w, value)
	require.ErrorIs(t, err, wantErr)
	require.ErrorContains(t, err, "could not close encrypted writer")
}

// TestEncryptPreservesBothFailures checks encoding and close errors survive together.
func TestEncryptPreservesBothFailures(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	encodeErr := errors.New("encoding failed")
	closeErr := errors.New("disk full")
	w := &failingWriter{}
	value := marshalFunc(func() ([]byte, error) {
		w.err = closeErr
		return nil, encodeErr
	})

	err = encrypt(key, w, value)
	require.ErrorIs(t, err, encodeErr)
	require.ErrorIs(t, err, closeErr)
}

// rootedFyneStorage gives the Fyne test app an isolated data directory.
type rootedFyneStorage struct {
	fyne.Storage
	root fyne.URI
}

// RootURI returns the directory shared by the two backends in this test.
func (s rootedFyneStorage) RootURI() fyne.URI {
	return s.root
}

// TestStorageInteroperability checks OS and Fyne share the CXF storage format.
func TestStorageInteroperability(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name   string
		writer Storage
		reader Storage
	}{
		{name: "OS to Fyne", writer: osStorage, reader: fyneStorage},
		{name: "Fyne to OS", writer: fyneStorage, reader: osStorage},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vault, err := tt.writer.CreateVault(tt.name, "test password")
			require.NoError(t, err)
			id := vault.Accounts[0].ID
			note, err := model.NewNote("Note", "Synthetic private note")
			require.NoError(t, err)
			login, err := model.NewLogin("Login", "example user", "Synthetic login password", model.PasswordSettings{})
			require.NoError(t, err)
			items := []*model.Item{note, login}
			for _, item := range items {
				require.NoError(t, tt.writer.StoreItem(vault, item))
				data, err := os.ReadFile(itemPath(tt.writer, id, item.ID))
				require.NoError(t, err)
				require.True(t, bytes.HasPrefix(data, []byte("age-encryption.org/v1")))
				require.NotContains(t, string(data), "Synthetic")
			}

			loadedKey, err := tt.reader.LoadVaultKey(id, "test password")
			require.NoError(t, err)
			loadedVault, err := tt.reader.LoadVault(id, loadedKey)
			require.NoError(t, err)
			require.Len(t, loadedVault.Accounts[0].Items, 2)
			for _, item := range items {
				loadedItem, err := tt.reader.LoadItem(loadedVault, item.ID)
				require.NoError(t, err)
				require.Equal(t, item.Item, loadedItem.Item)
				require.Equal(t, item.Metadata, loadedItem.Metadata)
			}
			vaults, err := tt.reader.Vaults()
			require.NoError(t, err)
			require.Contains(t, vaults, VaultInfo{ID: id, Name: tt.name})

			state := &paw.AppState{Modified: time.Now().UTC(), Preferences: paw.NewDefaultPreferences()}
			state.Preferences.FaviconDownloader.Disabled = true
			state.Preferences.Password.Random.DefaultLength = 24
			require.NoError(t, tt.writer.StoreAppState(state))
			loadedState, err := tt.reader.LoadAppState()
			require.NoError(t, err)
			require.Equal(t, state, loadedState)
			require.Equal(t, tt.writer.SocketAgentPath(), tt.reader.SocketAgentPath())
			require.Equal(t, tt.writer.LockFilePath(), tt.reader.LockFilePath())
			require.Equal(t, tt.writer.LogFilePath(), tt.reader.LogFilePath())
		})
	}
}

// TestDeleteVaultRemovesCXFDirectory checks both backends delete vaults by ID.
func TestDeleteVaultRemovesCXFDirectory(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)
	vault, err := osStorage.CreateVault("Personal", "test password")
	require.NoError(t, err)
	id := vault.Accounts[0].ID
	require.NoError(t, fyneStorage.DeleteVault(id))
	require.NoDirExists(t, vaultRootPath(osStorage, id))
}

// TestCreateCXFVaultFiles checks OS and Fyne write the same encrypted vault files.
func TestCreateCXFVaultFiles(t *testing.T) {
	for _, name := range []string{"OS", "Fyne"} {
		t.Run(name, func(t *testing.T) {
			base, err := NewOSStorageRooted(t.TempDir())
			require.NoError(t, err)
			osStorage := base.(*OSStorage)
			backend := selectCXFTestBackend(t, osStorage, name)
			vault, err := backend.CreateVault("Personal "+name, "test password")
			require.NoError(t, err)
			id := vault.Accounts[0].ID
			directoryID := filepath.Base(vaultRootPath(osStorage, id))
			require.NotContains(t, directoryID, "-")
			require.Equal(t, strings.ToLower(directoryID), directoryID)
			require.True(t, isFilesystemVaultID(directoryID))
			require.FileExists(t, keyPath(osStorage, id))
			require.FileExists(t, vaultPath(osStorage, id))
			require.FileExists(t, vaultMetadataPath(osStorage, id))

			key, err := osStorage.LoadVaultKey(id, "test password")
			require.NoError(t, err)
			ciphertext, err := os.ReadFile(vaultPath(osStorage, id))
			require.NoError(t, err)
			decoded, err := decryptVault(key, bytes.NewReader(ciphertext))
			require.NoError(t, err)
			require.Equal(t, vault.Accounts[0].ID, decoded.Accounts[0].ID)
			require.Equal(t, vault.Metadata, decoded.Metadata)

			listed, err := osStorage.Vaults()
			require.NoError(t, err)
			require.Contains(t, listed, VaultInfo{ID: id, Name: "Personal " + name})
		})
	}
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

// TestIsLegacyVaultNameRejectsPathTraversal checks legacy names stay within storage.
func TestIsLegacyVaultNameRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../outside", "vault/child", "vault.bck"} {
		require.False(t, isLegacyVaultName(name), name)
	}
	require.True(t, isLegacyVaultName("Personal"))
}

// TestCreateCXFVaultDirectoryFailure checks both backends report real filesystem errors.
func TestCreateCXFVaultDirectoryFailure(t *testing.T) {
	for _, name := range []string{"OS", "Fyne"} {
		t.Run(name, func(t *testing.T) {
			storage, _ := newItemStoreVault(t)
			backend := selectCXFTestBackend(t, storage, name)
			root := storageRootPath(storage)
			require.NoError(t, os.Rename(root, root+".saved"))
			require.NoError(t, os.WriteFile(root, nil, 0600))
			vault, err := backend.CreateVault("Personal", "test password")
			require.ErrorContains(t, err, "could not create vault directory")
			require.Nil(t, vault)
		})
	}
}

// TestCreateCXFVaultRemovesFailedMetadata checks a partial catalog entry is removed.
func TestCreateCXFVaultRemovesFailedMetadata(t *testing.T) {
	for _, name := range []string{"OS", "Fyne"} {
		t.Run(name, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			backend := selectCXFTestBackend(t, storage, name)
			id := vault.Accounts[0].ID
			err := backend.publishVault(VaultInfo{ID: id, Name: ""})
			require.ErrorContains(t, err, "could not publish vault metadata")
			require.FileExists(t, vaultPath(storage, id))
			require.NoFileExists(t, vaultMetadataPath(storage, id))
			vaults, err := storage.Vaults()
			require.NoError(t, err)
			require.Empty(t, vaults)
		})
	}
}

// TestPublishVaultRejectsInvalidID checks cleanup cannot delete outside a vault directory.
func TestPublishVaultRejectsInvalidID(t *testing.T) {
	for _, name := range []string{"OS", "Fyne"} {
		t.Run(name, func(t *testing.T) {
			storage, _ := newItemStoreVault(t)
			backend := selectCXFTestBackend(t, storage, name)
			directory := filepath.Join(storage.Root(), "victim")
			require.NoError(t, os.Mkdir(directory, 0700))
			sentinel := filepath.Join(directory, vaultMetadataFileName)
			require.NoError(t, os.WriteFile(sentinel, []byte("Preserve this file"), 0600))
			err := backend.publishVault(VaultInfo{ID: "../victim", Name: "Invalid"})
			require.ErrorContains(t, err, "invalid CXF vault ID")
			data, err := os.ReadFile(sentinel)
			require.NoError(t, err)
			require.Equal(t, "Preserve this file", string(data))
		})
	}
}

// cxfTestBackend exercises the prepared backend methods before the public API cutover.
type cxfTestBackend interface {
	Storage
	publishVault(info VaultInfo) error
	storeVaultInfo(info VaultInfo) error
}

// selectCXFTestBackend uses the selected backend on the same isolated local directory.
func selectCXFTestBackend(t *testing.T, storage *OSStorage, name string) cxfTestBackend {
	t.Helper()
	if name == "OS" {
		return storage
	}
	app := test.NewApp()
	t.Cleanup(app.Quit)
	backend, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(storage.Root()),
	})
	require.NoError(t, err)
	return backend.(*FyneStorage)
}

// TestVaultsExcludesBackups checks migration backups cannot be selected as vaults.
func TestVaultsExcludesBackups(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)
	for _, name := range []string{"Personal", "Work", "Personal.261023_120000.bck", "Work.261023_130000.bck"} {
		require.NoError(t, os.Mkdir(filepath.Join(storageRootPath(osStorage), name), 0700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(storageRootPath(osStorage), "unrelated.txt"), nil, 0600))
	for _, backend := range []Storage{osStorage, fyneStorage} {
		vaults, err := backend.Vaults()
		require.NoError(t, err)
		require.ElementsMatch(t, []VaultInfo{
			{ID: "Personal", Name: "Personal", NeedsMigration: true},
			{ID: "Work", Name: "Work", NeedsMigration: true},
		}, vaults)
	}
}

// TestStoreVaultInfoListsCXFVaults checks both backends publish ID and name metadata.
func TestStoreVaultInfoListsCXFVaults(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)

	info := VaultInfo{ID: base64.RawURLEncoding.EncodeToString(make([]byte, 16)), Name: "Personal"}
	for _, backend := range []cxfTestBackend{osStorage.(*OSStorage), fyneStorage.(*FyneStorage)} {
		require.NoError(t, backend.storeVaultInfo(info))
		data, err := os.ReadFile(vaultMetadataPath(backend, info.ID))
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"Personal"}`, string(data))
		vaults, err := backend.Vaults()
		require.NoError(t, err)
		require.Contains(t, vaults, info)
	}
}

// TestVaultsHidesUnpublishedIDDirectories checks missing metadata keeps new folders hidden.
func TestVaultsHidesUnpublishedIDDirectories(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)
	id := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	require.NoError(t, os.Mkdir(vaultRootPath(osStorage, id), 0700))
	for _, backend := range []Storage{osStorage, fyneStorage} {
		vaults, err := backend.Vaults()
		require.NoError(t, err)
		require.Empty(t, vaults)
	}
}

// TestVaultsRejectsMalformedMetadata checks corrupt published metadata is reported.
func TestVaultsRejectsMalformedMetadata(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)
	id := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	dir := vaultRootPath(osStorage, id)
	require.NoError(t, os.Mkdir(dir, 0700))
	require.NoError(t, os.WriteFile(vaultMetadataPath(osStorage, id), []byte(`{"name":`), 0600))
	for _, backend := range []Storage{osStorage, fyneStorage} {
		_, err = backend.Vaults()
		require.ErrorContains(t, err, "invalid metadata")
	}
}

// TestDefaultAppState checks preferences are available before the first settings save.
func TestDefaultAppState(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osStorage, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)
	for _, backend := range []Storage{osStorage, fyneStorage} {
		// Fyne also reports the missing settings file; both return default preferences.
		state, _ := backend.LoadAppState()
		require.NotNil(t, state)
		require.NotNil(t, state.Preferences)
		require.False(t, state.Modified.IsZero())
		require.False(t, state.Preferences.FaviconDownloader.Disabled)
		require.Equal(t, paw.RandomPasswordDefaultLength, state.Preferences.Password.Random.DefaultLength)
		require.Equal(t, paw.RandomPasswordDefaultFormat, state.Preferences.Password.Random.DefaultFormat)
		require.Equal(t, paw.PassphrasePasswordDefaultLength, state.Preferences.Password.Passphrase.DefaultLength)
		require.Equal(t, paw.PinPasswordDefaultLength, state.Preferences.Password.Pin.DefaultLength)
		require.Equal(t, paw.TOTPDigitsDefault, state.Preferences.TOTP.Digits)
		require.Equal(t, paw.TOTPHashDefault, state.Preferences.TOTP.Hash)
		require.Equal(t, paw.TOTPIntervalDefault, state.Preferences.TOTP.Interval)
	}
}
