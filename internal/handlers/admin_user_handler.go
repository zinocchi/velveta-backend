package handlers

import (
	"encoding/json"
	"net/http"
	"velveta-backend/internal/config"
	"velveta-backend/internal/middleware"
	"velveta-backend/internal/models"
	"velveta-backend/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AdminCreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Role     string `json:"role" validate:"required,oneof=admin cashier user"`
}

type AdminUpdateUserRequest struct {
	Name     string `json:"name" validate:"omitempty,min=2,max=100"`
	Role     string `json:"role" validate:"omitempty,oneof=admin cashier user"`
	IsActive *bool  `json:"is_active"`
	Password string `json:"password" validate:"omitempty,min=6"`
}

func AdminGetUsers(w http.ResponseWriter, r *http.Request) {
	roleFilter := r.URL.Query().Get("role")
	query := config.DB.Order("created_at DESC")

	if roleFilter != "" {
		query = query.Where("role = ?", roleFilter)
	}

	var users []models.User
	if err := query.Find(&users).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal mengambil data pengguna", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Daftar pengguna berhasil diambil", users)
}

func AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}

	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Validasi gagal", err.Error())
		return
	}

	var existing models.User
	if err := config.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		utils.Error(w, http.StatusConflict, "Email sudah digunakan", nil)
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	newUser := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hashed),
		Role:         req.Role,
		IsActive:     true,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal membuat akun pengguna", nil)
		return
	}

	utils.JSON(w, http.StatusCreated, true, "Akun pengguna berhasil dibuat", newUser)
}

func AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID pengguna tidak valid", nil)
		return
	}

	var user models.User
	if err := config.DB.First(&user, "id = ?", targetID).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Pengguna tidak ditemukan", nil)
		return
	}

	var req AdminUpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}

	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Validasi gagal", err.Error())
		return
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Password != "" {
		hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		user.PasswordHash = string(hashed)
	}

	if err := config.DB.Save(&user).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal memperbarui data pengguna", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Data pengguna berhasil diperbarui", user)
}

func AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	currentAdminID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID pengguna tidak valid", nil)
		return
	}

	if currentAdminID == targetID {
		utils.Error(w, http.StatusBadRequest, "Anda tidak dapat menghapus akun Anda sendiri", nil)
		return
	}

	if err := config.DB.Delete(&models.User{}, "id = ?", targetID).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal menghapus pengguna", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Pengguna berhasil dihapus", nil)
}
