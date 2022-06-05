package repository

import (
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"gorm.io/gorm"
	"time"
)

type paymentRepository struct {
	DB *gorm.DB
}

type IPaymentRepository interface {
	Create(account *model.Payment) error
	FindCurrentPaymentByAddress(address string) (*model.Payment, error)
	Update(payment *model.Payment) error
	FindPaidPaymentsByMode(mode enum.Mode) ([]model.Payment, error)
	FindConfirmedPaymentsByMode(mode enum.Mode) ([]model.Payment, error)
	FindForwardedPaymentsByMode(mode enum.Mode) ([]model.Payment, error)
	FindExpiredPaymentsByMode(mode enum.Mode) ([]model.Payment, error)
	FindAllOutgoingTransactionIdsByUserWalletAndMode(userWallet string, mode enum.Mode) ([]string, error)
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

func (r *paymentRepository) FindPaidPaymentsByMode(mode enum.Mode) ([]model.Payment, error) {
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("\"CurrentPaymentState\".\"state_name\" = ? AND mode = ?", enum.Paid, mode).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindConfirmedPaymentsByMode(mode enum.Mode) ([]model.Payment, error) {
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("\"CurrentPaymentState\".\"state_name\" = ? AND mode = ?", enum.Confirmed, mode).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindForwardedPaymentsByMode(mode enum.Mode) ([]model.Payment, error) {
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("\"CurrentPaymentState\".\"state_name\" = ? AND mode = ?", enum.Forwarded, mode).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindExpiredPaymentsByMode(mode enum.Mode) ([]model.Payment, error) {
	t := time.Now().Add(time.Minute * -15)
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("payments.created_at < ? AND \"CurrentPaymentState\".\"state_name\" IN ? AND mode = ?", t, []enum.State{enum.CurrencySelection, enum.Waiting, enum.PartiallyPaid}, mode).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindAllOutgoingTransactionIdsByUserWalletAndMode(userWallet string, mode enum.Mode) ([]string, error) {
	var txIds []string
	result := r.DB.
		Table("payments").
		Select("forwarding_transaction_id").
		Where("user_wallet = ? AND mode = ? AND forwarding_transaction_id IS NOT NULL", userWallet, mode).Scan(&txIds)

	if result.Error != nil {
		return nil, result.Error
	}
	return txIds, nil
}
