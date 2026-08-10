// Package gormx provides a thin helper to open GORM on an Irmik *db.Database.
// GORM is optional — the rest of Irmik does not require it.
package gormx

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/boracomet/go-irmik/irmik/db"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

// Open wraps an existing irmik/db.Database with GORM.
func Open(database *db.Database, opts ...gorm.Option) (*gorm.DB, error) {
	if database == nil || database.DB() == nil {
		return nil, fmt.Errorf("gormx: nil database")
	}
	return OpenSQL(database.DB(), database.Driver(), opts...)
}

// OpenSQL wraps a *sql.DB with the appropriate GORM dialector for driver.
// driver should be postgres | sqlite | mysql (aliases accepted).
func OpenSQL(sqlDB *sql.DB, driver string, opts ...gorm.Option) (*gorm.DB, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("gormx: nil *sql.DB")
	}
	dialector, err := dialectorFor(driver, sqlDB)
	if err != nil {
		return nil, err
	}
	return gorm.Open(dialector, opts...)
}

func dialectorFor(driver string, sqlDB *sql.DB) (gorm.Dialector, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql", "pgx":
		return postgres.New(postgres.Config{Conn: sqlDB}), nil
	case "sqlite", "sqlite3":
		// Conn-only dialector avoids importing a second sqlite driver
		// (modernc is already registered by irmik/db).
		return sqliteConnDialector{conn: sqlDB}, nil
	case "mysql", "mariadb":
		return mysql.New(mysql.Config{Conn: sqlDB}), nil
	default:
		return nil, fmt.Errorf("gormx: unsupported driver %q", driver)
	}
}

// sqliteConnDialector is a minimal GORM dialector over an existing *sql.DB.
type sqliteConnDialector struct {
	conn *sql.DB
}

func (d sqliteConnDialector) Name() string { return "sqlite" }

func (d sqliteConnDialector) Initialize(db *gorm.DB) error {
	db.ConnPool = d.conn
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
		LastInsertIDReversed: true,
	})
	return nil
}

func (d sqliteConnDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return migrator.Migrator{Config: migrator.Config{DB: db, Dialector: d}}
}

func (d sqliteConnDialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "boolean"
	case schema.Int, schema.Uint:
		return "integer"
	case schema.Float:
		return "real"
	case schema.String:
		return "text"
	case schema.Time:
		return "datetime"
	case schema.Bytes:
		return "blob"
	default:
		return string(field.DataType)
	}
}

func (d sqliteConnDialector) DefaultValueOf(field *schema.Field) clause.Expression {
	return clause.Expr{SQL: "NULL"}
}

func (d sqliteConnDialector) BindVarTo(writer clause.Writer, _ *gorm.Statement, _ interface{}) {
	_ = writer.WriteByte('?')
}

func (d sqliteConnDialector) QuoteTo(writer clause.Writer, str string) {
	_ = writer.WriteByte('`')
	_, _ = writer.WriteString(str)
	_ = writer.WriteByte('`')
}

func (d sqliteConnDialector) Explain(sql string, vars ...interface{}) string {
	return logger.ExplainSQL(sql, nil, `"`, vars...)
}
