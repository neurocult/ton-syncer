package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/vgarvardt/gue/v5"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"go.uber.org/zap"
)

func (s *Syncer) updater(ctx context.Context, job *gue.Job) (err error) {
	var args jobArgs
	if jsonErr := json.Unmarshal(job.Args, &args); jsonErr != nil {
		return fmt.Errorf("json unmarshal: %w", jsonErr)
	}

	defer func() {
		if err != nil {
			s.logger.Error("updater: job failed, going to retry after timeout", zap.Error(err), zap.Any("job_args", args))
			time.Sleep(time.Hour)
			return
		}
		if setTimeErr := s.storage.SetAccountEndSynсTime(ctx, args.AccountID, time.Now()); setTimeErr != nil {
			s.logger.Error(
				"updater: failed to update account's end_sync_time",
				zap.Error(setTimeErr),
				zap.Int("account_id", args.AccountID),
			)
		}
	}()

	// sometimes we need to stop pagination because cursor intersects already stored history
	hashString := txHashToString(args.TxHash)
	ok, err := s.storage.IsExistingCryptoTransaction(ctx, args.AccountID, hashString)
	if err != nil {
		return fmt.Errorf("check transaction existence for account: %w", err)
	}
	if ok {
		s.logger.Debug(
			"updater: account is up to date",
			zap.Int("account_id", args.AccountID),
			zap.String("transaction_hash", hashString),
		)
		return nil
	}

	addr, err := address.ParseAddr(args.Addr)
	if err != nil {
		return fmt.Errorf("parse addr: %w", err)
	}

	ctx = s.tonpool.StickyContext(ctx) // fetch all transactions from single node
	allFetchedTxs, err := s.ton.ListTransactions(ctx, addr, uint32(100), args.TxLT, args.TxHash)
	if err != nil {
		return fmt.Errorf("ton list transactions: %w", err)
	}

	casted, err := castTransactions(allFetchedTxs, args.AccountID, s.cfg.AssetID)
	if err != nil {
		return fmt.Errorf("cast transactions: %w", err)
	}

	if err = s.storage.CreateTonTransactions(ctx, casted); err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}

	oldestFetchedTx := allFetchedTxs[0]
	if oldestFetchedTx.PrevTxLT != 0 {
		if enqueueErr := s.enqueue( // it's important to enqueue only if everything else is ok to avoid infinite loop
			ctx,
			addr,
			args.AccountID,
			oldestFetchedTx.PrevTxHash,
			oldestFetchedTx.PrevTxLT,
		); enqueueErr != nil {
			return fmt.Errorf("enqueue: %w", enqueueErr)
		}
	}

	return nil
}

func castTransactions(in []*tlb.Transaction, accountID int, assetID int) (out []Transaction, err error) {
	out = make([]Transaction, 0, len(in))

	// reverse order from older to newer to from newer to older to make ids order clear
	for i := len(in) - 1; i >= 0; i-- {
		tx := in[i]

		parsed, err := parseTx(tx)
		if err != nil {
			return nil, fmt.Errorf("parse transaction: %w", err)
		}

		withoutFee := Transaction{
			AccountID:   accountID,
			Merchant:    parsed.merchant,
			Amount:      parsed.amount,
			Comment:     parsed.desc,
			CryptoHash:  &parsed.hash,
			CryptoTonLT: &tx.LT,
			EffectiveAt: parsed.effectiveAt,
			AssetID:     assetID,
		}
		out = append(out, withoutFee)

		if parsed.fee.Neg().IsZero() {
			continue
		}

		fee := Transaction{
			AccountID:   accountID,
			Merchant:    parsed.merchant,
			Amount:      parsed.fee.Neg(),
			Comment:     parsed.desc,
			CryptoHash:  &parsed.hash,
			CryptoTonLT: &tx.LT,
			EffectiveAt: parsed.effectiveAt,
			AssetID:     assetID,
		}

		out = append(out, fee)
	}

	return out, nil
}

type parseTxResult struct {
	merchant, desc, hash string
	amount, fee          decimal.Decimal
	effectiveAt          time.Time
}

func parseTx(tx *tlb.Transaction) (*parseTxResult, error) {
	result := &parseTxResult{}
	result.effectiveAt = time.Unix(int64(tx.Now), 0)
	result.hash = txHashToString(tx.Hash)

	fee := tx.TotalFees.Coins.NanoTON()

	if tx.IO.Out != nil {
		listOut, err := tx.IO.Out.ToSlice()
		if err != nil {
			return nil, fmt.Errorf("parse out messages: %w", err)
		}

		for _, m := range listOut {
			if m.MsgType == tlb.MsgTypeInternal {
				msg := m.AsInternal()

				amount, err := decimal.NewFromString(msg.Amount.TON())
				if err != nil {
					return nil, fmt.Errorf("parse out amount: %w", err)
				}

				fee.Add(fee, msg.IHRFee.NanoTON())
				fee.Add(fee, msg.FwdFee.NanoTON())

				result.amount = result.amount.Add(amount.Neg())
				result.desc = msg.Comment()
				result.merchant = msg.DestAddr().String()
			}
		}
	}

	if tx.IO.In != nil && tx.IO.In.MsgType == tlb.MsgTypeInternal {
		msg := tx.IO.In.AsInternal()

		amount, err := decimal.NewFromString(msg.Amount.TON())
		if err != nil {
			return nil, fmt.Errorf("parse int amount: %w", err)
		}

		result.amount = amount
		result.merchant = msg.SrcAddr.String()
		result.desc = msg.Comment()
	}

	decimalFee, err := decimal.NewFromString(tlb.FromNanoTON(fee).TON())
	if err != nil {
		return nil, fmt.Errorf("parse fee: %w", err)
	}

	result.fee = decimalFee

	return result, nil
}
