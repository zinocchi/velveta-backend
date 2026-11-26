package handler

import (
	"net/http"
	"velveta-backend/internal/config"
	"velveta-backend/internal/handlers"
	customMiddleware "velveta-backend/internal/middleware"
	"velveta-backend/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

var router *chi.Mux

func init() {
	config.InitDB()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:3000", "https://*.vercel.app"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			utils.JSON(w, http.StatusOK, true, "Velveta POS Go API is running!", nil)
		})

		// Public Auth
		r.Post("/auth/register", handlers.Register)
		r.Post("/auth/login", handlers.Login)

		// Public Menu & Categories
		r.Get("/categories", handlers.GetCategories)
		r.Get("/menus", handlers.GetMenus)
		r.Get("/menus/category/{slug}", handlers.GetMenusByCategorySlug)
		r.Get("/menus/{id}", handlers.GetMenuByID)

		// Protected Routes
		r.Group(func(r chi.Router) {
			r.Use(customMiddleware.AuthMiddleware)
			r.Get("/auth/me", handlers.GetMe)

			// Admin Only Routes
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RequireRole("admin"))

				// User Management (Admin, Cashier, User)
				r.Get("/admin/users", handlers.AdminGetUsers)
				r.Post("/admin/users", handlers.AdminCreateUser)
				r.Put("/admin/users/{id}", handlers.AdminUpdateUser)
				r.Delete("/admin/users/{id}", handlers.AdminDeleteUser)

				// Category Management
				r.Post("/admin/categories", handlers.AdminCreateCategory)
				r.Delete("/admin/categories/{id}", handlers.AdminDeleteCategory)

				// Menu & Stock Management
				r.Post("/admin/menus", handlers.AdminCreateMenu)
				r.Put("/admin/menus/{id}", handlers.AdminUpdateMenu)
				r.Delete("/admin/menus/{id}", handlers.AdminDeleteMenu)
				r.Put("/admin/menus/{id}/stock", handlers.AdminUpdateStock)
				r.Post("/admin/menus/bulk-update-stock", handlers.AdminBulkUpdateStock)
				r.Get("/admin/stock/low-stock", handlers.AdminGetLowStockItems)
				r.Patch("/admin/menus/{id}/toggle-availability", handlers.AdminToggleMenuAvailability)
			})
		})
	})

	router = r
}

func Handler(w http.ResponseWriter, r *http.Request) {
	router.ServeHTTP(w, r)
}