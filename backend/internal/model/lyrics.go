package model

import "time"

type Lyrics struct {
	ID          int64     `db:"id" json:"id"`
	FilePath    string    `db:"file_path" json:"file_path"`
	ContentHash string    `db:"content_hash" json:"content_hash"`
	ModifiedAt  time.Time `db:"modified_at" json:"modified_at"`
	FileContent []byte    `db:"file_content" json:"file_content"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type FileMeta struct {
	Path        string    `json:"path"`
	ContentHash string    `json:"content_hash"`
	ModifiedAt  time.Time `json:"modified_at"`
	Size        int64     `json:"size"`
	Content     []byte    `json:"content,omitempty"`
}
