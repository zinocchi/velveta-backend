package config

import (
	"log"
	"os"
	"sync"
	"velveta-backend/internal/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB   *gorm.DB
	once sync.Once
)

func InitDB() *gorm.DB {
	once.Do(func() {
		_ = godotenv.Load()

		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("DATABASE_URL tidak ditemukan di environment variables")
		}

		db, err := gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, 
		}), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err != nil {
			log.Fatalf("Gagal terhubung ke database Supabase: %v", err)
		}

		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetMaxOpenConns(5)
		}

		err = db.AutoMigrate(
			&models.User{},
			&models.Category{},
			&models.Menu{},
			&models.Order{},
			&models.OrderItem{},
			&models.Payment{},
		)
		if err != nil {
			log.Fatalf("Gagal migrasi tabel: %v", err)
		}

		DB = db
		SeedInitialData(DB)
	})

	return DB
}