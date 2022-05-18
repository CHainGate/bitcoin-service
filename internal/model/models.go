package model

import (
	"database/sql/driver"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/CHainGate/backend/pkg/enum"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4()"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Account struct {
	Base
	Address  string `gorm:"type:varchar"`
	Used     bool
	Payments []Payment
}

type Payment struct {
	Base
	Account               *Account
	AccountID             uuid.UUID `gorm:"type:uuid"`
	UserWallet            string
	Mode                  enum.Mode
	PriceAmount           float64 `gorm:"type:numeric(30,15);default:0"`
	PriceCurrency         enum.FiatCurrency
	CurrentPaymentStateId *uuid.UUID     `gorm:"type:uuid"`
	CurrentPaymentState   PaymentState   `gorm:"<-:false;foreignKey:CurrentPaymentStateId"`
	PaymentStates         []PaymentState // in eth service this one is <-:false
	Confirmations         int
}

type PaymentState struct {
	Base
	/*	AccountID      uuid.UUID `gorm:"type:uuid;"`*/
	PayAmount                *BigInt `gorm:"type:numeric(30);default:0"`
	AmountReceived           *BigInt `gorm:"type:numeric(30);default:0"`
	StateName                enum.State
	PaymentID                uuid.UUID `gorm:"type:uuid"`
	TransactionID            string
	TransactionConfirmations int64
}

type BigInt struct {
	big.Int
}

func NewBigIntFromInt(value int64) *BigInt {
	x := new(big.Int).SetInt64(value)
	return NewBigInt(x)
}

func NewBigInt(value *big.Int) *BigInt {
	return &BigInt{Int: *value}
}

func (bigInt *BigInt) Value() (driver.Value, error) {
	if bigInt == nil {
		return "null", nil
	}
	return bigInt.String(), nil
}

func (bigInt *BigInt) Scan(val interface{}) error {
	if val == nil {
		return nil
	}
	var data string
	switch v := val.(type) {
	case []byte:
		data = string(v)
	case string:
		data = v
	case int64:
		*bigInt = *NewBigIntFromInt(v)
		return nil
	default:
		return fmt.Errorf("bigint: can't convert %s type to *big.Int", reflect.TypeOf(val).Kind())
	}
	bigI, ok := new(big.Int).SetString(data, 10)
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", data)
	}
	bigInt.Int = *bigI
	return nil
}
