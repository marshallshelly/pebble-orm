package models

// table_name: accounts
type Account struct {
	ID      int     `po:"id,primaryKey,serial"`
	UserID  int     `po:"user_id,integer,notNull"`
	Balance float64 `po:"balance,numeric(12,2),notNull,default(0)"`
}

// table_name: transactions
type Transaction struct {
	ID            int     `po:"id,primaryKey,serial"`
	FromAccountID int     `po:"from_account_id,integer,notNull"`
	ToAccountID   int     `po:"to_account_id,integer,notNull"`
	Amount        float64 `po:"amount,numeric(12,2),notNull"`
	Description   string  `po:"description,text"`
}
