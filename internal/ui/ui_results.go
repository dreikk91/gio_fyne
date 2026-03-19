package ui

import (
	"image/color"
	"strings"

	"github.com/dreikk91/gio_fyne/internal/core"

	"github.com/rs/zerolog/log"
)

func (m *model) pullBootstrapResult() {
	for {
		select {
		case r := <-m.bootCh:
			if r.reqID != m.bootReqID.Load() {
				continue
			}
			if r.err != nil {
				m.statusErr = "Bootstrap failed: " + r.err.Error()
				log.Error().Err(r.err).Msg("bootstrap reload failed")
				m.refreshMainUI()
				continue
			}
			m.mu.Lock()
			m.devices = map[int]core.DeviceDTO{}
			for _, d := range r.boot.Devices {
				m.devices[d.ID] = d
			}
			m.events = append([]core.EventDTO{}, r.boot.Events...)
			m.applyFiltersLocked()
			m.mu.Unlock()
			m.statusErr = ""
			m.statusMsg = "Bootstrap reloaded"
			m.refreshMainUI()
		default:
			return
		}
	}
}

func (m *model) pullEventsResult() {
	for {
		select {
		case r := <-m.eventsCh:
			if r.reqID != m.eventsReqID.Load() {
				continue
			}
			m.eventsBusy.Store(false)
			if r.err != nil {
				m.statusErr = "Events reload failed: " + r.err.Error()
				log.Error().Err(r.err).Msg("events reload failed")
				m.refreshMainUI()
				continue
			}
			m.mu.Lock()
			m.events = append([]core.EventDTO{}, r.events...)
			m.eventsLimit = r.limit
			m.applyFiltersLocked()
			m.mu.Unlock()
			m.statusErr = ""
			m.statusMsg = "Events reloaded"
			m.refreshMainUI()
		default:
			return
		}
	}
}

func (m *model) pullSaveResult() {
	for {
		select {
		case r := <-m.saveCh:
			m.runOnUI(func() {
				if r.err != nil {
					m.statusErr = "Save failed: " + r.err.Error()
					log.Error().Err(r.err).Msg("save config failed")
				} else {
					m.statusErr = ""
					m.statusMsg = "Settings saved"
					m.cfg = r.cfg
					m.loadCfgEditors(m.cfg)
				}
				m.refreshMainUI()
			})
		default:
			return
		}
	}
}

func (m *model) pullStatsResult() {
	for {
		select {
		case s := <-m.statsCh:
			m.mu.Lock()
			m.stats = s
			m.mu.Unlock()
			m.refreshMainUI()
		default:
			return
		}
	}
}

func (m *model) pullHistoryResult() {
	for {
		select {
		case r := <-m.hResult:
			if r.reqID != m.historyReqID.Load() {
				continue
			}
			m.historyBusy.Store(false)
			if r.err != nil {
				m.statusErr = "History reload failed: " + r.err.Error()
				log.Error().Err(r.err).Msg("history reload failed")
				m.refreshHistoryUI()
				continue
			}
			log.Debug().Int("device_id", r.id).Int("rows", len(r.events)).Msg("history loaded")
			m.mu.Lock()
			m.hRows = append([]core.EventDTO{}, r.events...)
			m.hLimit = r.limit
			m.resetHistWindowLocked()
			m.mu.Unlock()
			m.statusErr = ""
			m.refreshHistoryUI()
		default:
			return
		}
	}
}

func (m *model) pullDeleteResult() {
	for {
		select {
		case r := <-m.deleteCh:
			m.delBusy.Store(false)
			if r.err != nil {
				m.statusErr = "Delete failed: " + r.err.Error()
				log.Error().Err(r.err).Int("device_id", r.id).Msg("delete failed")
			} else {
				m.statusErr = ""
				m.statusMsg = "Object deleted"
				m.mu.Lock()
				delete(m.devices, r.id)
				filtered := m.events[:0]
				for _, e := range m.events {
					if !eventBelongsToDevice(e.DeviceID, r.id) {
						filtered = append(filtered, e)
					}
				}
				m.events = filtered
				m.applyFiltersLocked()
				m.mu.Unlock()
			}
			m.refreshMainUI()
		default:
			return
		}
	}
}

func (m *model) pullRfResult() {
	for {
		select {
		case r := <-m.rfResult:
			m.rfBusy.Store(false)
			if r.err != nil {
				m.statusErr = "Relay filter error: " + r.err.Error()
				log.Error().Err(r.err).Msg("relay filter result error")
				m.refreshRelayUI()
				continue
			}
			m.runOnUI(func() {
				m.statusErr = ""
				if m.rfOpen {
					m.statusMsg = "Relay filter rule saved"
					log.Info().Msg("relay filter rule updated from ui")
				} else {
					m.statusMsg = "Relay filter rule loaded"
				}
				m.openRelayFilterWindow()
				m.loadRfRule(r.rule)
				m.refreshRelayUI()
				m.refreshMainUI()
			})
		default:
			return
		}
	}
}

func (m *model) updateStatusBanner() {
	if m.statusBanner == nil {
		return
	}
	text := ""
	bg := color.NRGBA{}
	fg := cAccent
	show := false
	if strings.TrimSpace(m.statusErr) != "" {
		text = m.statusErr
		bg = cBadSoft
		fg = cBad
		show = true
	} else if strings.TrimSpace(m.statusMsg) != "" {
		text = m.statusMsg
		bg = cAccentSoft
		fg = cAccent
		show = true
	}
	m.statusBanner.text.Text = text
	m.statusBanner.text.Color = fg
	m.statusBanner.bg.FillColor = bg
	if show {
		m.statusBanner.box.Show()
	} else {
		m.statusBanner.box.Hide()
	}
	m.statusBanner.text.Refresh()
	m.statusBanner.bg.Refresh()
	m.statusBanner.box.Refresh()
}

