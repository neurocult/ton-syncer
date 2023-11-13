package syncer

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	ID          int
	AccountID   int
	AssetID     int
	CategoryID  int
	Merchant    string
	Amount      decimal.Decimal
	Comment     string
	CryptoHash  *string
	CryptoTonLT *uint64
	EffectiveAt time.Time
}

type Account struct {
	ID                 int
	UserID             int
	Name               string
	MainAssetID        int
	CryptoAddress      *string
	CryptoBlockchainID *int
}
