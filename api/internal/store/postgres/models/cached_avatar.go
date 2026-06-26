package models

import "time"

// CachedAvatar is a locally-cached avatar image (migration 000017). Keyed by a
// hash of the source URL so the same URL dedupes and a changed URL misses.
type CachedAvatar struct {
	URLHash     string    `gorm:"column:url_hash;primaryKey" json:"url_hash"`
	SourceURL   string    `gorm:"column:source_url;not null" json:"source_url"`
	ContentType string    `gorm:"column:content_type;not null" json:"content_type"`
	Bytes       []byte    `gorm:"column:bytes;not null" json:"-"`
	Width       int       `gorm:"column:width;not null" json:"width"`
	Height      int       `gorm:"column:height;not null" json:"height"`
	ByteSize    int       `gorm:"column:byte_size;not null" json:"byte_size"`
	FetchedAt   time.Time `gorm:"column:fetched_at;not null" json:"fetched_at"`
}

func (CachedAvatar) TableName() string { return "cached_avatars" }
