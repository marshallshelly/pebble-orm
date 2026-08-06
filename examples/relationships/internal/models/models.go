package models

// table_name: authors
type Author struct {
	ID    int    `po:"id,primaryKey,serial"`
	Name  string `po:"name,varchar(100),notNull"`
	Books []Book `po:"-,hasMany,foreignKey(author_id),references(id)"`
}

// table_name: books
type Book struct {
	ID       int     `po:"id,primaryKey,serial"`
	Title    string  `po:"title,varchar(255),notNull"`
	ISBN     string  `po:"isbn,varchar(20),unique"`
	AuthorID int     `po:"author_id,integer,notNull"`
	Author   *Author `po:"-,belongsTo,foreignKey(author_id),references(id)"`
}

// table_name: users
type User struct {
	ID      int      `po:"id,primaryKey,serial"`
	Name    string   `po:"name,varchar(100),notNull"`
	Email   string   `po:"email,varchar(255),unique,notNull"`
	Profile *Profile `po:"-,hasOne,foreignKey(user_id),references(id)"`
	Roles   []Role   `po:"-,manyToMany,joinTable(user_roles),foreignKey(user_id),references(id)"`
}

// table_name: profiles
type Profile struct {
	ID     int    `po:"id,primaryKey,serial"`
	Bio    string `po:"bio,text"`
	Avatar string `po:"avatar,varchar(255)"`
	UserID int    `po:"user_id,integer,notNull,unique"`
}

// table_name: roles
type Role struct {
	ID   int    `po:"id,primaryKey,serial"`
	Name string `po:"name,varchar(50),notNull,unique"`
}

// UserRole is the manyToMany junction table for User ↔ Role. A composite UNIQUE
// on the two foreign keys stops the same role being assigned to a user twice.
//
// table_name: user_roles
// unique: uq_user_role (user_id, role_id)
type UserRole struct {
	ID     int `po:"id,primaryKey,serial"`
	UserID int `po:"user_id,integer,notNull,fk:users(id),onDelete:CASCADE"`
	RoleID int `po:"role_id,integer,notNull,fk:roles(id),onDelete:CASCADE"`
}
