package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"
	"velveta-backend/internal/config"
	"velveta-backend/internal/middleware"
	"velveta-backend/internal/models"
	"velveta-backend/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateOrderRequest struct {
	OrderType     string `json:"order_type" validate:"omitempty,oneof=dine_in takeaway"`
	PaymentMethod string `json:"payment_method" validate:"required,oneof=midtrans cash"`
	Notes         string `json:"notes"`
	Items         []struct {
		MenuID   uuid.UUID `json:"menu_id" validate:"required"`
		Quantity int       `json:"quantity" validate:"required,min=1"`
	} `json:"items" validate:"required,min=1,dive"`
}

func generateInvoiceNumber() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	return fmt.Sprintf("INV-%s-%04d", time.Now().Format("20060102"), n.Int64()+1000)
}

// ==================== CREATE ORDER (CUSTOMER & CASHIER) ====================

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	role := r.Context().Value(middleware.RoleKey).(string)

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}
	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Validasi gagal", err.Error())
		return
	}

	// Hanya kasir atau admin yang boleh memproses pembayaran tunai (cash)
	if req.PaymentMethod == "cash" && role == "user" {
		utils.Error(w, http.StatusForbidden, "Pembayaran tunai hanya dapat diproses melalui kasir", nil)
		return
	}

	if req.OrderType == "" {
		req.OrderType = "dine_in"
	}

	var customer models.User
	if err := config.DB.First(&customer, "id = ?", userID).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Data pengguna tidak ditemukan", nil)
		return
	}

	var createdOrder models.Order

	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var totalAmount float64
		var orderItems []models.OrderItem

		for _, itemReq := range req.Items {
			var menu models.Menu
			// Kunci baris menu agar tidak terjadi race condition saat stok dipotong
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&menu, "id = ?", itemReq.MenuID).Error; err != nil {
				return fmt.Errorf("menu dengan ID %s tidak ditemukan", itemReq.MenuID)
			}

			if !menu.IsAvailable {
				return fmt.Errorf("menu '%s' sedang tidak tersedia", menu.Name)
			}
			if menu.Stock < itemReq.Quantity {
				return fmt.Errorf("stok '%s' tidak mencukupi (tersisa %d)", menu.Name, menu.Stock)
			}

			// Potong stok menu
			menu.Stock -= itemReq.Quantity
			if menu.Stock == 0 {
				menu.IsAvailable = false
			}
			if err := tx.Save(&menu).Error; err != nil {
				return err
			}

			subtotal := menu.Price * float64(itemReq.Quantity)
			totalAmount += subtotal

			menuCopy := menu
			orderItems = append(orderItems, models.OrderItem{
				MenuID:    menu.ID,
				Menu:      &menuCopy,
				Quantity:  itemReq.Quantity,
				UnitPrice: menu.Price,
				Subtotal:  subtotal,
			})
		}

		order := models.Order{
			InvoiceNumber: generateInvoiceNumber(),
			UserID:        userID,
			OrderType:     req.OrderType,
			TotalAmount:   totalAmount,
			OrderStatus:   "pending",
			Notes:         req.Notes,
			Items:         orderItems,
		}

		if role == "cashier" || role == "admin" {
			order.CashierID = &userID
		}

		if err := tx.Create(&order).Error; err != nil {
			return err
		}

		// Buat record pembayaran
		payment := models.Payment{
			OrderID:       order.ID,
			PaymentMethod: req.PaymentMethod,
			Amount:        totalAmount,
			Status:        "unpaid",
		}

		if req.PaymentMethod == "cash" {
			now := time.Now()
			payment.Status = "paid"
			payment.PaymentChannel = "cash"
			payment.PaidAt = &now
			order.OrderStatus = "preparing"
			tx.Save(&order)
		} else if req.PaymentMethod == "midtrans" {
			snapResp, snapErr := utils.CreateMidtransSnapTransaction(order, customer)
			if snapErr != nil {
				return snapErr
			}
			payment.SnapToken = snapResp.Token
			payment.SnapRedirectURL = snapResp.RedirectURL
		}

		if err := tx.Create(&payment).Error; err != nil {
			return err
		}

		order.Payment = &payment
		createdOrder = order
		return nil
	})

	if err != nil {
		utils.Error(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.JSON(w, http.StatusCreated, true, "Pesanan berhasil dibuat", createdOrder)
}

// ==================== USER ORDER HISTORY & STATUS ====================

func GetMyOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)

	var orders []models.Order
	if err := config.DB.
		Preload("Items.Menu").
		Preload("Payment").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal mengambil riwayat pesanan", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Riwayat pesanan berhasil diambil", orders)
}

func GetOrderDetail(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	role := r.Context().Value(middleware.RoleKey).(string)

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID pesanan tidak valid", nil)
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("User").
		Preload("Cashier").
		Preload("Items.Menu").
		Preload("Payment").
		First(&order, "id = ?", orderID).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Pesanan tidak ditemukan", nil)
		return
	}

	// Customer hanya boleh melihat pesanannya sendiri
	if role == "user" && order.UserID != userID {
		utils.Error(w, http.StatusForbidden, "Anda tidak memiliki akses ke pesanan ini", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Detail pesanan berhasil diambil", order)
}

// ==================== CASHIER & ADMIN ORDER MANAGEMENT ====================

func ManageGetOrders(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	paymentStatus := r.URL.Query().Get("payment_status")

	query := config.DB.
		Preload("User").
		Preload("Cashier").
		Preload("Items.Menu").
		Preload("Payment").
		Order("orders.created_at DESC")

	if statusFilter != "" {
		query = query.Where("orders.order_status = ?", statusFilter)
	}
	if paymentStatus != "" {
		query = query.Joins("JOIN payments ON payments.order_id = orders.id").
			Where("payments.status = ?", paymentStatus)
	}

	var orders []models.Order
	if err := query.Find(&orders).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal mengambil daftar pesanan", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Daftar pesanan berhasil diambil", orders)
}

func ManageUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	staffID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID pesanan tidak valid", nil)
		return
	}

	var req struct {
		OrderStatus string `json:"order_status" validate:"required,oneof=pending preparing ready completed cancelled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}
	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Status pesanan tidak valid", err.Error())
		return
	}

	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
			return fmt.Errorf("pesanan tidak ditemukan")
		}

		// Jika pesanan dibatalkan dan sebelumnya belum dibatalkan, kembalikan stok menu
		if req.OrderStatus == "cancelled" && order.OrderStatus != "cancelled" {
			for _, item := range order.Items {
				tx.Model(&models.Menu{}).Where("id = ?", item.MenuID).
					Updates(map[string]interface{}{
						"stock":        gorm.Expr("stock + ?", item.Quantity),
						"is_available": true,
					})
			}
		}

		order.OrderStatus = req.OrderStatus
		if order.CashierID == nil {
			order.CashierID = &staffID
		}
		return tx.Save(&order).Error
	})

	if err != nil {
		utils.Error(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Status pesanan berhasil diperbarui", nil)
}

func ManageBulkUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderIDs    []uuid.UUID `json:"order_ids" validate:"required,min=1"`
		OrderStatus string      `json:"order_status" validate:"required,oneof=pending preparing ready completed cancelled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Format JSON tidak valid", nil)
		return
	}
	if err := utils.Validate.Struct(req); err != nil {
		utils.Error(w, http.StatusBadRequest, "Validasi gagal", err.Error())
		return
	}

	if err := config.DB.Model(&models.Order{}).Where("id IN ?", req.OrderIDs).Update("order_status", req.OrderStatus).Error; err != nil {
		utils.Error(w, http.StatusInternalServerError, "Gagal memperbarui status massal", nil)
		return
	}

	utils.JSON(w, http.StatusOK, true, "Status pesanan massal berhasil diperbarui", nil)
}

// ConfirmCashPayment memungkinkan kasir melunasi pesanan self-order user yang memilih bayar di kasir
func ConfirmCashPayment(w http.ResponseWriter, r *http.Request) {
	staffID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "ID pesanan tidak valid", nil)
		return
	}

	var order models.Order
	if err := config.DB.Preload("Payment").First(&order, "id = ?", orderID).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Pesanan tidak ditemukan", nil)
		return
	}

	if order.Payment == nil {
		utils.Error(w, http.StatusBadRequest, "Data pembayaran tidak ditemukan", nil)
		return
	}
	if order.Payment.Status == "paid" {
		utils.Error(w, http.StatusBadRequest, "Pesanan ini sudah lunas", nil)
		return
	}

	now := time.Now()
	order.Payment.PaymentMethod = "cash"
	order.Payment.PaymentChannel = "cash"
	order.Payment.Status = "paid"
	order.Payment.PaidAt = &now
	config.DB.Save(order.Payment)

	if order.OrderStatus == "pending" {
		order.OrderStatus = "preparing"
	}
	order.CashierID = &staffID
	config.DB.Save(&order)

	utils.JSON(w, http.StatusOK, true, "Pembayaran tunai berhasil dikonfirmasi", order)
}

// ==================== PAYMENT METHODS & MIDTRANS WEBHOOK ====================

func GetPaymentMethods(w http.ResponseWriter, r *http.Request) {
	methods := []map[string]interface{}{
		{"code": "midtrans", "name": "QRIS / Virtual Account / E-Wallet (Midtrans)", "enabled": true},
		{"code": "cash", "name": "Tunai di Kasir (Cash)", "enabled": true},
	}
	utils.JSON(w, http.StatusOK, true, "Metode pembayaran tersedia", methods)
}

func MidtransWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.Error(w, http.StatusBadRequest, "Payload tidak valid", nil)
		return
	}

	invoiceNumber, _ := payload["order_id"].(string)
	statusCode, _ := payload["status_code"].(string)
	grossAmount, _ := payload["gross_amount"].(string)
	signatureKey, _ := payload["signature_key"].(string)
	transactionStatus, _ := payload["transaction_status"].(string)
	fraudStatus, _ := payload["fraud_status"].(string)
	paymentType, _ := payload["payment_type"].(string)
	transactionID, _ := payload["transaction_id"].(string)

	if !utils.VerifyMidtransSignature(invoiceNumber, statusCode, grossAmount, signatureKey) {
		utils.Error(w, http.StatusUnauthorized, "Signature Midtrans tidak valid", nil)
		return
	}

	var order models.Order
	if err := config.DB.Preload("Payment").Where("invoice_number = ?", invoiceNumber).First(&order).Error; err != nil {
		utils.Error(w, http.StatusNotFound, "Invoice tidak ditemukan", nil)
		return
	}

	if order.Payment == nil {
		utils.Error(w, http.StatusNotFound, "Data pembayaran tidak ditemukan", nil)
		return
	}

	order.Payment.PaymentChannel = paymentType
	order.Payment.MidtransTransactionID = transactionID

	switch transactionStatus {
	case "capture":
		if fraudStatus == "accept" {
			now := time.Now()
			order.Payment.Status = "paid"
			order.Payment.PaidAt = &now
			if order.OrderStatus == "pending" {
				order.OrderStatus = "preparing"
			}
		}
	case "settlement":
		now := time.Now()
		order.Payment.Status = "paid"
		order.Payment.PaidAt = &now
		if order.OrderStatus == "pending" {
			order.OrderStatus = "preparing"
		}
	case "expire":
		order.Payment.Status = "expired"
	case "cancel", "deny":
		order.Payment.Status = "failed"
	}

	config.DB.Save(order.Payment)
	config.DB.Save(&order)

	utils.JSON(w, http.StatusOK, true, "Webhook diproses", nil)
}