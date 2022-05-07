package repository

import (
	"errors"

	"github.com/CHainGate/bitcoin-service/internal/model"
	"gorm.io/gorm"
)

type accountRepository struct {
	DB *gorm.DB
}

type IAccountRepository interface {
	FindUnused() (*model.Account, error)
	Create(account *model.Account) error
	Update(account *model.Account) error
}

func NewAccountRepository(db *gorm.DB) IAccountRepository {
	return &accountRepository{db}
}

func (r *accountRepository) FindUnused() (*model.Account, error) {
	var unusedAccount model.Account
	result := r.DB.Where("used = false").First(&unusedAccount)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &unusedAccount, nil
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
