// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

func (a *app) makeShowItemView(fyneItemWidget FyneItemWidget) fyne.CanvasObject {
	itemViewWidget := newItemViewWidget(context.TODO(), fyneItemWidget, a.win)
	itemViewWidget.OnSubmit = func() {
		a.showEditItemView(fyneItemWidget)
	}
	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, itemViewWidget)
}
