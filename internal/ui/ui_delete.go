package ui

import (
	"fmt"

	"cid_gio_gio/internal/core"

	"fyne.io/fyne/v2/dialog"
)

func (m *model) openDeleteDialog(d core.DeviceDTO) {
	if m.delBusy.Load() {
		return
	}
	msg := fmt.Sprintf("Delete object %03d?\nThis will permanently remove the object, journal and history records.", d.ID)
	confirm := dialog.NewConfirm("Delete Object", msg, func(ok bool) {
		if !ok {
			return
		}
		if m.delBusy.CompareAndSwap(false, true) {
			go m.deleteDeviceRemote(d.ID)
		}
	}, m.win)
	confirm.Show()
}
