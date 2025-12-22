package models

import "time"

// table_name: people
type Person struct {
	ID        int64     `po:"id,primaryKey,autoIncrement"`
	FirstName string    `po:"first_name,varchar(100),notNull"`
	LastName  string    `po:"last_name,varchar(100),notNull"`
	FullName  string    `po:"full_name,varchar(255),generated:first_name || ' ' || last_name,stored"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
}

// table_name: measurements
type Measurement struct {
	ID        int64     `po:"id,primaryKey,autoIncrement"`
	Name      string    `po:"name,varchar(100),notNull"`
	HeightCm  float64   `po:"height_cm,numeric(10,2),notNull"`
	HeightIn  float64   `po:"height_in,numeric(10,2),generated:height_cm / 2.54,stored"`
	WeightKg  float64   `po:"weight_kg,numeric(10,2),notNull"`
	WeightLbs float64   `po:"weight_lbs,numeric(10,2),generated:weight_kg * 2.20462,stored"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
}

// table_name: products
type Product struct {
	ID        int64     `po:"id,primaryKey,autoIncrement"`
	Name      string    `po:"name,varchar(200),notNull"`
	ListPrice float64   `po:"list_price,numeric(10,2),notNull"`
	Tax       float64   `po:"tax,numeric(5,2),default(0),notNull"`      // Tax percentage
	Discount  float64   `po:"discount,numeric(5,2),default(0),notNull"` // Discount percentage
	NetPrice  float64   `po:"net_price,numeric(10,2),generated:(list_price + (list_price * tax / 100)) - (list_price * discount / 100),stored"`
	CreatedAt time.Time `po:"created_at,timestamptz,default(NOW()),notNull"`
}
