CREATE TABLE IF NOT EXISTS source_folders (
	id SERIAL NOT NULL PRIMARY KEY,
	folder TEXT UNIQUE
);

CREATE TABLE IF NOT EXISTS messages (
	id SERIAL NOT NULL PRIMARY KEY,
	parent_id INTEGER REFERENCES messages(id),
	telegram_id INTEGER,
	message_type TEXT,
	message_date INTEGER DEFAULT 0,
	message_from TEXT,
	from_id TEXT,
	forwarded_from TEXT,
	forwarded_from_id TEXT,
	-- additional field not present in tg/json
	-- 0 is default for unresolved
	resolution INTEGER DEFAULT 0,
	source_id INTEGER REFERENCES source_folders(id)
);

CREATE TABLE IF NOT EXISTS message_texts (
	id SERIAL NOT NULL PRIMARY KEY,
	message_id INTEGER REFERENCES messages(id),
	text_type TEXT,
	-- lmao
	text_text TEXT,
	href TEXT
);

CREATE TABLE IF NOT EXISTS photos (
	id SERIAL NOT NULL PRIMARY KEY,
	message_id INTEGER REFERENCES messages(id),
	photo TEXT
);

CREATE TABLE IF NOT EXISTS files (
	id SERIAL NOT NULL PRIMARY KEY,
	message_id INTEGER REFERENCES messages(id),
	file_file TEXT,
	media_type TEXT,
	mime_type TEXT
);
