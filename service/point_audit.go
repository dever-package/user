package service

import (
	"context"
	"fmt"

	"github.com/shemic/dever/server"
	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

const (
	defaultPointLedgerAuditLimit = 100
	maxPointLedgerAuditLimit     = 1000
	pointLedgerAccountBatchSize  = 200
	pointLedgerLogBatchSize      = 1000
)

type PointLedgerMismatch struct {
	UserPointID   uint64 `json:"user_point_id"`
	UserID        uint64 `json:"user_id"`
	PointConfigID uint64 `json:"point_config_id"`
	Balance       int64  `json:"balance"`
	LogBalance    int64  `json:"log_balance"`
	TotalAdded    int64  `json:"total_added"`
	LogTotalAdded int64  `json:"log_total_added"`
	TotalUsed     int64  `json:"total_used"`
	LogTotalUsed  int64  `json:"log_total_used"`
}

type pointLedgerTotals struct {
	balance    int64
	totalAdded int64
	totalUsed  int64
}

func (PointService) ProviderAuditLedger(c *server.Context, params []any) any {
	payload := clonePointPayload(params)
	rows, err := AuditPointLedger(c.Context(), util.ToIntDefault(payload["limit"], defaultPointLedgerAuditLimit))
	if err != nil {
		panic(err)
	}
	return map[string]any{"mismatch_count": len(rows), "mismatches": rows}
}

func AuditPointLedger(ctx context.Context, limit int) (mismatches []PointLedgerMismatch, resultErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			resultErr = fmt.Errorf("核对积分账本失败: %v", recovered)
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = defaultPointLedgerAuditLimit
	} else if limit > maxPointLedgerAuditLimit {
		limit = maxPointLedgerAuditLimit
	}
	for page := 1; ; page++ {
		accounts := usermodel.NewUserPointModel().SelectMap(ctx, nil, map[string]any{
			"field": "main.id,main.user_id,main.point_config_id,main.balance,main.total_added,main.total_used",
			"order": "main.id asc", "page": page, "pageSize": pointLedgerAccountBatchSize,
		})
		if len(accounts) == 0 {
			break
		}
		totals := pointLedgerLogTotals(ctx, accounts)
		for _, account := range accounts {
			accountID := util.ToUint64(account["id"])
			logged := totals[accountID]
			current := PointLedgerMismatch{
				UserPointID: accountID, UserID: util.ToUint64(account["user_id"]),
				PointConfigID: util.ToUint64(account["point_config_id"]),
				Balance:       util.ToInt64(account["balance"]), LogBalance: logged.balance,
				TotalAdded: util.ToInt64(account["total_added"]), LogTotalAdded: logged.totalAdded,
				TotalUsed: util.ToInt64(account["total_used"]), LogTotalUsed: logged.totalUsed,
			}
			if current.Balance == current.LogBalance && current.TotalAdded == current.LogTotalAdded && current.TotalUsed == current.LogTotalUsed {
				continue
			}
			mismatches = append(mismatches, current)
			if len(mismatches) >= limit {
				return mismatches, nil
			}
		}
		if len(accounts) < pointLedgerAccountBatchSize {
			break
		}
	}
	return mismatches, nil
}

func pointLedgerLogTotals(ctx context.Context, accounts []map[string]any) map[uint64]pointLedgerTotals {
	accountIDs := make([]any, 0, len(accounts))
	for _, account := range accounts {
		if id := util.ToUint64(account["id"]); id > 0 {
			accountIDs = append(accountIDs, id)
		}
	}
	result := make(map[uint64]pointLedgerTotals, len(accountIDs))
	for page := 1; len(accountIDs) > 0; page++ {
		rows := usermodel.NewPointLogModel().SelectMap(ctx, map[string]any{
			"user_point_id": accountIDs,
		}, map[string]any{
			"field": "main.user_point_id,main.change_type,main.amount",
			"order": "main.id asc", "page": page, "pageSize": pointLedgerLogBatchSize,
		})
		for _, row := range rows {
			accountID := util.ToUint64(row["user_point_id"])
			amount := util.ToInt64(row["amount"])
			current := result[accountID]
			switch normalizePointChangeType(row["change_type"]) {
			case pointChangeIncrease:
				current.balance += amount
				current.totalAdded += amount
			case pointChangeConsume:
				current.balance -= amount
				current.totalUsed += amount
			}
			result[accountID] = current
		}
		if len(rows) < pointLedgerLogBatchSize {
			break
		}
	}
	return result
}
