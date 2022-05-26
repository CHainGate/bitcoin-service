package repository

import (
	"errors"
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"gorm.io/gorm"
)

type accountRepository struct {
	DB *gorm.DB
}

type IAccountRepository interface {
	FindUnusedByMode(mode enum.Mode) (*model.Account, error)
	FindByAddress(address string) (*model.Account, error)
	Create(account *model.Account) error
	Update(account *model.Account) error
	FindAll() ([]model.Account, error)
}

func NewAccountRepository(db *gorm.DB) IAccountRepository {
	return &accountRepository{db}
}

func (r *accountRepository) FindUnusedByMode(mode enum.Mode) (*model.Account, error) {
	var unusedAccount model.Account
	result := r.DB.Where("used = false AND mode = ?", mode).First(&unusedAccount)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &unusedAccount, nil
}

func (r *accountRepository) FindByAddress(address string) (*model.Account, error) {
	var account model.Account
	result := r.DB.
		Preload("Payments.CurrentPaymentState").
		Preload("Payments.PaymentStates").
		Where("address = ?", address).
		Find(&account)
	if result.Error != nil {
		return nil, result.Error
	}
	return &account, nil
}

func (r *accountRepository) Create(account *model.Account) error {
	result := r.DB.Create(&account)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *accountRepository) Update(account *model.Account) error {
	result := r.DB.Save(&account)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (r *accountRepository) FindAll() ([]model.Account, error) {
	var acc []model.Account
	result := r.DB.
		Preload("Payments.CurrentPaymentState").
		Preload("Payments.PaymentStates").
		Find(&acc)

	if result.Error != nil {
		return nil, result.Error
	}
	return acc, nil
}
