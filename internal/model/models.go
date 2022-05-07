package model

import (
	"database/sql/driver"
	"fmt"
	"github.com/CHainGate/backend/pkg/enum"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math/big"
	"reflect"
	"time"
)

type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4()"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Account struct {
	Base
	Address   string `gorm:"type:varchar"`
	Used      bool
	Payments  []Payment
	Remainder *BigInt `gorm:"type:numeric(30);default:0"`
}

type Payment struct {
	Base
	Account               *Account
	AccountId             uuid.UUID `gorm:"type:uuid"`
	UserWallet            string
	Mode                  enum.Mode
	PriceAmount           float64 `gorm:"type:numeric(30,15);default:0"`
	PriceCurrency         enum.FiatCurrency
	CurrentPaymentStateId *uuid.UUID     `gorm:"type:uuid"`
	CurrentPaymentState   PaymentState   `gorm:"foreignKey:CurrentPaymentStateId"`
	PaymentStates         []PaymentState `gorm:"<-:false"`
	Confirmations         int
}

type PaymentState struct {
	Base
	AccountId      uuid.UUID `gorm:"type:uuid;"`
	PayAmount      *BigInt   `gorm:"type:numeric(30);default:0"`
	AmountReceived *BigInt   `gorm:"type:numeric(30);default:0"`
	StateName      enum.State
	PaymentId      uuid.UUID `gorm:"type:uuid"`
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
