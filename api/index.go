package handler

import (
	"net/http"
	"velveta-backend/internal/config"
	"velveta-backend/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

var router *chi.Mux

func init() {
	// Inisialisasi DB & AutoMigrate
	config.InitDB()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Konfigurasi CORS agar frontend React di Vercel bisa akses API ini
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000", "https://*.vercel.app"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		utils.JSON(w, http.StatusOK, true, "Velveta POS Go API is running!", map[string]string{
			"status":   "healthy",
			"database": "connected",
		})
	})

	router = r
}

// Handler adalah fungsi wajib yang dipanggil oleh Vercel Serverless
func Handler(w http.ResponseWriter, r *http.Request) {
	router.ServeHTTP(w, r)
}