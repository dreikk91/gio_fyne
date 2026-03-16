//go:build windows

package walk

import (
	"fmt"
	"time"

	"cid_fyne/internal/core"

	"github.com/lxn/walk"
)

type deviceTableModel struct {
	walk.TableModelBase
	app  *walkApp
	rows []core.DeviceDTO
	tv   *walk.TableView
}

func (m *deviceTableModel) RowCount() int {
	return len(m.rows)
}

func (m *deviceTableModel) Value(row, col int) any {
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	d := m.rows[row]
	switch col {
	case 0:
		if m.app.isDeviceInactive(d) {
			return "Неактивний"
		}
		return "Активний"
	case 1:
		return fmt.Sprintf("%03d", d.ID)
	case 2:
		return firstNonEmpty(d.ClientAddr, "-")
	case 3:
		return firstNonEmpty(d.LastEvent, "-")
	case 4:
		if d.LastEventTime.IsZero() {
			return "-"
		}
		return d.LastEventTime.Format("2006-01-02 15:04:05")
	default:
		return ""
	}
}

func (m *deviceTableModel) SetRows(rows []core.DeviceDTO) {
	old := append([]core.DeviceDTO(nil), m.rows...)
	m.rows = append(m.rows[:0], rows...)
	m.publishDeviceDiff(old, m.rows)
}

func (m *deviceTableModel) SetTableView(tv *walk.TableView) {
	m.tv = tv
}

func (m *deviceTableModel) ApplyRows(rows []core.DeviceDTO) {
	if m.tv == nil {
		m.SetRows(rows)
		return
	}
	m.tv.Synchronize(func() {
		m.SetRows(rows)
	})
}

func (m *deviceTableModel) Row(row int) (core.DeviceDTO, bool) {
	if row < 0 || row >= len(m.rows) {
		return core.DeviceDTO{}, false
	}
	return m.rows[row], true
}

type eventTableModel struct {
	walk.TableModelBase
	rows []core.EventDTO
	tv   *walk.TableView
}

func (m *eventTableModel) RowCount() int {
	return len(m.rows)
}

func (m *eventTableModel) Value(row, col int) any {
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	e := m.rows[row]
	switch col {
	case 0:
		if e.Time.IsZero() {
			return "-"
		}
		return e.Time.Format("2006-01-02 15:04:05")
	case 1:
		return e.DeviceID
	case 2:
		return e.Code
	case 3:
		return e.Type
	case 4:
		return e.Desc
	case 5:
		return firstNonEmpty(e.Zone, "-")
	case 6:
		return e.Category
	default:
		return ""
	}
}

func (m *eventTableModel) SetRows(rows []core.EventDTO) {
	old := append([]core.EventDTO(nil), m.rows...)
	m.rows = append(m.rows[:0], rows...)
	m.publishEventDiff(old, m.rows)
}

func (m *eventTableModel) SetTableView(tv *walk.TableView) {
	m.tv = tv
}

func (m *eventTableModel) ApplyRows(rows []core.EventDTO) {
	if m.tv == nil {
		m.SetRows(rows)
		return
	}
	m.tv.Synchronize(func() {
		m.SetRows(rows)
	})
}

func (m *eventTableModel) Row(row int) (core.EventDTO, bool) {
	if row < 0 || row >= len(m.rows) {
		return core.EventDTO{}, false
	}
	return m.rows[row], true
}


func (m *deviceTableModel) publishDeviceDiff(oldRows, newRows []core.DeviceDTO) {
	oldLen := len(oldRows)
	newLen := len(newRows)
	if oldLen == 0 || newLen == 0 || oldLen != newLen {
		m.PublishRowsReset()
		return
	}
	
	// Check for reordering or major changes
	diffCount := 0
	start := -1
	end := -1
	for i := 0; i < newLen; i++ {
		if !sameDeviceRow(oldRows[i], newRows[i]) {
			diffCount++
			if start == -1 {
				start = i
			}
			end = i
		}
	}
	
	if diffCount > newLen/2 || diffCount > 10 { // Significant change or reorder
		m.PublishRowsReset()
		return
	}
	
	if start == -1 {
		return
	}
	m.PublishRowsChanged(start, end)
}

func (m *eventTableModel) publishEventDiff(oldRows, newRows []core.EventDTO) {
	oldLen := len(oldRows)
	newLen := len(newRows)
	if oldLen == 0 || newLen == 0 {
		m.PublishRowsReset()
		return
	}
	if newLen > oldLen {
		diff := newLen - oldLen
		if eventSlicesEqual(newRows[diff:], oldRows) {
			m.PublishRowsInserted(0, diff-1)
			return
		}
		m.PublishRowsReset()
		return
	}
	if newLen < oldLen {
		m.PublishRowsReset()
		return
	}
	start := -1
	end := -1
	for i := 0; i < newLen; i++ {
		if sameEventRow(oldRows[i], newRows[i]) {
			continue
		}
		if start == -1 {
			start = i
		}
		end = i
	}
	if start == -1 {
		return
	}
	m.PublishRowsChanged(start, end)
}

func sameDeviceRow(a, b core.DeviceDTO) bool {
	return a.ID == b.ID &&
		a.Name == b.Name &&
		a.ClientAddr == b.ClientAddr &&
		a.LastEvent == b.LastEvent &&
		sameTime(a.LastEventTime, b.LastEventTime)
}

func sameEventRow(a, b core.EventDTO) bool {
	return sameTime(a.Time, b.Time) &&
		a.DeviceID == b.DeviceID &&
		a.Code == b.Code &&
		a.Type == b.Type &&
		a.Desc == b.Desc &&
		a.Zone == b.Zone &&
		a.Priority == b.Priority &&
		a.Category == b.Category
}

func eventSlicesEqual(a, b []core.EventDTO) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameEventRow(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameTime(a, b time.Time) bool {
	return a.Equal(b)
}
