package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shemic/dever/util"

	usermodel "github.com/dever-package/user/model"
)

func (AccountService) PointLogs(ctx context.Context, request AccountPageRequest) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit := normalizeAccountPageLimit(request.Limit)
	filters := map[string]any{"user_id": userID}
	if request.PointConfigID > 0 {
		filters["point_config_id"] = request.PointConfigID
	}
	if cursor, _ := strconv.ParseUint(strings.TrimSpace(request.Cursor), 10, 64); cursor > 0 {
		filters["id"] = map[string]any{"lt": cursor}
	}
	rows := usermodel.NewPointLogModel().SelectMap(ctx, filters, map[string]any{
		"order": "id desc", "pageSize": limit + 1,
	})
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]map[string]any, 0, len(rows))
	nextCursor := ""
	for _, row := range rows {
		id := util.ToUint64(row["id"])
		nextCursor = strconv.FormatUint(id, 10)
		items = append(items, map[string]any{
			"id": id, "point_config_id": util.ToUint64(row["point_config_id"]),
			"point_name": strings.TrimSpace(util.ToString(row["point_name"])), "point_symbol": strings.TrimSpace(util.ToString(row["point_symbol"])),
			"change_type": strings.TrimSpace(util.ToString(row["change_type"])), "source": strings.TrimSpace(util.ToString(row["source"])),
			"amount": util.ToIntDefault(row["amount"], 0), "balance_before": util.ToIntDefault(row["balance_before"], 0),
			"balance_after": util.ToIntDefault(row["balance_after"], 0), "remark": strings.TrimSpace(util.ToString(row["remark"])),
			"created_at": accountOrderTime(row["created_at"]),
		})
	}
	if !hasMore {
		nextCursor = ""
	}
	return map[string]any{"items": items, "next_cursor": nextCursor}, nil
}

func (AccountService) Orders(ctx context.Context, request AccountPageRequest) (map[string]any, error) {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit := normalizeAccountPageLimit(request.Limit)
	identityCursor, pointCursor := parseAccountOrderCursor(request.Cursor)
	typeFilter := normalizeAccountOrderType(request.Type)
	identityRows := loadIdentityOrderPage(ctx, userID, request.PointConfigID, identityCursor, limit, typeFilter)
	pointRows := loadPointOrderPage(ctx, userID, request.PointConfigID, pointCursor, limit, typeFilter)
	type orderRow struct {
		typeName string
		row      map[string]any
		time     time.Time
	}
	combined := make([]orderRow, 0, len(identityRows)+len(pointRows))
	for _, row := range identityRows {
		combined = append(combined, orderRow{typeName: AccountOrderTypeIdentity, row: row, time: normalizeUserIdentityTime(row["created_at"])})
	}
	for _, row := range pointRows {
		combined = append(combined, orderRow{typeName: AccountOrderTypePoint, row: row, time: normalizeUserIdentityTime(row["created_at"])})
	}
	sort.SliceStable(combined, func(i, j int) bool {
		if combined[i].time.Equal(combined[j].time) {
			if combined[i].typeName == combined[j].typeName {
				return util.ToUint64(combined[i].row["id"]) > util.ToUint64(combined[j].row["id"])
			}
			return combined[i].typeName < combined[j].typeName
		}
		return combined[i].time.After(combined[j].time)
	})
	hasMore := len(combined) > limit
	if hasMore {
		combined = combined[:limit]
	}
	items := make([]map[string]any, 0, len(combined))
	nextIdentityCursor := identityCursor
	nextPointCursor := pointCursor
	for _, item := range combined {
		items = append(items, accountOrderResponse(item.typeName, item.row))
		if item.typeName == AccountOrderTypeIdentity {
			nextIdentityCursor = util.ToUint64(item.row["id"])
		} else {
			nextPointCursor = util.ToUint64(item.row["id"])
		}
	}
	nextCursor := ""
	if hasMore {
		nextCursor = formatAccountOrderCursor(nextIdentityCursor, nextPointCursor)
	}
	return map[string]any{"items": items, "next_cursor": nextCursor}, nil
}

func loadIdentityOrderPage(ctx context.Context, userID uint64, pointConfigID uint64, cursor uint64, limit int, orderType string) []map[string]any {
	if orderType != "" && orderType != AccountOrderTypeIdentity {
		return nil
	}
	filters := accountOrderPageFilters(userID, pointConfigID, cursor)
	return usermodel.NewIdentityOrderModel().SelectMap(ctx, filters, map[string]any{"order": "id desc", "pageSize": limit + 1})
}

func loadPointOrderPage(ctx context.Context, userID uint64, pointConfigID uint64, cursor uint64, limit int, orderType string) []map[string]any {
	if orderType != "" && orderType != AccountOrderTypePoint {
		return nil
	}
	filters := accountOrderPageFilters(userID, pointConfigID, cursor)
	return usermodel.NewPointRechargeOrderModel().SelectMap(ctx, filters, map[string]any{"order": "id desc", "pageSize": limit + 1})
}

func accountOrderPageFilters(userID uint64, pointConfigID uint64, cursor uint64) map[string]any {
	filters := map[string]any{"user_id": userID}
	if pointConfigID > 0 {
		filters["point_config_id"] = pointConfigID
	}
	if cursor > 0 {
		filters["id"] = map[string]any{"lt": cursor}
	}
	return filters
}

func normalizeAccountPageLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 50 {
		return 50
	}
	return value
}

func parseAccountOrderCursor(value string) (uint64, uint64) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0
	}
	identityID, _ := strconv.ParseUint(parts[0], 10, 64)
	pointID, _ := strconv.ParseUint(parts[1], 10, 64)
	return identityID, pointID
}

func formatAccountOrderCursor(identityID uint64, pointID uint64) string {
	return strconv.FormatUint(identityID, 10) + ":" + strconv.FormatUint(pointID, 10)
}
