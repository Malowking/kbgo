package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xwb1989/sqlparser"
)

var (
	ErrNotReadOnly    = errors.New("SQL语句不是只读操作")
	ErrInvalidSQL     = errors.New("无效的SQL语句")
	ErrUnsafeKeywords = errors.New("SQL包含危险关键字")
	ErrNoFromClause   = errors.New("缺少FROM子句")
)

// SQLValidator SQL校验器
type SQLValidator struct {
	allowedKeywords []string
	bannedKeywords  []string
}

// NewSQLValidator 创建SQL校验器
func NewSQLValidator() *SQLValidator {
	return &SQLValidator{
		allowedKeywords: []string{"SELECT", "FROM", "WHERE", "GROUP BY", "ORDER BY", "HAVING", "LIMIT", "OFFSET", "JOIN", "LEFT JOIN", "INNER JOIN", "RIGHT JOIN", "AS", "ON", "AND", "OR", "IN", "NOT", "BETWEEN", "LIKE", "IS", "NULL", "DISTINCT"},
		bannedKeywords:  []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE", "GRANT", "REVOKE", "EXEC", "EXECUTE", "CALL", "LOAD_FILE", "OUTFILE", "DUMPFILE"},
	}
}

// Validate 校验SQL（默认方法，调用ValidateReadOnly）
func (v *SQLValidator) Validate(sql string) error {
	return v.ValidateReadOnly(sql)
}

// ValidateReadOnly 校验SQL是否只读
func (v *SQLValidator) ValidateReadOnly(sql string) error {
	// 1. 检查危险关键字
	sqlUpper := strings.ToUpper(sql)
	for _, banned := range v.bannedKeywords {
		if strings.Contains(sqlUpper, banned) {
			return fmt.Errorf("%w: %s", ErrUnsafeKeywords, banned)
		}
	}

	// 2. 使用sqlparser解析
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSQL, err)
	}

	// 3. 检查是否是SELECT语句或UNION查询
	switch s := stmt.(type) {
	case *sqlparser.Select:
		// 单个SELECT语句
		// 检查是否有FROM子句
		if len(s.From) == 0 {
			return ErrNoFromClause
		}
		// 检查子查询
		return v.checkSubqueries(s)

	case *sqlparser.Union:
		// UNION查询 - 递归检查每个SELECT
		return v.checkUnion(s)

	default:
		return fmt.Errorf("%w: 只允许SELECT查询", ErrNotReadOnly)
	}
}

// checkSubqueries 递归检查子查询是否只读
func (v *SQLValidator) checkSubqueries(stmt *sqlparser.Select) error {
	// 检查FROM子句中的子查询
	for _, from := range stmt.From {
		if aliasedTable, ok := from.(*sqlparser.AliasedTableExpr); ok {
			if subquery, ok := aliasedTable.Expr.(*sqlparser.Subquery); ok {
				if err := v.checkSubquerySelect(subquery.Select); err != nil {
					return err
				}
			}
		}
	}

	// 检查WHERE子句中的子查询
	if stmt.Where != nil {
		if err := v.checkWhereSubqueries(stmt.Where.Expr); err != nil {
			return err
		}
	}

	return nil
}

// checkUnion 检查UNION查询
func (v *SQLValidator) checkUnion(union *sqlparser.Union) error {
	// 检查左侧（可能是SELECT或另一个UNION）
	if err := v.checkSelectStatement(union.Left); err != nil {
		return err
	}
	// 检查右侧（必须是SELECT）
	if err := v.checkSelectStatement(union.Right); err != nil {
		return err
	}
	return nil
}

// checkSelectStatement 检查SelectStatement（可能是Select或Union）
func (v *SQLValidator) checkSelectStatement(stmt sqlparser.SelectStatement) error {
	switch s := stmt.(type) {
	case *sqlparser.Select:
		// 检查是否有FROM子句
		if len(s.From) == 0 {
			return ErrNoFromClause
		}
		return v.checkSubqueries(s)
	case *sqlparser.Union:
		return v.checkUnion(s)
	default:
		return fmt.Errorf("%w: 只允许SELECT查询", ErrNotReadOnly)
	}
}

// checkSubquerySelect 检查子查询的SELECT语句
func (v *SQLValidator) checkSubquerySelect(sel sqlparser.SelectStatement) error {
	selectStmt, ok := sel.(*sqlparser.Select)
	if !ok {
		return fmt.Errorf("%w: 子查询只允许SELECT", ErrNotReadOnly)
	}
	return v.checkSubqueries(selectStmt)
}

// checkWhereSubqueries 检查WHERE子句中的子查询
func (v *SQLValidator) checkWhereSubqueries(expr sqlparser.Expr) error {
	switch e := expr.(type) {
	case *sqlparser.Subquery:
		return v.checkSubquerySelect(e.Select)
	case *sqlparser.ComparisonExpr:
		if err := v.checkWhereSubqueries(e.Left); err != nil {
			return err
		}
		if err := v.checkWhereSubqueries(e.Right); err != nil {
			return err
		}
	case *sqlparser.AndExpr:
		if err := v.checkWhereSubqueries(e.Left); err != nil {
			return err
		}
		if err := v.checkWhereSubqueries(e.Right); err != nil {
			return err
		}
	case *sqlparser.OrExpr:
		if err := v.checkWhereSubqueries(e.Left); err != nil {
			return err
		}
		if err := v.checkWhereSubqueries(e.Right); err != nil {
			return err
		}
	}
	return nil
}

// ParseSQL 解析SQL并返回AST
func (v *SQLValidator) ParseSQL(sql string) (*sqlparser.Select, error) {
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSQL, err)
	}

	selectStmt, ok := stmt.(*sqlparser.Select)
	if !ok {
		return nil, fmt.Errorf("%w: 只支持SELECT语句", ErrNotReadOnly)
	}

	return selectStmt, nil
}

// AddLimitIfMissing 如果SQL没有LIMIT子句，添加默认LIMIT
func (v *SQLValidator) AddLimitIfMissing(sql string, defaultLimit int) (string, error) {
	stmt, err := v.ParseSQL(sql)
	if err != nil {
		return "", err
	}

	// 检查是否已经有LIMIT
	if stmt.Limit == nil {
		stmt.Limit = &sqlparser.Limit{
			Rowcount: sqlparser.NewIntVal([]byte(fmt.Sprintf("%d", defaultLimit))),
		}
	}

	return sqlparser.String(stmt), nil
}

// ExtractTables 提取SQL中使用的所有表名
func (v *SQLValidator) ExtractTables(sql string) ([]string, error) {
	stmt, err := v.ParseSQL(sql)
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0)
	for _, from := range stmt.From {
		if aliasedTable, ok := from.(*sqlparser.AliasedTableExpr); ok {
			if tableName, ok := aliasedTable.Expr.(sqlparser.TableName); ok {
				tables = append(tables, tableName.Name.String())
			}
		}

		// 处理JOIN
		if joinTable, ok := from.(*sqlparser.JoinTableExpr); ok {
			leftTables := v.extractTablesFromTableExpr(joinTable.LeftExpr)
			rightTables := v.extractTablesFromTableExpr(joinTable.RightExpr)
			tables = append(tables, leftTables...)
			tables = append(tables, rightTables...)
		}
	}

	return tables, nil
}

// extractTablesFromTableExpr 从TableExpr提取表名
func (v *SQLValidator) extractTablesFromTableExpr(expr sqlparser.TableExpr) []string {
	tables := make([]string, 0)
	if aliasedTable, ok := expr.(*sqlparser.AliasedTableExpr); ok {
		if tableName, ok := aliasedTable.Expr.(sqlparser.TableName); ok {
			tables = append(tables, tableName.Name.String())
		}
	}
	return tables
}

// ExtractColumns 提取SQL中使用的所有列名
func (v *SQLValidator) ExtractColumns(sql string) ([]string, error) {
	stmt, err := v.ParseSQL(sql)
	if err != nil {
		return nil, err
	}

	columns := make([]string, 0)
	for _, selectExpr := range stmt.SelectExprs {
		switch expr := selectExpr.(type) {
		case *sqlparser.AliasedExpr:
			if colName, ok := expr.Expr.(*sqlparser.ColName); ok {
				columns = append(columns, colName.Name.String())
			}
		case *sqlparser.StarExpr:
			columns = append(columns, "*")
		}
	}

	return columns, nil
}
