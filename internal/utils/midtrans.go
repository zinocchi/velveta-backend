package utils

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"velveta-backend/internal/models"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

func getSnapClient() snap.Client {
	var s snap.Client
	env := midtrans.Sandbox
	if os.Getenv("MIDTRANS_IS_PRODUCTION") == "true" {
		env = midtrans.Production
	}
	s.New(os.Getenv("MIDTRANS_SERVER_KEY"), env)
	return s
}

func CreateMidtransSnapTransaction(order models.Order, customer models.User) (*snap.Response, error) {
	s := getSnapClient()

	var items []midtrans.ItemDetails
	for _, item := range order.Items {
		menuName := "Velveta Item"
		if item.Menu != nil {
			menuName = item.Menu.Name
		}
		items = append(items, midtrans.ItemDetails{
			ID:    item.MenuID.String(),
			Name:  menuName,
			Price: int64(item.UnitPrice),
			Qty:   int32(item.Quantity),
		})
	}

	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  order.InvoiceNumber,
			GrossAmt: int64(order.TotalAmount),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: customer.Name,
			Email: customer.Email,
		},
		Items: &items,
	}

	resp, err := s.CreateTransaction(req)
	if err != nil {
		return nil, fmt.Errorf("midtrans snap error: %v", err)
	}

	return resp, nil
}

// VerifyMidtransSignature memvalidasi SHA512(order_id + status_code + gross_amount + ServerKey)
func VerifyMidtransSignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	raw := orderID + statusCode + grossAmount + serverKey
	hash := sha512.Sum512([]byte(raw))
	expected := hex.EncodeToString(hash[:])
	return expected == signatureKey
}