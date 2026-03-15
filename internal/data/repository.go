package data

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"cid_gio_gio/internal/core"

	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"
)

type Repository struct {
	db        *sql.DB
	writeLock sync.Mutex
	cacheMu   sync.RWMutex
	typeRefs  map[string]int64
	zoneRefs  map[string]int64
	catRefs   map[string]int64
}

func NewRepository(path string) (*Repository, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=1", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)

	repo := &Repository{
		db:       db,
		typeRefs: make(map[string]int64, 32),
		zoneRefs: make(map[string]int64, 1024),
		catRefs:  make(map[string]int64, 2048),
	}
	if err := repo.initSchema(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := repo.ensureEventTypes(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := repo.preloadRefs(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	log.Info().Str("db_path", path).Msg("repository ready")
	return repo, nil
}

func (r *Repository) Close() error { return r.db.Close() }

func (r *Repository) initSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    number INTEGER NOT NULL UNIQUE,
    name TEXT,
    client_addr TEXT,
    last_event TEXT,
    last_event_time INTEGER
);
CREATE TABLE IF NOT EXISTS event_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS event_catalog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    type_text TEXT,
    desc_text TEXT,
    category TEXT,
    event_type_id INTEGER,
    version INTEGER DEFAULT 1
);
CREATE TABLE IF NOT EXISTS event_zones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    zone_text TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    time INTEGER,
    device_ref_id INTEGER NOT NULL,
    event_catalog_id INTEGER NOT NULL,
    zone_ref_id INTEGER,
    group_no TEXT DEFAULT '',
    priority INTEGER,
    event_type_id INTEGER NOT NULL,
    is_test INTEGER NOT NULL DEFAULT 0,
    relay_blocked INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(device_ref_id) REFERENCES devices(id) ON DELETE CASCADE,
    FOREIGN KEY(event_catalog_id) REFERENCES event_catalog(id) ON DELETE CASCADE,
    FOREIGN KEY(event_type_id) REFERENCES event_types(id) ON DELETE CASCADE,
    FOREIGN KEY(zone_ref_id) REFERENCES event_zones(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS relay_filter_meta (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS relay_filter_objects (
    object_id INTEGER PRIMARY KEY REFERENCES devices(number) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS relay_filter_groups (
    group_no INTEGER PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS relay_filter_codes (
    code TEXT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS relay_filter_object_codes (
    object_id INTEGER REFERENCES devices(number) ON DELETE CASCADE,
    code TEXT,
    PRIMARY KEY (object_id, code)
);
CREATE TABLE IF NOT EXISTS relay_filter_zones (
    code TEXT NOT NULL,
    object_id INTEGER NOT NULL DEFAULT 0,
    zone INTEGER NOT NULL,
    PRIMARY KEY (code, object_id, zone),
    FOREIGN KEY(object_id) REFERENCES devices(number) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS relay_filter_partitions (
    code TEXT NOT NULL,
    object_id INTEGER NOT NULL DEFAULT 0,
    partition_no INTEGER NOT NULL,
    PRIMARY KEY (code, object_id, partition_no),
    FOREIGN KEY(object_id) REFERENCES devices(number) ON DELETE CASCADE
);
INSERT INTO relay_filter_meta(id, enabled) VALUES (1, 0) ON CONFLICT(id) DO NOTHING;
`
	// CREATE INDEX IF NOT EXISTS idx_devices_number ON devices(number);
	// CREATE INDEX IF NOT EXISTS idx_events_time_desc ON events(time DESC);
	// CREATE INDEX IF NOT EXISTS idx_events_device_time_desc ON events(device_ref_id, time DESC);
	// CREATE INDEX IF NOT EXISTS idx_events_event_type_time_desc ON events(event_type_id, time DESC);
	// CREATE INDEX IF NOT EXISTS idx_event_catalog_type_text ON event_catalog(type_text);
	// CREATE INDEX IF NOT EXISTS idx_event_catalog_desc_text ON event_catalog(desc_text);

	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return err
	}
	// Migrate relay_blocked column if it doesn't exist
	_, _ = r.db.ExecContext(ctx, "ALTER TABLE events ADD COLUMN relay_blocked INTEGER NOT NULL DEFAULT 0")
	return nil
}

func (r *Repository) ensureEventTypes(ctx context.Context) error {
	types := map[string]string{
		"alarm":    "Alarm",
		"test":     "Test",
		"fault":    "Fault",
		"guard":    "Guard",
		"disguard": "Disarm",
		"other":    "Other",
	}
	for key, title := range types {
		if err := r.execWrite(ctx, "INSERT INTO event_types(key,title) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET title=excluded.title", key, title); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpsertEventCatalog(ctx context.Context, entries []core.EventCatalogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		typeID, err := ensureTypeRefTx(ctx, tx, entry.Category)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO event_catalog(code, type_text, desc_text, category, event_type_id, version)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(code) DO UPDATE SET
    type_text = excluded.type_text,
    desc_text = excluded.desc_text,
    category = excluded.category,
    event_type_id = excluded.event_type_id`, entry.Code, entry.Type, entry.Desc, entry.Category, typeID, entry.Version); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Warm in-memory refs after bulk catalog updates.
	return r.preloadCatalogRefs(ctx)
}

func (r *Repository) GetDevices(ctx context.Context) ([]core.DeviceDTO, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT number, COALESCE(name,''), COALESCE(client_addr,''), COALESCE(last_event,''), COALESCE(last_event_time, 0) FROM devices ORDER BY number ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.DeviceDTO{}
	for rows.Next() {
		var number int
		var name string
		var clientAddr string
		var lastEvent string
		var ts any
		if err := rows.Scan(&number, &name, &clientAddr, &lastEvent, &ts); err != nil {
			return nil, err
		}
		out = append(out, core.DeviceDTO{ID: number, Name: name, ClientAddr: clientAddr, LastEvent: lastEvent, LastEventTime: parseDBTimestamp(ts)})
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	log.Debug().Int("rows", len(out)).Msg("repo GetDevices loaded")
	return out, nil
}

func (r *Repository) GetEventCatalogCategories(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT code, COALESCE(category,'') FROM event_catalog")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var code string
		var cat string
		if err := rows.Scan(&code, &cat); err != nil {
			return out, err
		}
		code = strings.ToUpper(strings.TrimSpace(code))
		cat = strings.ToLower(strings.TrimSpace(cat))
		if code == "" {
			continue
		}
		out[code] = cat
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func (r *Repository) GetEvents(ctx context.Context, limit int) ([]core.EventDTO, error) {
	if limit < 1 {
		limit = 1
	}
	return r.queryEvents(ctx, "WHERE 1=1", []any{limit})
}

func (r *Repository) GetEventsFiltered(ctx context.Context, limit int, typeFilter string, hideTests bool, hideBlocked bool, query string) ([]core.EventDTO, error) {
	return r.getEventsFiltered(ctx, 0, limit, time.Time{}, time.Time{}, typeFilter, hideTests, hideBlocked, query, false)
}

func (r *Repository) GetDeviceEventsFiltered(ctx context.Context, deviceID int, limit int, from, to time.Time, typeFilter string, hideTests bool, hideBlocked bool, query string) ([]core.EventDTO, error) {
	return r.getEventsFiltered(ctx, deviceID, limit, from, to, typeFilter, hideTests, hideBlocked, query, true)
}

func (r *Repository) getEventsFiltered(ctx context.Context, deviceID int, limit int, from, to time.Time, typeFilter string, hideTests bool, hideBlocked bool, query string, withDevice bool) ([]core.EventDTO, error) {
	if limit < 1 {
		limit = 1
	}
	where := "WHERE 1=1"
	args := []any{}
	if withDevice {
		where += " AND e.device_ref_id = (SELECT id FROM devices WHERE number=?)"
		args = append(args, deviceID)
		if !from.IsZero() {
			where += " AND e.time >= ?"
			args = append(args, from.Unix())
		}
		if !to.IsZero() {
			where += " AND e.time <= ?"
			args = append(args, to.Unix())
		}
	}
	where, args = applyFilters(where, args, typeFilter, hideTests, hideBlocked, query)
	args = append(args, limit)
	return r.queryEvents(ctx, where, args)
}

func applyFilters(where string, args []any, typeFilter string, hideTests bool, hideBlocked bool, query string) (string, []any) {
	typeFilter = strings.ToLower(strings.TrimSpace(typeFilter))
	if typeFilter != "" && typeFilter != "all" {
		where += " AND et.key = ?"
		args = append(args, typeFilter)
	}
	if hideTests {
		where += " AND et.key <> 'test'"
	}
	if hideBlocked {
		where += " AND COALESCE(e.relay_blocked, 0) = 0"
	}
	search := strings.ToLower(strings.TrimSpace(query))
	if search == "" {
		return where, args
	}
	patterns := strings.Fields(search)
	if len(patterns) == 0 {
		patterns = []string{search}
	}
	for _, p := range patterns {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		like := "%" + p + "%"
		if n, err := strconv.Atoi(p); err == nil {
			numPrefix := p + "%"
			where += " AND (" +
				"d.number = ? OR CAST(d.number AS TEXT) LIKE ? OR " +
				"COALESCE(e.group_no,'') LIKE ? COLLATE NOCASE OR " +
				"COALESCE(z.zone_text,'') LIKE ? COLLATE NOCASE OR " +
				"e.event_catalog_id IN (SELECT ec.id FROM event_catalog ec WHERE ec.code = ? COLLATE NOCASE OR ec.type_text LIKE ? COLLATE NOCASE OR ec.desc_text LIKE ? COLLATE NOCASE) OR " +
				"e.event_type_id IN (SELECT et2.id FROM event_types et2 WHERE et2.key = ? COLLATE NOCASE OR et2.title LIKE ? COLLATE NOCASE)" +
				")"
			args = append(args, n, numPrefix, like, like, p, like, like, p, like)
			continue
		}
		where += " AND (" +
			"CAST(d.number AS TEXT) LIKE ? OR " +
			"COALESCE(e.group_no,'') LIKE ? COLLATE NOCASE OR " +
			"COALESCE(z.zone_text,'') LIKE ? COLLATE NOCASE OR " +
			"e.event_catalog_id IN (SELECT ec.id FROM event_catalog ec WHERE ec.code LIKE ? COLLATE NOCASE OR ec.type_text LIKE ? COLLATE NOCASE OR ec.desc_text LIKE ? COLLATE NOCASE) OR " +
			"e.event_type_id IN (SELECT et2.id FROM event_types et2 WHERE et2.key LIKE ? COLLATE NOCASE OR et2.title LIKE ? COLLATE NOCASE)" +
			")"
		args = append(args, like, like, like, like, like, like, like, like)
	}
	return where, args
}

func (r *Repository) queryEvents(ctx context.Context, where string, args []any) ([]core.EventDTO, error) {
	query := `
SELECT
    e.time,
    CAST(d.number AS TEXT),
    c.code,
    COALESCE(c.type_text, ''),
    COALESCE(c.desc_text, ''),
    COALESCE(z.zone_text, ''),
    COALESCE(e.priority, 0),
    COALESCE(et.key, 'other'),
    COALESCE(e.relay_blocked, 0)
FROM events e
JOIN event_catalog c ON c.id = e.event_catalog_id
JOIN event_types et ON et.id = e.event_type_id
JOIN devices d ON d.id = e.device_ref_id
LEFT JOIN event_zones z ON z.id = e.zone_ref_id
` + where + `
ORDER BY e.time DESC
LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []core.EventDTO{}
	for rows.Next() {
		var ts any
		var relayBlocked int
		var evt core.EventDTO
		if err := rows.Scan(&ts, &evt.DeviceID, &evt.Code, &evt.Type, &evt.Desc, &evt.Zone, &evt.Priority, &evt.Category, &relayBlocked); err != nil {
			return nil, err
		}
		evt.Time = parseDBTimestamp(ts)
		evt.RelayBlocked = relayBlocked != 0
		out = append(out, evt)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	log.Debug().Int("rows", len(out)).Msg("repo queryEvents loaded")
	return out, nil
}

func (r *Repository) SaveDevice(ctx context.Context, device core.DeviceDTO) error {
	return r.execWrite(ctx, `
INSERT INTO devices(number, name, client_addr, last_event, last_event_time)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(number) DO UPDATE SET
    name=excluded.name,
    client_addr=excluded.client_addr,
    last_event=excluded.last_event,
    last_event_time=excluded.last_event_time`, device.ID, device.Name, strings.TrimSpace(device.ClientAddr), device.LastEvent, device.LastEventTime.Unix())
}

func (r *Repository) SaveEvent(ctx context.Context, evt core.EventDTO) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	cache := newBatchRefCache()
	if err := r.saveEventTxCached(ctx, tx, evt, cache); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *Repository) SaveEventsBatch(ctx context.Context, events []core.EventDTO) error {
	if len(events) == 0 {
		return nil
	}
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	cache := newBatchRefCache()
	for _, evt := range events {
		if err := r.saveEventTxCached(ctx, tx, evt, cache); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) GetRelayFilterRule(ctx context.Context) (core.RelayFilterRule, error) {
	var rule core.RelayFilterRule
	rule.ObjectCodes = make(map[int][]string)

	var enabled int
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(enabled,0) FROM relay_filter_meta WHERE id = 1").Scan(&enabled)
	if err != nil && err != sql.ErrNoRows {
		return rule, err
	}
	rule.Enabled = enabled != 0

	rows, err := r.db.QueryContext(ctx, "SELECT object_id FROM relay_filter_objects ORDER BY object_id ASC")
	if err == nil {
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err == nil {
				rule.ObjectIDs = append(rule.ObjectIDs, id)
			}
		}
		rows.Close()
	}

	rows, err = r.db.QueryContext(ctx, "SELECT group_no FROM relay_filter_groups ORDER BY group_no ASC")
	if err == nil {
		for rows.Next() {
			var g int
			if err := rows.Scan(&g); err == nil {
				rule.GroupNumbers = append(rule.GroupNumbers, g)
			}
		}
		rows.Close()
	}

	rows, err = r.db.QueryContext(ctx, "SELECT code FROM relay_filter_codes ORDER BY code ASC")
	if err == nil {
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err == nil {
				rule.Codes = append(rule.Codes, c)
			}
		}
		rows.Close()
	}

	rows, err = r.db.QueryContext(ctx, "SELECT object_id, code FROM relay_filter_object_codes ORDER BY object_id ASC, code ASC")
	if err == nil {
		for rows.Next() {
			var id int
			var c string
			if err := rows.Scan(&id, &c); err == nil {
				rule.ObjectCodes[id] = append(rule.ObjectCodes[id], c)
			}
		}
		rows.Close()
	}

	rule.CodeDetails = make(map[string]core.RelayFilterDetail)
	rule.ObjCodeDetails = make(map[int]map[string]core.RelayFilterDetail)

	// Load zones
	rows, err = r.db.QueryContext(ctx, "SELECT code, object_id, zone FROM relay_filter_zones")
	if err == nil {
		for rows.Next() {
			var code string
			var objID, zone int
			if err := rows.Scan(&code, &objID, &zone); err == nil {
				if objID == 0 {
					d := rule.CodeDetails[code]
					d.Zones = append(d.Zones, zone)
					rule.CodeDetails[code] = d
				} else {
					if rule.ObjCodeDetails[objID] == nil {
						rule.ObjCodeDetails[objID] = make(map[string]core.RelayFilterDetail)
					}
					d := rule.ObjCodeDetails[objID][code]
					d.Zones = append(d.Zones, zone)
					rule.ObjCodeDetails[objID][code] = d
				}
			}
		}
		rows.Close()
	}

	// Load partitions
	rows, err = r.db.QueryContext(ctx, "SELECT code, object_id, partition_no FROM relay_filter_partitions")
	if err == nil {
		for rows.Next() {
			var code string
			var objID, part int
			if err := rows.Scan(&code, &objID, &part); err == nil {
				if objID == 0 {
					d := rule.CodeDetails[code]
					d.Partitions = append(d.Partitions, part)
					rule.CodeDetails[code] = d
				} else {
					if rule.ObjCodeDetails[objID] == nil {
						rule.ObjCodeDetails[objID] = make(map[string]core.RelayFilterDetail)
					}
					d := rule.ObjCodeDetails[objID][code]
					d.Partitions = append(d.Partitions, part)
					rule.ObjCodeDetails[objID][code] = d
				}
			}
		}
		rows.Close()
	}

	return rule, nil
}

func (r *Repository) SaveRelayFilterRule(ctx context.Context, rule core.RelayFilterRule) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	enabled := 0
	if rule.Enabled {
		enabled = 1
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO relay_filter_meta(id, enabled) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled", enabled)
	if err != nil {
		return err
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM relay_filter_objects")
	for _, id := range rule.ObjectIDs {
		_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_objects(object_id) VALUES (?)", id)
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM relay_filter_groups")
	for _, g := range rule.GroupNumbers {
		_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_groups(group_no) VALUES (?)", g)
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM relay_filter_codes")
	for _, c := range rule.Codes {
		_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_codes(code) VALUES (?)", c)
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM relay_filter_object_codes")
	for id, codes := range rule.ObjectCodes {
		for _, c := range codes {
			_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_object_codes(object_id, code) VALUES (?, ?)", id, c)
		}
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM relay_filter_zones")
	_, _ = tx.ExecContext(ctx, "DELETE FROM relay_filter_partitions")

	for code, d := range rule.CodeDetails {
		for _, z := range d.Zones {
			_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_zones(code, object_id, zone) VALUES (?, 0, ?)", code, z)
		}
		for _, p := range d.Partitions {
			_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_partitions(code, object_id, partition_no) VALUES (?, 0, ?)", code, p)
		}
	}

	for id, codeMap := range rule.ObjCodeDetails {
		for code, d := range codeMap {
			for _, z := range d.Zones {
				_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_zones(code, object_id, zone) VALUES (?, ?, ?)", code, id, z)
			}
			for _, p := range d.Partitions {
				_, _ = tx.ExecContext(ctx, "INSERT INTO relay_filter_partitions(code, object_id, partition_no) VALUES (?, ?, ?)", code, id, p)
			}
		}
	}

	return tx.Commit()
}

func (r *Repository) DeleteDeviceWithHistory(ctx context.Context, deviceID int) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func() { _ = tx.Rollback() }
	if _, err := tx.ExecContext(ctx, "DELETE FROM devices WHERE number=?", deviceID); err != nil {
		rollback()
		return err
	}
	return tx.Commit()
}

func ensureDeviceRefTx(ctx context.Context, tx *sql.Tx, deviceID string) (int64, error) {
	num, err := strconv.Atoi(strings.TrimSpace(deviceID))
	if err != nil {
		num = 0
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO devices(number,name) VALUES (?,?) ON CONFLICT(number) DO NOTHING", num, fmt.Sprintf("%03d", num)); err != nil {
		return 0, err
	}
	return queryInt64Tx(ctx, tx, "SELECT id FROM devices WHERE number=?", num)
}

func ensureTypeRefTx(ctx context.Context, tx *sql.Tx, category string) (int64, error) {
	key := strings.ToLower(strings.TrimSpace(category))
	if key == "" {
		key = "other"
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO event_types(key,title) VALUES (?,?) ON CONFLICT(key) DO NOTHING", key, key); err != nil {
		return 0, err
	}
	return queryInt64Tx(ctx, tx, "SELECT id FROM event_types WHERE key=?", key)
}

func ensureCatalogRefTx(ctx context.Context, tx *sql.Tx, evt core.EventDTO, typeID int64) (int64, error) {
	_, err := tx.ExecContext(ctx, `
INSERT INTO event_catalog(code,type_text,desc_text,category,event_type_id,version)
VALUES (?, ?, ?, ?, ?, 1)
ON CONFLICT(code) DO UPDATE SET
    type_text=COALESCE(NULLIF(excluded.type_text,''),event_catalog.type_text),
    desc_text=COALESCE(NULLIF(excluded.desc_text,''),event_catalog.desc_text),
    category=COALESCE(NULLIF(excluded.category,''),event_catalog.category),
    event_type_id=COALESCE(excluded.event_type_id,event_catalog.event_type_id)`, evt.Code, evt.Type, evt.Desc, evt.Category, typeID)
	if err != nil {
		return 0, err
	}
	return queryInt64Tx(ctx, tx, "SELECT id FROM event_catalog WHERE code=?", evt.Code)
}

func ensureZoneRefTx(ctx context.Context, tx *sql.Tx, zone string) (any, error) {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO event_zones(zone_text) VALUES (?) ON CONFLICT(zone_text) DO NOTHING", zone); err != nil {
		return nil, err
	}
	id, err := queryInt64Tx(ctx, tx, "SELECT id FROM event_zones WHERE zone_text=?", zone)
	if err != nil {
		return nil, err
	}
	return id, nil
}

type zoneRefCache struct {
	id    int64
	valid bool
}

type batchRefCache struct {
	device map[string]int64
	etype  map[string]int64
	zone   map[string]zoneRefCache
	cat    map[string]int64
}

func newBatchRefCache() *batchRefCache {
	return &batchRefCache{
		device: make(map[string]int64, 512),
		etype:  make(map[string]int64, 32),
		zone:   make(map[string]zoneRefCache, 1024),
		cat:    make(map[string]int64, 2048),
	}
}

func (r *Repository) saveEventTxCached(ctx context.Context, tx *sql.Tx, evt core.EventDTO, cache *batchRefCache) error {
	deviceKey := strings.TrimSpace(evt.DeviceID)
	deviceRef, ok := cache.device[deviceKey]
	if !ok {
		var err error
		deviceRef, err = ensureDeviceRefTx(ctx, tx, deviceKey)
		if err != nil {
			return err
		}
		cache.device[deviceKey] = deviceRef
	}

	typeKey := strings.ToLower(strings.TrimSpace(evt.Category))
	if typeKey == "" {
		typeKey = "other"
	}
	typeRef, ok := cache.etype[typeKey]
	if !ok {
		if v, exists := r.loadTypeRef(typeKey); exists {
			typeRef = v
		} else {
			var err error
			typeRef, err = ensureTypeRefTx(ctx, tx, typeKey)
			if err != nil {
				return err
			}
			r.storeTypeRef(typeKey, typeRef)
		}
		cache.etype[typeKey] = typeRef
	}

	codeKey := strings.TrimSpace(evt.Code)
	if codeKey == "" {
		return fmt.Errorf("empty event code")
	}
	catalogRef, ok := cache.cat[codeKey]
	if !ok {
		if v, exists := r.loadCatalogRef(codeKey); exists {
			catalogRef = v
		} else {
			var err error
			catalogRef, err = ensureCatalogRefTx(ctx, tx, evt, typeRef)
			if err != nil {
				return err
			}
			r.storeCatalogRef(codeKey, catalogRef)
		}
		cache.cat[codeKey] = catalogRef
	}

	zoneKey := strings.TrimSpace(evt.Zone)
	zoneMeta, ok := cache.zone[zoneKey]
	if !ok {
		if zoneKey == "" {
			zoneMeta = zoneRefCache{}
		} else {
			if v, exists := r.loadZoneRef(zoneKey); exists {
				zoneMeta = zoneRefCache{id: v, valid: true}
			} else {
				zoneAny, err := ensureZoneRefTx(ctx, tx, zoneKey)
				if err != nil {
					return err
				}
				if zoneAny == nil {
					zoneMeta = zoneRefCache{}
				} else {
					zoneMeta = zoneRefCache{id: zoneAny.(int64), valid: true}
					r.storeZoneRef(zoneKey, zoneMeta.id)
				}
			}
		}
		cache.zone[zoneKey] = zoneMeta
	}

	isTest := 0
	if strings.EqualFold(evt.Category, "test") {
		isTest = 1
	}
	relayBlocked := 0
	if evt.RelayBlocked {
		relayBlocked = 1
	}

	var zoneRef any
	if zoneMeta.valid {
		zoneRef = zoneMeta.id
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO events(time, device_ref_id, event_catalog_id, zone_ref_id, group_no, priority, event_type_id, is_test, relay_blocked)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, evt.Time.Unix(), deviceRef, catalogRef, zoneRef, parseGroup(evt.Zone), evt.Priority, typeRef, isTest, relayBlocked)
	if err != nil {
		return err
	}
	_, _ = res.LastInsertId()
	return nil
}

func (r *Repository) preloadRefs(ctx context.Context) error {
	if err := r.preloadTypeRefs(ctx); err != nil {
		return err
	}
	if err := r.preloadZoneRefs(ctx); err != nil {
		return err
	}
	if err := r.preloadCatalogRefs(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Repository) preloadTypeRefs(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, "SELECT id, key FROM event_types")
	if err != nil {
		return err
	}
	defer rows.Close()
	tmp := make(map[string]int64, 32)
	for rows.Next() {
		var id int64
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			return err
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			tmp[key] = id
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.cacheMu.Lock()
	r.typeRefs = tmp
	r.cacheMu.Unlock()
	return nil
}

func (r *Repository) preloadZoneRefs(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, "SELECT id, zone_text FROM event_zones")
	if err != nil {
		return err
	}
	defer rows.Close()
	tmp := make(map[string]int64, 1024)
	for rows.Next() {
		var id int64
		var zone string
		if err := rows.Scan(&id, &zone); err != nil {
			return err
		}
		zone = strings.TrimSpace(zone)
		if zone != "" {
			tmp[zone] = id
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.cacheMu.Lock()
	r.zoneRefs = tmp
	r.cacheMu.Unlock()
	return nil
}

func (r *Repository) preloadCatalogRefs(ctx context.Context) error {
	rows, err := r.db.QueryContext(ctx, "SELECT id, code FROM event_catalog")
	if err != nil {
		return err
	}
	defer rows.Close()
	tmp := make(map[string]int64, 2048)
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return err
		}
		code = strings.TrimSpace(code)
		if code != "" {
			tmp[code] = id
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.cacheMu.Lock()
	r.catRefs = tmp
	r.cacheMu.Unlock()
	return nil
}

func (r *Repository) loadTypeRef(key string) (int64, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	v, ok := r.typeRefs[key]
	return v, ok
}

func (r *Repository) storeTypeRef(key string, id int64) {
	r.cacheMu.Lock()
	r.typeRefs[key] = id
	r.cacheMu.Unlock()
}

func (r *Repository) loadCatalogRef(code string) (int64, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	v, ok := r.catRefs[code]
	return v, ok
}

func (r *Repository) storeCatalogRef(code string, id int64) {
	r.cacheMu.Lock()
	r.catRefs[code] = id
	r.cacheMu.Unlock()
}

func (r *Repository) loadZoneRef(zone string) (int64, bool) {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()
	v, ok := r.zoneRefs[zone]
	return v, ok
}

func (r *Repository) storeZoneRef(zone string, id int64) {
	r.cacheMu.Lock()
	r.zoneRefs[zone] = id
	r.cacheMu.Unlock()
}

func queryInt64Tx(ctx context.Context, tx *sql.Tx, q string, args ...any) (int64, error) {
	var v int64
	err := tx.QueryRowContext(ctx, q, args...).Scan(&v)
	return v, err
}

func (r *Repository) execWrite(ctx context.Context, query string, args ...any) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *Repository) CompactAndVacuum(ctx context.Context) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	return r.compactAndVacuumLocked(ctx)
}

func (r *Repository) compactAndVacuumLocked(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		log.Debug().Err(err).Msg("wal checkpoint passive failed")
	}
	if _, err := r.db.ExecContext(ctx, "ANALYZE"); err != nil {
		log.Debug().Err(err).Msg("analyze failed")
	}
	return nil
}

func parseGroup(zone string) string {
	parts := strings.Split(zone, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToLower(p), "group") {
			idx := strings.Index(p, " ")
			if idx >= 0 && idx+1 < len(p) {
				return strings.TrimSpace(p[idx+1:])
			}
		}
	}
	return ""
}

func parseDBTimestamp(v any) time.Time {
	switch t := v.(type) {
	case int64:
		return unixToLocal(t)
	case int:
		return unixToLocal(int64(t))
	case float64:
		return unixToLocal(int64(t))
	case []byte:
		return parseDBTimestamp(string(t))
	case string:
		t = strings.TrimSpace(t)
		if t == "" {
			return time.Time{}
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return unixToLocal(n)
		}
		if dt, err := time.Parse(time.RFC3339, t); err == nil {
			return dt.Local()
		}
	}
	return time.Time{}
}

func unixToLocal(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	if v > 32503680000 {
		v = v / 1000
	}
	return time.Unix(v, 0).Local()
}
