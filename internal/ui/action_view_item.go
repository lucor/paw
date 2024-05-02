// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
