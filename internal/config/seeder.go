package config

import (
	"log"
	"velveta-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedInitialData(db *gorm.DB) {
	// 1. Seed Super Admin jika belum ada
	var adminCount int64
	db.Model(&models.User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := models.User{
			Name:         "Velveta Owner",
			Email:        "admin@velveta.com",
			PasswordHash: string(hashedPassword),
			Role:         "admin",
			IsActive:     true,
		}
		if err := db.Create(&admin).Error; err == nil {
			log.Println("Seeder: Akun Super Admin dibuat (admin@velveta.com / admin123)")
		}
	}

	// 2. Seed Kategori Awal jika masih kosong
	var catCount int64
	db.Model(&models.Category{}).Count(&catCount)
	if catCount == 0 {
		categories := []models.Category{
			{Name: "Espresso Based", Slug: "espresso-based"},
			{Name: "Manual Brew", Slug: "manual-brew"},
			{Name: "Non-Coffee", Slug: "non-coffee"},
			{Name: "Pastry & Bakery", Slug: "pastry-bakery"},
		}
		db.Create(&categories)
		log.Println("Seeder: Kategori menu awal berhasil dibuat")
	}
}