package repository

import (
	"time"

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
	FindPaidPayments() ([]model.Payment, error)
	FindConfirmedPayments() ([]model.Payment, error)
	FindForwardedPayments() ([]model.Payment, error)
	FindExpiredPayments() ([]model.Payment, error)
	FindAllOutgoingTransactionIdsByUserWallet(userWallet string) ([]string, error)
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

func (r *paymentRepository) FindPaidPayments() ([]model.Payment, error) {
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("confirmations = 0 AND  \"CurrentPaymentState\".\"state_name\" = ?", enum.Paid).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindConfirmedPayments() ([]model.Payment, error) {
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("\"CurrentPaymentState\".\"state_name\" = ?", enum.Confirmed).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindForwardedPayments() ([]model.Payment, error) {
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("\"CurrentPaymentState\".\"state_name\" = ?", enum.Forwarded).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindExpiredPayments() ([]model.Payment, error) {
	t := time.Now().Add(time.Minute * -15)
	var payments []model.Payment
	result := r.DB.
		Preload("Account").
		Joins("CurrentPaymentState").
		Where("payments.created_at < ? AND \"CurrentPaymentState\".\"state_name\" IN ?", t, []enum.State{enum.CurrencySelection, enum.Waiting, enum.PartiallyPaid}).
		Find(&payments)

	if result.Error != nil {
		return nil, result.Error
	}
	return payments, nil
}

func (r *paymentRepository) FindAllOutgoingTransactionIdsByUserWallet(userWallet string) ([]string, error) {
	var txIds []string
	result := r.DB.
		Table("payments").
		Select("payment_states.transaction_id").
		Joins("join payment_states on payment_states.payment_id = payments.id").
		Where("payments.user_wallet = ? AND payment_states.transaction_id != ''", userWallet).Scan(&txIds)

	if result.Error != nil {
		return nil, result.Error
	}
	return txIds, nil
}
