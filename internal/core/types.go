package core

import "time"

type DeviceDTO struct {
	ID            int
	Name          string
	ClientAddr    string
	LastEvent     string
	LastEventTime time.Time
}

type EventDTO struct {
	Time         time.Time
	DeviceID     string
	Code         string
	Type         string
	Desc         string
	Zone         string
	Priority     int
	Category     string
	RelayBlocked bool
}

type RelayFilterDetail struct {
	Zones      []int `json:"zones"`
	Partitions []int `json:"partitions"`
}

type RelayFilterRule struct {
	Enabled        bool                                 `json:"enabled"`
	ObjectIDs      []int                                `json:"object_ids"`
	GroupNumbers   []int                                `json:"group_numbers"`
	Codes          []string                             `json:"codes"`
	ObjectCodes    map[int][]string                     `json:"object_codes"`
	CodeDetails    map[string]RelayFilterDetail         `json:"code_details"`
	ObjCodeDetails map[int]map[string]RelayFilterDetail `json:"obj_code_details"`
}

type StatsDTO struct {
	Accepted   int64
	Rejected   int64
	Reconnects int64
	ReceivedPS int64
	ReceivedPM int64
	Clients    int
	Uptime     string
	Connected  bool
}

type BootstrapDTO struct {
	Devices []DeviceDTO
	Events  []EventDTO
}

type EventTypeDTO struct {
	ID        int
	Key       string
	Title     string
	Color     string
	FontColor string
}

type EventCatalogEntry struct {
	Code     string
	Type     string
	Desc     string
	Category string
	Version  int
}
