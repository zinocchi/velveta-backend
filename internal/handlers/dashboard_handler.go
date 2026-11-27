package handlers

import (
	"net/http"
	"time"
	"velveta-backend/internal/config"
	"velveta-backend/internal/models"
	"velveta-backend/internal/utils"
)

func AdminGetDashboard(w http.ResponseWriter, r *http.Request) {
	var totalRevenue float64
	config.DB.Model(&models.Payment{}).
		Where("status = ?", "paid").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&totalRevenue)

	var totalOrders int64
	config.DB.Model(&models.Order{}).Count(&totalOrders)

	var pendingOrders int64
	config.DB.Model(&models.Order{}).Where("order_status IN ?", []string{"pending", "preparing"}).Count(&pendingOrders)

	var lowStockCount int64
	config.DB.Model(&models.Menu{}).Where("stock <= min_stock").Count(&lowStockCount)

	var totalUsers int64
	config.DB.Model(&models.User{}).Where("role = ?", "user").Count(&totalUsers)

	var recentOrders []models.Order
	config.DB.Preload("User").Preload("Payment").Order("created_at DESC").Limit(5).Find(&recentOrders)

	type TopMenu struct {
		MenuID    string  `json:"menu_id"`
		Name      string  `json:"name"`
		TotalSold int64   `json:"total_sold"`
		Revenue   float64 `json:"revenue"`
	}
	var topMenus []TopMenu
	config.DB.Table("order_items").
		Select("order_items.menu_id, menus.name, SUM(order_items.quantity) as total_sold, SUM(order_items.subtotal) as revenue").
		Joins("JOIN menus ON menus.id = order_items.menu_id").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.order_status != ?", "cancelled").
		Group("order_items.menu_id, menus.name").
		Order("total_sold DESC").
		Limit(5).
		Scan(&topMenus)

	utils.JSON(w, http.StatusOK, true, "Data dashboard berhasil diambil", map[string]interface{}{
		"summary": map[string]interface{}{
			"total_revenue":   totalRevenue,
			"total_orders":    totalOrders,
			"pending_orders":  pendingOrders,
			"low_stock_items": lowStockCount,
			"total_customers": totalUsers,
		},
		"top_menus":     topMenus,
		"recent_orders": recentOrders,
	})
}

func AdminGetRevenueReport(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	query := config.DB.Table("payments").
		Select("DATE(paid_at) as date, COUNT(id) as total_transactions, SUM(amount) as revenue").
		Where("status = ? AND paid_at IS NOT NULL", "paid")

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			query = query.Where("paid_at >= ?", t)
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			query = query.Where("paid_at < ?", t.Add(24*time.Hour))
		}
	}

	type DailyRevenue struct {
		Date              string  `json:"date"`
		TotalTransactions int64   `json:"total_transactions"`
		Revenue           float64 `json:"revenue"`
	}
	var report []DailyRevenue
	query.Group("DATE(paid_at)").Order("date ASC").Scan(&report)

	utils.JSON(w, http.StatusOK, true, "Laporan pendapatan berhasil diambil", report)
}