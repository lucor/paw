package browser

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteNativeManifests(t *testing.T) {

	dir, err := os.MkdirTemp("", "paw")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	switch runtime.GOOS {
	case "linux", "darwin":
		os.Setenv("HOME", dir)
	case "windows":
		t.Skipf("unsupported OS: %s", runtime.GOOS)
		os.Setenv("USERPROFILE", dir)
	default:
		t.Skipf("unsupported OS: %s", runtime.GOOS)
	}

	t.Run("chrome", func(t *testing.T) {
		err := WriteNativeManifests()
		require.NoError(t, err)
		locations, err := chromeNativeManifestLocations()
		require.NoError(t, err)
		for _, location := range locations {
			assert.FileExists(t, location)
			assertManifestHasFields(t, location)
		}
	})

	t.Run("firefox", func(t *testing.T) {
		err := WriteNativeManifests()
		require.NoError(t, err)
		locations, err := chromeNativeManifestLocations()
		require.NoError(t, err)
		for _, location := range locations {
			assert.FileExists(t, location)
			assertManifestHasFields(t, location)
		}
	})
}

func assertManifestHasFields(t *testing.T, location string) {
	b, err := os.ReadFile(location)
	require.NoError(t, err)
	data := map[string]any{}
	err = json.Unmarshal(b, &data)
	require.NoError(t, err)
	assert.Equal(t, data["name"], "paw")
	assert.Equal(t, data["type"], "stdio")
	assert.NotEmpty(t, data["path"])
}
