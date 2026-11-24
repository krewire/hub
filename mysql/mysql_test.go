package mysql_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/krewire/hub/mysql"
	"github.com/krewire/krewire/orm"
)

func TestDriverRegistration(t *testing.T) {
	var found bool
	for _, d := range sql.Drivers() {
		if d == mysql.DriverName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("driver %q was not registered in database/sql", mysql.DriverName)
	}
}

func TestConfigAndDSN(t *testing.T) {
	config := mysql.Config("db.production.local", 3307, "krewire_admin", "s3cret@p@ss", "krewire_core")
	dsn := mysql.DSN(config)

	expectedPrefix := "krewire_admin:s3cret@p@ss@tcp(db.production.local:3307)/krewire_core?"
	if !strings.HasPrefix(dsn, expectedPrefix) {
		t.Errorf("expected DSN prefix %s, got %s", expectedPrefix, dsn)
	}

	for _, token := range []string{
		"charset=utf8mb4",
		"collation=utf8mb4_unicode_ci",
		"parseTime=true",
		"loc=Local",
		"interpolateParams=true",
		"checkConnLiveness=true",
		"allowNativePasswords=true",
	} {
		if !strings.Contains(dsn, token) {
			t.Errorf("DSN missing expected token %q: %s", token, dsn)
		}
	}
}

func TestProductionConfig(t *testing.T) {
	prod := mysql.ProductionConfig("10.0.1.50", 0, "root", "root_pass", "orders_db")
	if prod.Port != 3306 {
		t.Errorf("expected default port 3306, got %d", prod.Port)
	}
	if prod.MaxOpenConns != 50 {
		t.Errorf("expected MaxOpenConns 50, got %d", prod.MaxOpenConns)
	}
	if prod.MaxIdleConns != 15 {
		t.Errorf("expected MaxIdleConns 15, got %d", prod.MaxIdleConns)
	}
	if prod.ConnMaxLifetime != 15*time.Minute {
		t.Errorf("expected ConnMaxLifetime 15m, got %v", prod.ConnMaxLifetime)
	}
	if !prod.InterpolateParams {
		t.Error("expected InterpolateParams true")
	}
}

func TestMySQLDialectCompatibility(t *testing.T) {
	dialect := orm.GetDialect(mysql.DriverName)
	if dialect.Name() != "mysql" {
		t.Errorf("expected dialect mysql, got %s", dialect.Name())
	}
	if quoted := dialect.QuoteIdentifier("users"); quoted != "`users`" {
		t.Errorf("expected `users`, got %s", quoted)
	}
	if ph := dialect.Placeholder(1); ph != "?" {
		t.Errorf("expected ?, got %s", ph)
	}
	if pk := dialect.AutoIncrementPrimaryKeyClause(); pk != "BIGINT AUTO_INCREMENT PRIMARY KEY" {
		t.Errorf("unexpected PK clause: %s", pk)
	}
}
