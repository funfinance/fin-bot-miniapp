package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	instance *gorm.DB
	once     sync.Once
)

// Init initializes database connection (Singleton pattern)
func Init(dbPath string) error {
	var err error
	once.Do(func() {
		// Create database directory if not exists
		dbDir := filepath.Dir(dbPath)
		if mkdirErr := os.MkdirAll(dbDir, 0755); mkdirErr != nil {
			err = fmt.Errorf("create database directory: %w", mkdirErr)
			return
		}

		instance, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		})
		if err != nil {
			err = fmt.Errorf("open database: %w", err)
			return
		}
	})

	return err
}

// Get returns the database instance
func Get() *gorm.DB {
	if instance == nil {
		panic("database not initialized, call Init() first")
	}
	return instance
}

// Close closes the database connection
func Close() error {
	if instance == nil {
		return nil
	}

	sqlDB, err := instance.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
