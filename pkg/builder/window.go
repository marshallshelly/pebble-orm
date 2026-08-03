package builder

import (
	"fmt"
	"strings"
)

// WindowExpr builds a PostgreSQL window-function expression for use inside a
// SELECT column list (Columns). A window function computes a value per row over
// a set of related rows — the "window" — defined by PARTITION BY, ORDER BY and
// an optional frame clause. For example:
//
//	users, err := builder.Select[Sale](qb).
//	    Columns("product_id", "amount",
//	        builder.Window("SUM(amount)").
//	            PartitionBy("product_id").
//	            OrderByAsc("sale_date").
//	            As("running_total")).
//	    All(ctx)
//
// produces:
//
//	SELECT product_id, amount,
//	       SUM(amount) OVER (PARTITION BY product_id ORDER BY sale_date ASC) AS running_total
//	FROM sales
//
// The aliased result scans into the matching struct field (e.g. a
// `RunningTotal int64 ` + "`po:\"running_total\"`" + ` field), exactly like any
// other selected column.
//
// Window expressions are assembled from developer-supplied function and column
// names — the same trust model as Columns — and carry no bound parameters, so
// they must never be built from untrusted input.
type WindowExpr struct {
	fn          string
	partitionBy []string
	orderBy     []OrderBy
	frame       string
}

// Window creates a window expression from a function call such as
// "SUM(amount)", "ROW_NUMBER()" or "LAG(price, 1)". Prefer the named
// constructors (RowNumber, SumOver, Lag, …) for the common functions.
func Window(fn string) *WindowExpr {
	return &WindowExpr{fn: fn}
}

// PartitionBy adds columns to the window's PARTITION BY clause.
func (w *WindowExpr) PartitionBy(columns ...string) *WindowExpr {
	w.partitionBy = append(w.partitionBy, columns...)
	return w
}

// OrderBy adds a column to the window's ORDER BY clause.
func (w *WindowExpr) OrderBy(column string, direction OrderDirection) *WindowExpr {
	w.orderBy = append(w.orderBy, OrderBy{Column: column, Direction: direction, NullsPos: NullsDefault})
	return w
}

// OrderByAsc adds an ascending column to the window's ORDER BY clause.
func (w *WindowExpr) OrderByAsc(column string) *WindowExpr {
	return w.OrderBy(column, Asc)
}

// OrderByDesc adds a descending column to the window's ORDER BY clause.
func (w *WindowExpr) OrderByDesc(column string) *WindowExpr {
	return w.OrderBy(column, Desc)
}

// Frame sets an explicit frame clause, e.g.
// "ROWS BETWEEN 2 PRECEDING AND CURRENT ROW" for a moving average.
func (w *WindowExpr) Frame(spec string) *WindowExpr {
	w.frame = spec
	return w
}

// String renders the window expression without an alias:
// "fn OVER (PARTITION BY ... ORDER BY ... frame)".
func (w *WindowExpr) String() string {
	var parts []string
	if len(w.partitionBy) > 0 {
		parts = append(parts, "PARTITION BY "+strings.Join(w.partitionBy, ", "))
	}
	if len(w.orderBy) > 0 {
		ob := make([]string, len(w.orderBy))
		for i, o := range w.orderBy {
			ob[i] = o.Column + " " + string(o.Direction)
			if o.NullsPos != NullsDefault {
				ob[i] += " " + string(o.NullsPos)
			}
		}
		parts = append(parts, "ORDER BY "+strings.Join(ob, ", "))
	}
	if w.frame != "" {
		parts = append(parts, w.frame)
	}
	return w.fn + " OVER (" + strings.Join(parts, " ") + ")"
}

// As renders the window expression with a column alias, ready to drop into
// Columns(): "fn OVER (...) AS alias".
func (w *WindowExpr) As(alias string) string {
	return w.String() + " AS " + alias
}

// Ranking window functions.

// RowNumber is ROW_NUMBER() — a unique sequential number per partition.
func RowNumber() *WindowExpr { return Window("ROW_NUMBER()") }

// Rank is RANK() — rank with gaps after ties.
func Rank() *WindowExpr { return Window("RANK()") }

// DenseRank is DENSE_RANK() — rank without gaps after ties.
func DenseRank() *WindowExpr { return Window("DENSE_RANK()") }

// PercentRank is PERCENT_RANK() — relative rank in [0, 1].
func PercentRank() *WindowExpr { return Window("PERCENT_RANK()") }

// CumeDist is CUME_DIST() — cumulative distribution.
func CumeDist() *WindowExpr { return Window("CUME_DIST()") }

// Ntile is NTILE(buckets) — divides the partition into the given number of ranked buckets.
func Ntile(buckets int) *WindowExpr { return Window(fmt.Sprintf("NTILE(%d)", buckets)) }

// Offset and value window functions.

// Lag is LAG(column, offset) — a value from a preceding row in the partition.
func Lag(column string, offset int) *WindowExpr {
	return Window(fmt.Sprintf("LAG(%s, %d)", column, offset))
}

// Lead is LEAD(column, offset) — a value from a following row in the partition.
func Lead(column string, offset int) *WindowExpr {
	return Window(fmt.Sprintf("LEAD(%s, %d)", column, offset))
}

// FirstValue is first_value(column) — the first value in the window frame.
func FirstValue(column string) *WindowExpr {
	return Window(fmt.Sprintf("first_value(%s)", column))
}

// LastValue is last_value(column) — the last value in the window frame.
func LastValue(column string) *WindowExpr {
	return Window(fmt.Sprintf("last_value(%s)", column))
}

// NthValue is nth_value(column, n) — the nth value in the window frame.
func NthValue(column string, n int) *WindowExpr {
	return Window(fmt.Sprintf("nth_value(%s, %d)", column, n))
}

// Aggregate window functions (the aggregate applied over the window).

// SumOver is SUM(column) as a window function.
func SumOver(column string) *WindowExpr { return Window(fmt.Sprintf("SUM(%s)", column)) }

// AvgOver is AVG(column) as a window function.
func AvgOver(column string) *WindowExpr { return Window(fmt.Sprintf("AVG(%s)", column)) }

// CountOver is COUNT(column) as a window function.
func CountOver(column string) *WindowExpr { return Window(fmt.Sprintf("COUNT(%s)", column)) }

// MinOver is MIN(column) as a window function.
func MinOver(column string) *WindowExpr { return Window(fmt.Sprintf("MIN(%s)", column)) }

// MaxOver is MAX(column) as a window function.
func MaxOver(column string) *WindowExpr { return Window(fmt.Sprintf("MAX(%s)", column)) }
