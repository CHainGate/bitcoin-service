package repository

import (
	"fmt"
	"github.com/CHainGate/bitcoin-service/internal/model"
	"github.com/CHainGate/bitcoin-service/internal/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func SetupDatabase() error {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", utils.Opts.DbHost, utils.Opts.DbUser, utils.Opts.DbPassword, utils.Opts.DbName, utils.Opts.DbPort)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil
	}

	err = autoMigrateDB(db)
	if err != nil {
		return err
	}

	/*	merchantRepo, apiKeyRepo, paymentRepo, err := createRepositories(db)
		if err != nil {
			return err
		}*/

	return nil
}

func autoMigrateDB(db *gorm.DB) error {
	err := db.AutoMigrate(&model.Payment{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&model.PaymentState{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&model.Account{})
	if err != nil {
		return err
	}
	return nil
}

/*func createRepositories(db *gorm.DB) (IMerchantRepository, IApiKeyRepository, IPaymentRepository, error) {
	merchantRepo, err := NewMerchantRepository(db)
	if err != nil {
		return nil, nil, nil, err
	}

	paymentRepo, err := NewPaymentRepository(db)
	if err != nil {
		return nil, nil, nil, err
	}

	apiKeyRepo, err := NewApiKeyRepository(db)
	if err != nil {
		return nil, nil, nil, err
	}
	return merchantRepo, apiKeyRepo, paymentRepo, nil
}*/
