package repository

import (
	"github.com/CHainGate/bitcoin-service/internal/model"
	"gorm.io/gorm"
)

type paymentRepository struct {
	DB *gorm.DB
}

type IPaymentRepository interface {
	Create(account *model.Payment) error
}

func NewPaymentRepository(db *gorm.DB) IPaymentRepository {
	return &paymentRepository{db}
}

func (r *paymentRepository) Create(payment *model.Payment) error {
	result := r.DB.Create(&payment)
	if result.Error != nil {
		return result.Error
	}
	return nil
}
