package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipDatabaseMigrationDoesNotCreateTables(t *testing.T) {
	oldDB, oldLogDB := DB, LOG_DB
	oldPath, oldMaster := common.SQLitePath, common.IsMasterNode
	oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.SQLitePath, common.IsMasterNode = oldPath, oldMaster
		common.SetMainDatabaseType(oldMainType)
		common.SetLogDatabaseType(oldLogType)
		initCol()
	})
	t.Setenv("SKIP_DATABASE_MIGRATION", "true")
	t.Setenv("SQL_DSN", "local")
	t.Setenv("LOG_SQL_DSN", "local")
	common.IsMasterNode = true
	common.SQLitePath = filepath.Join(t.TempDir(), "main.db")
	require.NoError(t, InitDB())
	mainSQL, err := DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, mainSQL.Close()) })
	var tables int64
	require.NoError(t, DB.Table("sqlite_master").Where("type = ?", "table").Count(&tables).Error)
	assert.Zero(t, tables)

	common.SQLitePath = filepath.Join(t.TempDir(), "logs.db")
	require.NoError(t, InitLogDB())
	logSQL, err := LOG_DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, logSQL.Close()) })
	require.NoError(t, LOG_DB.Table("sqlite_master").Where("type = ?", "table").Count(&tables).Error)
	assert.Zero(t, tables)
}
