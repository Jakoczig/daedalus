CREATE TABLE itineraries(id INTEGER PRIMARY KEY,commander_fid TEXT NOT NULL REFERENCES commander(fid),name TEXT NOT NULL,source_hash TEXT NOT NULL,created_at TEXT NOT NULL,next_hop INTEGER NOT NULL DEFAULT 0,state TEXT NOT NULL DEFAULT 'awaiting_arrival',active INTEGER NOT NULL DEFAULT 0,last_event INTEGER NOT NULL DEFAULT 0,origin TEXT NOT NULL DEFAULT 'import',UNIQUE(commander_fid,name));
CREATE UNIQUE INDEX itinerary_active ON itineraries(commander_fid) WHERE active=1;
CREATE TABLE itinerary_waypoints(itinerary_id INTEGER REFERENCES itineraries(id),ordinal INTEGER NOT NULL,name TEXT NOT NULL,address TEXT NOT NULL,neutron INTEGER NOT NULL,refuel INTEGER NOT NULL,coordinates TEXT,PRIMARY KEY(itinerary_id,ordinal));
CREATE TABLE itinerary_edits(id INTEGER PRIMARY KEY,itinerary_id INTEGER REFERENCES itineraries(id),next_hop INTEGER NOT NULL,origin TEXT NOT NULL,source_event INTEGER REFERENCES journal_events(event_id),at TEXT NOT NULL);
CREATE TABLE field_tally(source_event INTEGER NOT NULL REFERENCES journal_events(event_id),ordinal INTEGER NOT NULL,commander_fid TEXT NOT NULL,system_address TEXT NOT NULL,faction TEXT NOT NULL,category TEXT NOT NULL,observed_at TEXT NOT NULL,amount TEXT NOT NULL,unit TEXT NOT NULL,qualifier TEXT NOT NULL,mission_id TEXT NOT NULL,formula TEXT NOT NULL,PRIMARY KEY(source_event,ordinal,category,system_address,faction));
CREATE INDEX field_tally_scope ON field_tally(commander_fid,system_address,faction,source_event DESC);
CREATE INDEX field_tally_page ON field_tally(commander_fid,source_event DESC);
CREATE TABLE field_ticks(at TEXT PRIMARY KEY,source TEXT NOT NULL,fetched_at TEXT NOT NULL);
CREATE TABLE bio_positions(source_event INTEGER PRIMARY KEY REFERENCES journal_events(event_id),commander_fid TEXT NOT NULL,system_address TEXT NOT NULL,body_id TEXT NOT NULL,latitude REAL NOT NULL,longitude REAL NOT NULL,radius REAL NOT NULL,status_at TEXT NOT NULL,evidence TEXT NOT NULL);
CREATE INDEX bio_position_scope ON bio_positions(commander_fid,system_address,body_id,source_event DESC);
CREATE TABLE builtin_modules(id TEXT PRIMARY KEY,enabled INTEGER NOT NULL);

CREATE INDEX body_commander_current ON body_observations(commander_fid,system_address,body_id,source_event_id DESC);
