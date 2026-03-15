//go:build windows

package walk

import (
	"time"

	"cid_fyne/internal/core"
	"github.com/lxn/walk"
)

func (a *walkApp) flushLoop() {
	ticker := time.NewTicker(uiRefreshTick)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.mw == nil || (len(a.pendingDevices) == 0 && len(a.pendingEvents) == 0 && len(a.pendingDeleted) == 0) {
				continue
			}
			a.flushPending()
		}
	}
}

func (a *walkApp) flushPending() {
	upd := map[int]core.DeviceDTO{}
	deleteIDs := map[int]struct{}{}
	eventBatch := make([]core.EventDTO, 0, uiDrainBatchSize)

	for pass := 0; pass < 4; pass++ {
		for i := 0; i < uiDrainBatchSize; i++ {
			select {
			case id := <-a.pendingDeleted:
				deleteIDs[id] = struct{}{}
			default:
				i = uiDrainBatchSize
			}
		}
		for i := 0; i < uiDrainBatchSize; i++ {
			select {
			case d := <-a.pendingDevices:
				upd[d.ID] = d
			default:
				i = uiDrainBatchSize
			}
		}
		for i := 0; i < uiDrainBatchSize; i++ {
			select {
			case e := <-a.pendingEvents:
				eventBatch = append(eventBatch, e)
			default:
				i = uiDrainBatchSize
			}
		}
		if len(a.pendingDevices) == 0 && len(a.pendingEvents) == 0 && len(a.pendingDeleted) == 0 {
			break
		}
	}

	if len(upd) == 0 && len(eventBatch) == 0 && len(deleteIDs) == 0 {
		return
	}

	updateDevices := len(upd) > 0 || len(deleteIDs) > 0
	updateEvents := len(eventBatch) > 0
	a.mu.Lock()
	for id := range deleteIDs {
		delete(a.devices, id)
	}
	for _, d := range upd {
		a.devices[d.ID] = d
	}
	if len(eventBatch) > 0 {
		capN := maxInt(1000, a.eventsLimit)
		a.events = prependEvents(a.events, eventBatch, capN)
		a.eventsAllShown.Store(false)
	}
	a.applyFiltersLocked()
	a.mu.Unlock()
	a.syncUIStateAsync(updateDevices, updateEvents)
}

func (a *walkApp) eventsScrollLoop() {
	ticker := time.NewTicker(uiScrollTick)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.mw == nil || a.eventTable == nil || a.eventsLoading.Load() || a.eventsAllShown.Load() {
				continue
			}
			a.mw.Synchronize(func() {
				if a.eventTable == nil || a.eventsLoading.Load() || a.eventsAllShown.Load() {
					return
				}
				if a.shouldAutoLoadByScroll(a.eventTable, len(a.filteredEvents)) {
					a.loadMoreEvents()
				}
			})
		}
	}
}

func (a *walkApp) shouldAutoLoadByScroll(tv *walk.TableView, rows int) bool {
	if tv == nil || rows < 1 {
		return false
	}
	if rows <= tv.RowsPerPage()+4 {
		return true
	}
	trigger := rows - maxInt(8, tv.RowsPerPage()/2)
	if trigger < 0 {
		trigger = 0
	}
	return tv.ItemVisible(trigger)
}

func (a *walkApp) statsLoop() {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.mw == nil {
				continue
			}
			stats := a.rt.GetStats()
			a.mw.Synchronize(func() {
				a.stats = stats
				a.updateStatusBar()
			})
		}
	}
}
