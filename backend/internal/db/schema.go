package db

import "github.com/jmoiron/sqlx"

const SchemaSQL = `
CREATE SCHEMA IF NOT EXISTS lyrics_schema;
CREATE TABLE IF NOT EXISTS lyrics_schema.lyrics (
    id SERIAL PRIMARY KEY,
    file_path TEXT NOT NULL UNIQUE,
    content_hash TEXT NOT NULL,
    modified_at TIMESTAMP NOT NULL,
    file_content BYTEA,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_lyrics_path ON lyrics_schema.lyrics(file_path);
`

func InitSchema(db *sqlx.DB) error {
	_, err := db.Exec(SchemaSQL)
	return err
}
