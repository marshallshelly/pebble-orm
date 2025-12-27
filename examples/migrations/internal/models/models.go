package models

// Simple models for migration demonstration

// table_name: products
type Product struct {
	ID          int    `po:"id,primaryKey,serial"`
	Name        string `po:"name,varchar(255),notNull"`
	Description string `po:"description,text"`
	Price       int    `po:"price,integer,notNull"`
	InStock     bool   `po:"in_stock,boolean,default(true),notNull"`
}

// table_name: categories
type Category struct {
	ID   int    `po:"id,primaryKey,serial"`
	Name string `po:"name,varchar(100),notNull,unique"`
}
