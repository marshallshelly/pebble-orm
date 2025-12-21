package models

import (
	"time"

	"github.com/marshallshelly/pebble-orm/pkg/schema"
)

// Models demonstrating PostgreSQL-specific features

// table_name: documents
type Document struct {
	ID        int          `po:"id,primaryKey,serial"`
	Title     string       `po:"title,varchar(255),notNull"`
	Content   string       `po:"content,text,notNull"`
	Metadata  schema.JSONB `po:"metadata,jsonb"`      // JSONB field
	Tags      []string     `po:"tags,text[]"`         // Array field
	SearchVec string       `po:"search_vec,tsvector"` // Full-text search
	CreatedAt time.Time    `po:"created_at,timestamptz,default(NOW()),notNull"`
}

// table_name: locations
type Location struct {
	ID     int    `po:"id,primaryKey,serial"`
	Name   string `po:"name,varchar(100),notNull"`
	Coords string `po:"coords,point"` // Geometric type
}

// table_name: products
type Product struct {
	ID     int    `po:"id,primaryKey,serial"`
	Name   string `po:"name,varchar(100),notNull"`
	Prices []int  `po:"prices,integer[]"` // Integer array
	Active bool   `po:"active,boolean,default(true),notNull"`
}
