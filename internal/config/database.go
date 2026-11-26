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
		// Load .env jika dijalankan di lokal (di Vercel env sudah otomatis ter-inject)
		_ = godotenv.Load()

		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("DATABASE_URL tidak ditemukan di environment variables")
		}

		db, err := gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, // Wajib untuk Supabase Transaction Pooler (port 6543)
		}), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err != nil {
			log.Fatalf("Gagal terhubung ke database Supabase: %v", err)
		}

		// Batasi pool koneksi per serverless instance
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetMaxOpenConns(5)
		}

		// Auto-migrate tabel sesuai ERD
		err = db.AutoMigrate(
			&models.User{},
			&models.WorkPin{},
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