package handlers

import (
	"encoding/json"
	"net/http"
	"velveta-backend/internal/config"
	"velveta-backend/internal/middleware"
	"velveta-backend/internal/models"
	"velveta-backend/internal/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
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
		utils.Error(w, http.StatusConflict, "Email sudah terdaftar", nil)
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hashed),
		Role:         "user",
		IsActive:     true,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal membuat akun", nil)
		return
	}

	token, _ := utils.GenerateToken(user.ID, user.Role)
	utils.JSON(w, http.StatusCreated, true, "Registrasi berhasil", map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}

	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Validasi gagal", err.Error())
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.Error(w, http.StatusUnauthorized, "Email atau password salah", nil)
		return
	}

	if !user.IsActive {
		utils.Error(w, http.StatusForbidden, "Akun Anda telah dinonaktifkan oleh Admin", nil)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		utils.Error(w, http.StatusUnauthorized, "Email atau password salah", nil)
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Role)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal membuat sesi login", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Login berhasil", map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func GetMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)

	var user models.User
	if err := config.DB.First(&user, "id = ?", userID).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "User tidak ditemukan", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Data profil berhasil diambil", user)
}
