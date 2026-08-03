package builder

import (
	"testing"

	"github.com/marshallshelly/pebble-orm/pkg/registry"
)

func TestWindowExpr(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"running total", Window("SUM(amount)").PartitionBy("product_id").OrderByAsc("sale_date").As("running_total"),
			"SUM(amount) OVER (PARTITION BY product_id ORDER BY sale_date ASC) AS running_total"},
		{"row number desc", RowNumber().PartitionBy("dept").OrderByDesc("salary").As("rn"),
			"ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS rn"},
		{"dense rank multi partition", DenseRank().PartitionBy("a", "b").OrderByAsc("c").As("dr"),
			"DENSE_RANK() OVER (PARTITION BY a, b ORDER BY c ASC) AS dr"},
		{"moving avg with frame", AvgOver("amount").OrderByAsc("d").Frame("ROWS BETWEEN 2 PRECEDING AND CURRENT ROW").As("ma"),
			"AVG(amount) OVER (ORDER BY d ASC ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) AS ma"},
		{"lag", Lag("amount", 1).PartitionBy("p").OrderByAsc("d").As("prev"),
			"LAG(amount, 1) OVER (PARTITION BY p ORDER BY d ASC) AS prev"},
		{"ntile", Ntile(4).OrderByAsc("score").As("quartile"),
			"NTILE(4) OVER (ORDER BY score ASC) AS quartile"},
		{"no partition or order", CountOver("*").String(),
			"COUNT(*) OVER ()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expr != tt.want {
				t.Errorf("\n got  %q\n want %q", tt.expr, tt.want)
			}
		})
	}
}

func TestWindowInSelectToSQL(t *testing.T) {
	type Sale struct {
		ID        int   `po:"id,primaryKey,serial"`
		ProductID int   `po:"product_id,integer"`
		Amount    int64 `po:"amount,bigint"`
	}
	if err := registry.Register(Sale{}); err != nil {
		t.Fatal(err)
	}
	db := New(nil)
	sql, _, err := Select[Sale](db).
		Columns("product_id", "amount",
			SumOver("amount").PartitionBy("product_id").OrderByAsc("id").As("running_total")).
		ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT product_id, amount, SUM(amount) OVER (PARTITION BY product_id ORDER BY id ASC) AS running_total FROM sale"
	if sql != want {
		t.Errorf("\n got  %q\n want %q", sql, want)
	}
}
