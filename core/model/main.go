package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/env"
	// import fastjson serializer
	_ "github.com/labring/aiproxy/core/common/fastJSONSerializer"
	"github.com/labring/aiproxy/core/common/notify"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var (
	DB    *gorm.DB
	LogDB *gorm.DB
)

func chooseDB(envName string) (*gorm.DB, error) {
	dsn := os.Getenv(envName)

	switch {
	case strings.HasPrefix(dsn, "postgres"):
		// Use PostgreSQL
		log.Info("using PostgreSQL as database")

		return OpenPostgreSQL(dsn)
	default:
		// Use SQLite
		absPath, err := filepath.Abs(common.SQLitePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path of SQLite database: %w", err)
		}

		log.Info("SQL_DSN not set, using SQLite as database: ", absPath)

		common.UsingSQLite = true

		return OpenSQLite(absPath)
	}
}

func newDBLogger() gormLogger.Interface {
	var logLevel gormLogger.LogLevel
	if config.DebugSQLEnabled {
		logLevel = gormLogger.Info
	} else {
		logLevel = gormLogger.Warn
	}

	return gormLogger.New(
		log.StandardLogger(),
		gormLogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      !config.DebugSQLEnabled,
			Colorful:                  common.NeedColor(),
		},
	)
}

func OpenPostgreSQL(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		PrepareStmt:                              true, // precompile SQL
		TranslateError:                           true,
		Logger:                                   newDBLogger(),
		DisableForeignKeyConstraintWhenMigrating: false,
		IgnoreRelationshipsWhenMigrating:         false,
	})
}

func OpenMySQL(dsn string) (*gorm.DB, error) {
	return gorm.Open(mysql.New(mysql.Config{
		DSN: strings.TrimPrefix(dsn, "mysql://"),
	}), &gorm.Config{
		PrepareStmt:                              true, // precompile SQL
		TranslateError:                           true,
		Logger:                                   newDBLogger(),
		DisableForeignKeyConstraintWhenMigrating: false,
		IgnoreRelationshipsWhenMigrating:         false,
	})
}

func OpenSQLite(sqlitePath string) (*gorm.DB, error) {
	baseDir := filepath.Dir(sqlitePath)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	dsn := fmt.Sprintf("%s?_busy_timeout=%d", sqlitePath, common.SQLiteBusyTimeout)

	return gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt:                              true, // precompile SQL
		TranslateError:                           true,
		Logger:                                   newDBLogger(),
		DisableForeignKeyConstraintWhenMigrating: false,
		IgnoreRelationshipsWhenMigrating:         false,
	})
}

func InitDB() error {
	var err error

	DB, err = chooseDB("SQL_DSN")
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	setDBConns(DB)

	if config.DisableAutoMigrateDB {
		return nil
	}

	log.Info("database migration started")

	if err = migrateDB(); err != nil {
		log.Fatal("failed to migrate database: " + err.Error())
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Info("database migrated")

	return nil
}

func migrateDB() error {
	err := DB.AutoMigrate(
		&Channel{},
		&ChannelTest{},
		&Token{},
		&PublicMCP{},
		&GroupModelConfig{},
		&PublicMCPReusingParam{},
		&GroupMCP{},
		&Group{},
		&Option{},
		&ModelConfig{},
	)
	if err != nil {
		return err
	}

	return nil
}

func InitLogDB(batchSize int) error {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LogDB = DB
	} else {
		log.Info("using log database for table logs")

		var err error

		LogDB, err = chooseDB("LOG_SQL_DSN")
		if err != nil {
			return fmt.Errorf("failed to initialize log database: %w", err)
		}

		setDBConns(LogDB)
	}

	if config.DisableAutoMigrateDB {
		return nil
	}

	log.Info("log database migration started")

	err := migrateLOGDB(batchSize)
	if err != nil {
		return fmt.Errorf("failed to migrate log database: %w", err)
	}

	log.Info("log database migrated")

	return nil
}

func migrateLOGDB(batchSize int) error {
	// Pre-migration cleanup to remove expired data
	err := preMigrationCleanup(batchSize)
	if err != nil {
		log.Warn("failed to perform pre-migration cleanup: ", err.Error())
	}

	err = LogDB.AutoMigrate(
		&Log{},
		&RequestDetail{},
		&RetryLog{},
		&GroupSummary{},
		&Summary{},
		&ConsumeError{},
		&StoreV2{},
		&SummaryMinute{},
		&GroupSummaryMinute{},
	)
	if err != nil {
		return err
	}

	go func() {
		err := CreateLogIndexes(LogDB)
		if err != nil {
			notify.ErrorThrottle(
				"createLogIndexes",
				time.Minute*10,
				"failed to create log indexes",
				err.Error(),
			)
		}

		err = CreateSummaryIndexs(LogDB)
		if err != nil {
			notify.ErrorThrottle(
				"createSummaryIndexs",
				time.Minute*10,
				"failed to create summary indexs",
				err.Error(),
			)
		}

		err = CreateGroupSummaryIndexs(LogDB)
		if err != nil {
			notify.ErrorThrottle(
				"createGroupSummaryIndexs",
				time.Minute*10,
				"failed to create group summary indexs",
				err.Error(),
			)
		}

		err = CreateSummaryMinuteIndexs(LogDB)
		if err != nil {
			notify.ErrorThrottle(
				"createSummaryMinuteIndexs",
				time.Minute*10,
				"failed to create summary minute indexs",
				err.Error(),
			)
		}

		err = CreateGroupSummaryMinuteIndexs(LogDB)
		if err != nil {
			notify.ErrorThrottle(
				"createSummaryMinuteIndexs",
				time.Minute*10,
				"failed to create group summary minute indexs",
				err.Error(),
			)
		}
	}()

	return nil
}

func setDBConns(db *gorm.DB) {
	if config.DebugSQLEnabled {
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("failed to connect database: " + err.Error())
		return
	}

	sqlDB.SetMaxIdleConns(int(env.Int64("SQL_MAX_IDLE_CONNS", 100)))
	sqlDB.SetMaxOpenConns(int(env.Int64("SQL_MAX_OPEN_CONNS", 1000)))
	sqlDB.SetConnMaxLifetime(time.Second * time.Duration(env.Int64("SQL_MAX_LIFETIME", 60)))
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	err = sqlDB.Close()

	return err
}

func CloseDB() error {
	if LogDB != DB {
		err := closeDB(LogDB)
		if err != nil {
			return err
		}
	}

	return closeDB(DB)
}

func ignoreNoSuchTable(err error) bool {
	message := err.Error()
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist")
}

// preMigrationCleanup cleans up expired logs and request details before migration
// to reduce database size and improve migration performance
func preMigrationCleanup(batchSize int) error {
	log.Info("starting pre-migration cleanup of expired data")

	// Clean up logs
	err := preMigrationCleanupLogs(batchSize)
	if err != nil {
		if ignoreNoSuchTable(err) {
			return nil
		}
		return fmt.Errorf("failed to cleanup logs: %w", err)
	}

	// Clean up retry logs
	err = preMigrationCleanupRetryLogs(batchSize)
	if err != nil {
		if ignoreNoSuchTable(err) {
			return nil
		}
		return fmt.Errorf("failed to cleanup retry logs: %w", err)
	}

	// Clean up request details
	err = preMigrationCleanupRequestDetails(batchSize)
	if err != nil {
		if ignoreNoSuchTable(err) {
			return nil
		}
		return fmt.Errorf("failed to cleanup request details: %w", err)
	}

	log.Info("pre-migration cleanup completed")

	return nil
}

// preMigrationCleanupLogs cleans up expired logs using ID-based batch deletion
func preMigrationCleanupLogs(batchSize int) error {
	logStorageHours := config.GetLogStorageHours()
	if logStorageHours == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	cutoffTime := time.Now().Add(-time.Duration(logStorageHours) * time.Hour)

	// First, get the IDs to delete
	ids := make([]int, 0, batchSize)

	for {
		ids = ids[:0]

		err := LogDB.Model(&Log{}).
			Select("id").
			Where("created_at < ?", cutoffTime).
			Limit(batchSize).
			Find(&ids).Error
		if err != nil {
			return err
		}

		// If no IDs found, we're done
		if len(ids) == 0 {
			break
		}

		// Delete by IDs
		err = LogDB.Where("id IN (?)", ids).
			Session(&gorm.Session{SkipDefaultTransaction: true}).
			Delete(&Log{}).Error
		if err != nil {
			return err
		}

		log.Infof("deleted %d expired log records", len(ids))

		// If we got less than batchSize, we're done
		if len(ids) < batchSize {
			break
		}
	}

	return nil
}

// preMigrationCleanupRetryLogs cleans up expired logs using ID-based batch deletion
func preMigrationCleanupRetryLogs(batchSize int) error {
	logStorageHours := config.GetRetryLogStorageHours()
	if logStorageHours == 0 {
		logStorageHours = config.GetLogStorageHours()
	}

	if logStorageHours == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	cutoffTime := time.Now().Add(-time.Duration(logStorageHours) * time.Hour)

	// First, get the IDs to delete
	ids := make([]int, 0, batchSize)

	for {
		ids = ids[:0]

		err := LogDB.Model(&RetryLog{}).
			Select("id").
			Where("created_at < ?", cutoffTime).
			Limit(batchSize).
			Find(&ids).Error
		if err != nil {
			return err
		}

		// If no IDs found, we're done
		if len(ids) == 0 {
			break
		}

		// Delete by IDs
		err = LogDB.Where("id IN (?)", ids).
			Session(&gorm.Session{SkipDefaultTransaction: true}).
			Delete(&Log{}).Error
		if err != nil {
			return err
		}

		log.Infof("deleted %d expired retry log records", len(ids))

		// If we got less than batchSize, we're done
		if len(ids) < batchSize {
			break
		}
	}

	return nil
}

// preMigrationCleanupRequestDetails cleans up expired request details using ID-based batch deletion
func preMigrationCleanupRequestDetails(batchSize int) error {
	detailStorageHours := config.GetLogDetailStorageHours()
	if detailStorageHours == 0 {
		detailStorageHours = config.GetLogStorageHours()
	}

	if detailStorageHours == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = defaultCleanLogBatchSize
	}

	cutoffTime := time.Now().Add(-time.Duration(detailStorageHours) * time.Hour)

	// First, get the IDs to delete
	ids := make([]int, 0, batchSize)

	for {
		ids = ids[:0]

		err := LogDB.Model(&RequestDetail{}).
			Select("id").
			Where("created_at < ?", cutoffTime).
			Limit(batchSize).
			Find(&ids).Error
		if err != nil {
			return err
		}

		// If no IDs found, we're done
		if len(ids) == 0 {
			break
		}

		// Delete by IDs
		err = LogDB.Where("id IN (?)", ids).
			Session(&gorm.Session{SkipDefaultTransaction: true}).
			Delete(&RequestDetail{}).Error
		if err != nil {
			return err
		}

		log.Infof("deleted %d expired request detail records", len(ids))

		// If we got less than batchSize, we're done
		if len(ids) < batchSize {
			break
		}
	}

	return nil
}
