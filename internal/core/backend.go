package core

import (
	"context"
	"time"
	"cid_fyne/internal/config"
)

// Backend defines the interface for the application logic,
// allowing different UIs to interact with the system in a unified way.
type Backend interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetConfig() config.AppConfig
	Bootstrap(ctx context.Context, limit int) (BootstrapDTO, error)
	SubscribeDevice(func(DeviceDTO))
	SubscribeEvent(func(EventDTO))
	SubscribeDeviceDeleted(func(int))
	GetStats() StatsDTO
	FilterEvents(ctx context.Context, limit int, eventType string, hideTests bool, hideBlocked bool, query string) ([]EventDTO, error)
	FilterDeviceHistory(ctx context.Context, deviceID, limit int, from, to time.Time, eventType string, hideTests bool, hideBlocked bool, query string) ([]EventDTO, error)
	DeleteDeviceWithHistory(ctx context.Context, deviceID int) error
	SaveConfig(ctx context.Context, cfg config.AppConfig) error
	GetRelayFilterRule(ctx context.Context) (RelayFilterRule, error)
	SaveRelayFilterRule(ctx context.Context, rule RelayFilterRule) error
	GetEventList() []CIDEvent
	GetEventCatalogCategories() map[string]string
	GetDevices() []DeviceDTO
	GetEventTypes(ctx context.Context) ([]EventTypeDTO, error)
	SaveEventTypeColors(ctx context.Context, key, color, fontColor string) error
}
