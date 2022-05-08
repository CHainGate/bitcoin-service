package repository

import (
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"gorm.io/gorm"
)

type paymentRepository struct {
	DB *gorm.DB
}

type IPaymentRepository interface {
	Create(account *model.Payment) error
	FindCurrentPaymentByAddress(address string) (*model.Payment, error)
	Update(payment *model.Payment) error
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

func (r *paymentRepository) Update(payment *model.Payment) error {
	result := r.DB.Save(&payment)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *paymentRepository) FindCurrentPaymentByAddress(address string) (*model.Payment, error) {
	var payment model.Payment
	result := r.DB.
		Joins("CurrentPaymentState").
		Joins("Account").
		Where("\"Account\".\"address\" = ? AND \"CurrentPaymentState\".\"state_name\" IN ?", address, []enum.State{enum.Waiting, enum.PartiallyPaid}).
		First(&payment)

	if result.Error != nil {
		return nil, result.Error
	}
	return &payment, nil
}
