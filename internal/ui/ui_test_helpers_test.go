// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func findPilotIconButton(t *testing.T, view fyne.CanvasObject, label string) *iconButton {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == label {
			return button
		}
	}
	t.Fatalf("icon button %q is not rendered", label)
	return nil
}

func findPilotNavigation(t *testing.T, view fyne.CanvasObject) *Navigation {
	t.Helper()
	var find func(fyne.CanvasObject) *Navigation
	find = func(object fyne.CanvasObject) *Navigation {
		if navigation, ok := object.(*Navigation); ok {
			return navigation
		}
		container, ok := object.(*fyne.Container)
		if !ok {
			return nil
		}
		for _, child := range container.Objects {
			if navigation := find(child); navigation != nil {
				return navigation
			}
		}
		return nil
	}
	if navigation := find(view); navigation != nil {
		return navigation
	}
	t.Fatal("bottom navigation is not rendered")
	return nil
}
