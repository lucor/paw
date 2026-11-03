// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginURL_Set(t *testing.T) {

	t.Run("Valid URL with scheme", func(t *testing.T) {
		u := NewLoginURL()
		err := u.Set("https://example.com")
		assert.NoError(t, err)
		assert.Equal(t, "https://example.com", u.URL().String())
		assert.Equal(t, "example.com", u.tldPlusOne)
	})

	t.Run("Valid URL without scheme", func(t *testing.T) {
		u := NewLoginURL()
		err := u.Set("example.com")
		assert.NoError(t, err)
		assert.Equal(t, "https://example.com", u.URL().String())
		assert.Equal(t, "example.com", u.tldPlusOne)
	})

	t.Run("Invalid URL", func(t *testing.T) {
		u := NewLoginURL()
		err := u.Set(":")
		assert.Error(t, err)
		assert.Empty(t, u.URL())
		assert.Empty(t, u.tldPlusOne)
	})

	t.Run("Empty URL", func(t *testing.T) {
		u := NewLoginURL()
		err := u.Set("")
		assert.NoError(t, err)
		assert.Empty(t, u.URL())
		assert.Empty(t, u.tldPlusOne)
	})

	t.Run("Update with invalid a valid URL", func(t *testing.T) {
		u := NewLoginURL()
		err := u.Set("example.com")
		assert.NoError(t, err)
		err = u.Set(":")
		assert.Error(t, err)
		assert.Equal(t, "https://example.com", u.URL().String())
		assert.Equal(t, "example.com", u.tldPlusOne)
	})
}
func TestLoginURL_MarshalJSON(t *testing.T) {
	t.Run("URL is nil", func(t *testing.T) {
		u := NewLoginURL()
		data, err := u.MarshalJSON()
		assert.NoError(t, err)
		assert.Equal(t, []byte(`""`), data)
	})

	t.Run("URL is not nil", func(t *testing.T) {
		u := NewLoginURL()
		err := u.Set("https://example.com")
		assert.NoError(t, err)
		data, err := u.MarshalJSON()
		assert.NoError(t, err)
		assert.Equal(t, []byte(`"https://example.com"`), data)
	})
}

func TestLoginURL_UnmarshalJSON(t *testing.T) {
	t.Run("Empty JSON data", func(t *testing.T) {
		u := NewLoginURL()
		err := u.UnmarshalJSON([]byte(`""`))
		assert.NoError(t, err)
		assert.Empty(t, u.URL())
		assert.Empty(t, u.tldPlusOne)
	})

	t.Run("Valid JSON data", func(t *testing.T) {
		u := NewLoginURL()
		err := u.UnmarshalJSON([]byte(`"https://example.com"`))
		assert.NoError(t, err)
		expectedURL, err := url.Parse("https://example.com")
		assert.NoError(t, err)
		assert.Equal(t, expectedURL, u.URL())
		assert.Equal(t, "example.com", u.tldPlusOne)
	})

	t.Run("Invalid JSON data", func(t *testing.T) {
		u := NewLoginURL()
		err := u.UnmarshalJSON([]byte(`123`))
		assert.Error(t, err)
		assert.Empty(t, u.URL())
		assert.Empty(t, u.tldPlusOne)
	})
}
