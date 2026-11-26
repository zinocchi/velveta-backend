package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name         string    `gorm:"type:varchar(100);not null" json:"name"`
	Email        string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"type:text;not null" json:"-"`
	Role         string    `gorm:"type:varchar(20);not null;default:'user'" json:"role"` // admin, cashier, user
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}
type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(50);not null" json:"name"`
	Slug      string    `gorm:"type:varchar(60);uniqueIndex;not null" json:"slug"`
	Menus     []Menu    `gorm:"foreignKey:CategoryID" json:"menus,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Menu struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	CategoryID  uint      `gorm:"not null" json:"category_id"`
	Category    *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Price       float64   `gorm:"type:decimal(12,2);not null" json:"price"`
	Stock       int       `gorm:"not null;default:0" json:"stock"`
	MinStock    int       `gorm:"not null;default:5" json:"min_stock"`
	ImageURL    string    `gorm:"type:text" json:"image_url"`
	IsAvailable bool      `gorm:"default:true" json:"is_available"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (m *Menu) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return
}

type Order struct {
	ID            uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	InvoiceNumber string      `gorm:"type:varchar(50);uniqueIndex;not null" json:"invoice_number"`
	UserID        uuid.UUID   `gorm:"type:uuid;not null" json:"user_id"`
	User          *User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CashierID     *uuid.UUID  `gorm:"type:uuid" json:"cashier_id"`
	Cashier       *User       `gorm:"foreignKey:CashierID" json:"cashier,omitempty"`
	OrderType     string      `gorm:"type:varchar(20);default:'dine_in'" json:"order_type"` // dine_in, takeaway
	TotalAmount   float64     `gorm:"type:decimal(12,2);not null" json:"total_amount"`
	OrderStatus   string      `gorm:"type:varchar(20);default:'pending'" json:"order_status"` // pending, preparing, ready, completed, cancelled
	Notes         string      `gorm:"type:text" json:"notes"`
	Items         []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	Payment       *Payment    `gorm:"foreignKey:OrderID" json:"payment,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

func (o *Order) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return
}

type OrderItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrderID   uuid.UUID `gorm:"type:uuid;not null" json:"order_id"`
	MenuID    uuid.UUID `gorm:"type:uuid;not null" json:"menu_id"`
	Menu      *Menu     `gorm:"foreignKey:MenuID" json:"menu,omitempty"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	UnitPrice float64   `gorm:"type:decimal(12,2);not null" json:"unit_price"`
	Subtotal  float64   `gorm:"type:decimal(12,2);not null" json:"subtotal"`
}

func (oi *OrderItem) BeforeCreate(tx *gorm.DB) (err error) {
	if oi.ID == uuid.Nil {
		oi.ID = uuid.New()
	}
	return
}

type Payment struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrderID               uuid.UUID  `gorm:"type:uuid;uniqueIndex;not null" json:"order_id"`
	PaymentMethod         string     `gorm:"type:varchar(30);not null" json:"payment_method"` // midtrans, cash
	PaymentChannel        string     `gorm:"type:varchar(50)" json:"payment_channel"`
	Amount                float64    `gorm:"type:decimal(12,2);not null" json:"amount"`
	Status                string     `gorm:"type:varchar(20);default:'unpaid'" json:"status"` // unpaid, paid, expired, failed
	SnapToken             string     `gorm:"type:text" json:"snap_token,omitempty"`
	SnapRedirectURL       string     `gorm:"type:text" json:"snap_redirect_url,omitempty"`
	MidtransTransactionID string     `gorm:"type:varchar(100)" json:"midtrans_transaction_id,omitempty"`
	PaidAt                *time.Time `json:"paid_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

func (p *Payment) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return
}