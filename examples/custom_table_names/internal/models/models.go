package models

// table_name: custom_users_table
type User struct {
	ID    int    `po:"id,primaryKey,serial"`
	Name  string `po:"name,varchar(100),notNull"`
	Email string `po:"email,varchar(255),unique,notNull"`
}

// table_name: products_inventory
type Product struct {
	ID    int    `po:"id,primaryKey,serial"`
	Name  string `po:"name,varchar(200),notNull"`
	Price int    `po:"price,integer,notNull"`
	Stock int    `po:"stock,integer,default(0),notNull"`
}

// No custom table name - will use snake_case: "order"
type Order struct {
	ID        int `po:"id,primaryKey,serial"`
	UserID    int `po:"user_id,integer,notNull"`
	ProductID int `po:"product_id,integer,notNull"`
	Quantity  int `po:"quantity,integer,notNull"`
	Total     int `po:"total,integer,notNull"`

	// Relationships
	User    *User    `po:"-,belongsTo,foreignKey(user_id),references(id)"`
	Product *Product `po:"-,belongsTo,foreignKey(product_id),references(id)"`
}
