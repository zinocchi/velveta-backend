package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"velveta-backend/internal/config"
	"velveta-backend/internal/models"
	"velveta-backend/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func slugify(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ==================== PUBLIC MENU & CATEGORY ====================

func GetCategories(w http.ResponseWriter, r *http.Request) {
	var categories []models.Category
	if err := config.DB.Order("id ASC").Find(&categories).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal mengambil data kategori", nil)
		return
	}
	utils.JSON(w, http.StatusOK, true, "Daftar kategori berhasil diambil", categories)
}

func GetMenus(w http.ResponseWriter, r *http.Request) {
	categorySlug := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")
	onlyAvailable := r.URL.Query().Get("available")

	query := config.DB.Preload("Category").Order("created_at DESC")

	if categorySlug != "" {
		query = query.Joins("JOIN categories ON categories.id = menus.category_id").
			Where("categories.slug = ?", categorySlug)
	}
	if search != "" {
		query = query.Where("menus.name ILIKE ?", "%"+search+"%")
	}
	if onlyAvailable == "true" {
		query = query.Where("menus.is_available = ?", true)
	}

	var menus []models.Menu
	if err := query.Find(&menus).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal mengambil daftar menu", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Daftar menu berhasil diambil", menus)
}

func GetMenusByCategorySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var category models.Category
	if err := config.DB.Where("slug = ?", slug).First(&category).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Kategori tidak ditemukan", nil)
		return
	}

	var menus []models.Menu
	config.DB.Preload("Category").Where("category_id = ?", category.ID).Find(&menus)

	utils.JSON(w, http.StatusOK, true, "Menu berdasarkan kategori berhasil diambil", map[string]interface{}{
		"category": category,
		"menus":    menus,
	})
}

func GetMenuByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID menu tidak valid", nil)
		return
	}

	var menu models.Menu
	if err := config.DB.Preload("Category").First(&menu, "id = ?", id).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Menu tidak ditemukan", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Detail menu berhasil diambil", menu)
}

// ==================== ADMIN CATEGORY CRUD ====================

type CategoryRequest struct {
	Name string `json:"name" validate:"required,min=2,max=50"`
}

func AdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req CategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}
	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Validasi gagal", err.Error())
		return
	}

	category := models.Category{
		Name: req.Name,
		Slug: slugify(req.Name),
	}
	if err := config.DB.Create(&category).Error; err != nil {
		utils.Error(w, http.StatusConflict, "Kategori sudah ada atau gagal disimpan", nil)
		return
	}

	utils.JSON(w, http.StatusCreated, true, "Kategori berhasil dibuat", category)
}

func AdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := config.DB.Delete(&models.Category{}, id).Error; err != nil {
		utils.Error(w, http.StatusBadRequest, "Gagal menghapus kategori (pastikan tidak ada menu yang terhubung)", nil)
		return
	}
	utils.JSON(w, http.StatusOK, true, "Kategori berhasil dihapus", nil)
}

// ==================== ADMIN MENU & STOCK MANAGEMENT ====================

func AdminCreateMenu(w http.ResponseWriter, r *http.Request) {
	var menu models.Menu
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil { // max 10MB
			utils.Error(w, http.StatusBadRequest, "Ukuran file terlalu besar (maks 10MB)", nil)
			return
		}

		catID, _ := strconv.Atoi(r.FormValue("category_id"))
		price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
		stock, _ := strconv.Atoi(r.FormValue("stock"))
		minStock, err := strconv.Atoi(r.FormValue("min_stock"))
		if err != nil || minStock <= 0 {
			minStock = 5
		}

		menu = models.Menu{
			CategoryID:  uint(catID),
			Name:        r.FormValue("name"),
			Description: r.FormValue("description"),
			Price:       price,
			Stock:       stock,
			MinStock:    minStock,
			IsAvailable: r.FormValue("is_available") != "false",
		}

		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			imageURL, uploadErr := utils.UploadToSupabaseStorage(file, header)
			if uploadErr != nil {
				utils.Error(w, http.StatusInternalServerError, "Gagal upload gambar menu: "+uploadErr.Error(), nil)
				return
			}
			menu.ImageURL = imageURL
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
			utils.Error(w, http.StatusBadRequest, "Format request tidak valid", nil)
			return
		}
		if menu.MinStock <= 0 {
			menu.MinStock = 5
		}
	}

	if menu.Name == "" || menu.CategoryID == 0 || menu.Price <= 0 {
		utils.Error(w, http.StatusBadRequest, "Nama, kategori, dan harga menu wajib diisi", nil)
		return
	}

	if err := config.DB.Create(&menu).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal menyimpan menu", nil)
		return
	}

	config.DB.Preload("Category").First(&menu, "id = ?", menu.ID)
	utils.JSON(w, http.StatusCreated, true, "Menu berhasil ditambahkan", menu)
}

func AdminUpdateMenu(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID menu tidak valid", nil)
		return
	}

	var menu models.Menu
	if err := config.DB.First(&menu, "id = ?", id).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Menu tidak ditemukan", nil)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(10 << 20)

		if val := r.FormValue("name"); val != "" {
			menu.Name = val
		}
		if val := r.FormValue("description"); val != "" {
			menu.Description = val
		}
		if val := r.FormValue("category_id"); val != "" {
			catID, _ := strconv.Atoi(val)
			menu.CategoryID = uint(catID)
		}
		if val := r.FormValue("price"); val != "" {
			price, _ := strconv.ParseFloat(val, 64)
			menu.Price = price
		}
		if val := r.FormValue("stock"); val != "" {
			stock, _ := strconv.Atoi(val)
			menu.Stock = stock
		}
		if val := r.FormValue("min_stock"); val != "" {
			minStock, _ := strconv.Atoi(val)
			menu.MinStock = minStock
		}
		if val := r.FormValue("is_available"); val != "" {
			menu.IsAvailable = val == "true"
		}

		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			imageURL, uploadErr := utils.UploadToSupabaseStorage(file, header)
			if uploadErr != nil {
				utils.Error(w, http.StatusInternalServerError, "Gagal upload gambar baru: "+uploadErr.Error(), nil)
				return
			}
			menu.ImageURL = imageURL
		}
	} else {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
			return
		}
		config.DB.Model(&menu).Updates(payload)
	}

	config.DB.Save(&menu)
	config.DB.Preload("Category").First(&menu, "id = ?", menu.ID)
	utils.JSON(w, http.StatusOK, true, "Menu berhasil diperbarui", menu)
}

func AdminDeleteMenu(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID menu tidak valid", nil)
		return
	}

	if err := config.DB.Delete(&models.Menu{}, "id = ?", id).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal menghapus menu", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Menu berhasil dihapus", nil)
}

func AdminUpdateStock(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID menu tidak valid", nil)
		return
	}

	var req struct {
		Stock int `json:"stock" validate:"min=0"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}

	var menu models.Menu
	if err := config.DB.First(&menu, "id = ?", id).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Menu tidak ditemukan", nil)
		return
	}

	menu.Stock = req.Stock
	config.DB.Save(&menu)
	utils.JSON(w, http.StatusOK, true, "Stok menu berhasil diperbarui", menu)
}

func AdminBulkUpdateStock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			MenuID uuid.UUID `json:"menu_id" validate:"required"`
			Stock  int       `json:"stock" validate:"min=0"`
		} `json:"items" validate:"required,min=1"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range req.Items {
			if err := tx.Model(&models.Menu{}).Where("id = ?", item.MenuID).Update("stock", item.Stock).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal memperbarui stok secara massal", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Stok massal berhasil diperbarui", nil)
}

func AdminGetLowStockItems(w http.ResponseWriter, r *http.Request) {
	var menus []models.Menu
	if err := config.DB.Preload("Category").Where("stock <= min_stock").Order("stock ASC").Find(&menus).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal mengambil data stok menipis", nil)
		return
	}
	utils.JSON(w, http.StatusOK, true, "Daftar menu dengan stok menipis", menus)
}

func AdminToggleMenuAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID menu tidak valid", nil)
		return
	}

	var menu models.Menu
	if err := config.DB.First(&menu, "id = ?", id).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Menu tidak ditemukan", nil)
		return
	}

	menu.IsAvailable = !menu.IsAvailable
	config.DB.Save(&menu)
	utils.JSON(w, http.StatusOK, true, "Status ketersediaan menu diubah", menu)
}