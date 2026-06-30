package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/gamedata"
	"poker-fate-server/internal/proto/gen"
)

// Shop module handlers. Implements the contract that ShopModel.lua expects:
//   - /shop/limit       -> {code, list:[{shop_type, id, count, double, reward, first_recharge}]}
//   - /shop/pid         -> {code, currency_id, currency_code, list:[{id, pri_1..pri_4, ...}]}
//   - /shop/createOrder -> {code, order_id, uuid}
//   - pay endpoints     -> {code, item_list:[{major_type, item_id, num}]}
//   - /shop/reward      -> {code}  (free reward claim; server marks reward_claimed)
//   - /shop/buyWithProp -> {code}  (in-game-currency purchase; server updates counts)
//
// Purchase counters are persisted in user_shop_limits. The private server
// grants every purchase (no real-money validation) and delivers gold/items.

// ShopCreateOrderHandler creates a pending shop order. The returned order_id
// and uuid are passed by the client SDK to the native payment flow.
//
// On the private server the PC/Steam native payment path is broken (Goldberg
// steam_api64 never fires onPcPayResult, and PayHelper.lua's onPcPayResult is
// commented out). To unblock recharges, the server delivers rewards right
// after creating the order and pushes a WS NoticeBRC (type 10091, the
// "pay success for stove" range) so the client's net_game.lua:272 handler
// calls ShopModel:doPaySuc() (hides PayMask) + onPaySucces() (shows reward).
func (r *Router) ShopCreateOrderHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	shopType := intVal(params, "shop_type")
	productID := intVal(params, "id")
	pid := strVal(params, "pid")

	orderID := uuid.New().String()
	orderUUID := uuid.New().String()

	if uid > 0 && productID > 0 {
		_, _ = r.DB.ExecContext(context.Background(),
			`INSERT INTO shop_orders (uid, order_id, product_id, status)
			 VALUES ($1, $2, $3, 0)`,
			uid, orderID, pid)
	}

	// Deliver rewards immediately and push a pay-success notification via WS
	// so the client closes PayMask and shows the reward UI. The push is sent
	// in a goroutine with a short delay to ensure the HTTP response (which
	// triggers PayHelper:pay on the client) is processed first.
	rewards := r.shopRewardsForPid(uid, pid)
	if uid > 0 {
		go r.pushPaySuccess(uid, pid, rewards)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"order_id":  orderID,
		"uuid":      orderUUID,
		"shop_type": shopType,
		"id":        productID,
	})
}

// pushPaySuccess sends a WS NoticeBRC (type 10091) to the user so the client's
// net_game.lua pay-success handler fires: doPaySuc() hides PayMask and
// onPaySucces() shows the reward view. The 500ms delay lets the createOrder
// HTTP response callback (which calls PayHelper:pay) complete first.
func (r *Router) pushPaySuccess(uid int64, pid string, rewards []gin.H) {
	time.Sleep(500 * time.Millisecond)

	sess := r.WSSrv.GetSession(uid)
	if sess == nil {
		r.Logger.Warn("pushPaySuccess: user not online, skipping WS push",
			zap.Int64("uid", uid))
		return
	}

	msg := map[string]interface{}{
		"pid":       pid,
		"item_list": rewards,
	}
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		r.Logger.Warn("pushPaySuccess: json marshal failed", zap.Error(err))
		return
	}

	push := &gen.NoticeBRC{
		Type:    proto.Int32(10091), // pay success for stove (10091..10099 range)
		Message: proto.String(string(msgJSON)),
	}
	body, err := proto.Marshal(push)
	if err != nil {
		r.Logger.Warn("pushPaySuccess: proto marshal failed", zap.Error(err))
		return
	}

	sess.SendPacket("pb.NoticeBRC", 0, body)
	r.Logger.Info("pushPaySuccess: sent pay-success push",
		zap.Int64("uid", uid), zap.String("pid", pid))
	r.pushGoldUpdate(uid)
}

// pushGoldUpdate sends a WS UserValueRSP so the client's gold display refreshes
// immediately after a server-side gold change (task reward, shop purchase, VIP
// reward, etc.). The client's net:UserValueRSP handler calls PlayerModel:setGold
// and emits evt_refreshTopInfo. A short delay lets the triggering HTTP response
// land first so the reward popup isn't visually interrupted.
func (r *Router) pushGoldUpdate(uid int64) {
	if uid == 0 {
		return
	}
	var gold int64
	err := r.DB.QueryRowContext(context.Background(),
		`SELECT COALESCE(gold, 0) FROM users WHERE uid=$1`, uid,
	).Scan(&gold)
	if err != nil {
		r.Logger.Warn("pushGoldUpdate: read gold failed",
			zap.Int64("uid", uid), zap.Error(err))
		return
	}
	sess := r.WSSrv.GetSession(uid)
	if sess == nil {
		return
	}
	rsp := &gen.UserValueRSP{
		Type:  proto.Int32(10100001), // GPropId.Gold
		Value: proto.Int64(gold),
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.UserValueRSP", 0, body)
}

// ShopSteamPayHandler handles /shop/steamPay. The client's SteamPayment.lua
// expects {code, trans_id, order_id} and stores them in _steamPayInfo, which
// DoSteamPayResult later matches against the SDK's authorized order_id. The
// private server generates synthetic ids so the Lua state machine advances
// without a real Stove SDK callback.
func (r *Router) ShopSteamPayHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	orderID := strVal(params, "order_id")

	transID := uuid.New().String()
	// Echo the client's order_id so DoSteamPayResult can match it.
	if orderID == "" {
		orderID = uuid.New().String()
	}

	if uid > 0 && orderID != "" {
		_, _ = r.DB.ExecContext(context.Background(),
			`UPDATE shop_orders SET status=1, updated_at=NOW() WHERE order_id=$1 AND uid=$2`,
			orderID, uid)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"trans_id": transID,
		"order_id": orderID,
	})
}

// ShopPayDirectHandler is the unified pay-confirmation handler for finishSteamPay,
// Google, Apple and third-party payments. On success it delivers rewards and
// marks the order complete. ShopModel:onPaySucces reads data.item_list.
func (r *Router) ShopPayDirectHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	pid := strVal(params, "pid")
	orderID := strVal(params, "order_id")

	// Fallback: if the client didn't send pid (e.g. legacy DoSteamPayResult
	// path), look it up from the pending order so rewards can be determined.
	if pid == "" && orderID != "" && uid > 0 {
		_ = r.DB.QueryRowContext(context.Background(),
			`SELECT product_id FROM shop_orders WHERE order_id=$1 AND uid=$2`,
			orderID, uid).Scan(&pid)
	}

	// Mark the originating order complete so /shop/limit counters stay in sync.
	if orderID != "" && uid > 0 {
		_, _ = r.DB.ExecContext(context.Background(),
			`UPDATE shop_orders SET status=1, updated_at=NOW() WHERE order_id=$1 AND uid=$2`,
			orderID, uid)
	}

	rewards := r.shopRewardsForPid(uid, pid)
	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"item_list": rewards,
	})
}

// ShopRewardHandler claims a free reward (first-recharge reward or daily-free
// recharge item). Server marks the corresponding flag in user_shop_limits so
// the client can no longer claim it, and delivers the configured rewards so
// the user actually receives the items.
func (r *Router) ShopRewardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	shopType := intVal(params, "shop_type")
	productID := intVal(params, "id")

	var itemList []gin.H
	if uid > 0 && shopType > 0 && productID > 0 {
		_, _ = r.DB.ExecContext(context.Background(),
			`INSERT INTO user_shop_limits (uid, shop_type, product_id, reward_claimed, first_recharge_claimed, updated_at)
			 VALUES ($1, $2, $3, true, true, NOW())
			 ON CONFLICT (uid, shop_type, product_id)
			 DO UPDATE SET reward_claimed=true, first_recharge_claimed=true, updated_at=NOW()`,
			uid, shopType, productID)

		// Deliver configured rewards for this product.
		if row, ok := gamedata.ShopPidByID(int32(productID)); ok {
			rewards := gamedata.ShopRewardsByPid(row.Pid)
			if len(rewards) > 0 {
				itemList = r.grantRewards(uid, rewards)
			}
		}
	}
	if itemList == nil {
		itemList = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "item_list": itemList})
}

// ShopBuyWithPropHandler processes an in-game-currency purchase. The server
// increments the user_shop_limits counter so the client can refresh sold-out
// state via the subsequent /shop/limit call, and delivers the configured
// rewards so the user actually receives the items.
func (r *Router) ShopBuyWithPropHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	shopType := intVal(params, "shop_type")
	productID := intVal(params, "id")
	num := intVal(params, "num")
	if num <= 0 {
		num = 1
	}

	var itemList []gin.H
	if uid > 0 && shopType > 0 && productID > 0 {
		_, _ = r.DB.ExecContext(context.Background(),
			`INSERT INTO user_shop_limits (uid, shop_type, product_id, count, period_count, updated_at)
			 VALUES ($1, $2, $3, $4, $4, NOW())
			 ON CONFLICT (uid, shop_type, product_id)
			 DO UPDATE SET count=user_shop_limits.count+$4,
			               period_count=user_shop_limits.period_count+$4,
			               updated_at=NOW()`,
			uid, shopType, productID, num)

		// Deliver configured rewards for this product (per-unit rewards scaled
		// by num). The private server does not deduct the purchase currency;
		// operators who want strict accounting should add a balance check here.
		if row, ok := gamedata.ShopPidByID(int32(productID)); ok {
			rewards := gamedata.ShopRewardsByPid(row.Pid)
			if len(rewards) > 0 && num > 0 {
				scaled := make([]gamedata.Reward, 0, len(rewards))
				for _, rw := range rewards {
					scaled = append(scaled, gamedata.Reward{
						MajorType: rw.MajorType,
						ItemID:    rw.ItemID,
						Num:       rw.Num * int32(num),
					})
				}
				itemList = r.grantRewards(uid, scaled)
			}
		}
	}
	if itemList == nil {
		itemList = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "item_list": itemList})
}

// ShopLimitHandler returns the purchase-counter list. ShopModel.lua:414 does
// `pairs(data.list)` with no nil guard, so list MUST be present (empty array
// is fine for a fresh account).
func (r *Router) ShopLimitHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT shop_type, product_id, count, period_count,
		        double_reward_claimed, reward_claimed, first_recharge_claimed
		 FROM user_shop_limits WHERE uid=$1`, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var shopType, productID, count, periodCount int
		var double, reward, firstRecharge bool
		_ = rows.Scan(&shopType, &productID, &count, &periodCount, &double, &reward, &firstRecharge)

		doubleVal := 0
		if double {
			doubleVal = 1
		}
		rewardVal := 0
		if reward {
			rewardVal = 1
		}
		firstRechargeVal := 0
		if firstRecharge {
			firstRechargeVal = 1
		}

		list = append(list, gin.H{
			"shop_type":      shopType,
			"id":             productID,
			"count":          count,
			"period_count":   periodCount,
			"double":         doubleVal,
			"reward":         rewardVal,
			"first_recharge": firstRechargeVal,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// ShopPidHandler returns the user's currency and the price list.
// ShopModel.lua:433 assigns data.currency_id directly to self._currency_id
// (no guard) and iterates data.list at line 436 (no guard), so both fields
// are mandatory. The currency_id must be 1..4 to keep `pri_<currency_id>`
// lookups valid.
func (r *Router) ShopPidHandler(c *gin.Context) {
	// Private server uses USD everywhere. The price list is sourced from the
	// embedded client config (tpl_shop_pid) so the client sees real prices.
	rows := gamedata.ShopPidList()
	list := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		list = append(list, gin.H{
			"id":        row.ID,
			"pid":       row.Pid,
			"buy_id":    row.BuyID,
			"avenue_id": row.AvenueID,
			"pri":       row.Pri,
			"pri_int":   row.PriInt,
			"pri_1":     row.Pri1,
			"pri_2":     row.Pri2,
			"pri_3":     row.Pri3,
			"pri_4":     row.Pri4,
			"shop_type": row.ShopType,
			"vip_exp":   row.VipExp,
			"name":      row.Name,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code":          0,
		"currency_id":   1,
		"currency_code": "USD",
		"list":          list,
	})
}

// ShopPaymentMethodHandler returns the available payment methods for the
// user's region. The Lua client never decodes the body; the C# SDK uses it.
// Returning an empty list with code=0 is the safest no-op.
func (r *Router) ShopPaymentMethodHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// grantRewards applies a reward list to the user's DB state: gold
// (item_id=10100001) is added to users.gold, other items are upserted into
// user_items (count column). Returns the same list shaped as item_list for
// handlers to echo back. When gold changes, pushGoldUpdate is scheduled so the
// client's gold display refreshes immediately.
func (r *Router) grantRewards(uid int64, rewards []gamedata.Reward) []gin.H {
	var totalGold int64
	out := make([]gin.H, 0, len(rewards))
	for _, rw := range rewards {
		out = append(out, gin.H{
			"major_type": rw.MajorType,
			"item_id":    rw.ItemID,
			"num":        rw.Num,
		})
		if uid == 0 || rw.Num <= 0 {
			continue
		}
		if rw.ItemID == 10100001 {
			totalGold += int64(rw.Num)
		} else {
			_, _ = r.DB.ExecContext(context.Background(),
				`INSERT INTO user_items (uid, item_id, count) VALUES ($1, $2, $3)
				 ON CONFLICT (uid, item_id) DO UPDATE SET count=user_items.count+$3`,
				uid, rw.ItemID, rw.Num)
		}
	}
	if uid > 0 && totalGold > 0 {
		_, _ = r.DB.ExecContext(context.Background(),
			`UPDATE users SET gold = gold + $2 WHERE uid=$1`, uid, totalGold)
		go func() {
			time.Sleep(200 * time.Millisecond)
			r.pushGoldUpdate(uid)
		}()
	}
	return out
}

// shopRewardsForPid maps a product identifier to the reward item_list the
// client should display. Rewards are sourced from the embedded client config
// (tpl_shop_pid -> tpl_shop_recharge / tpl_monthly_card / tpl_shop_gifts).
// Falls back to a 100k gold payout if the pid is unknown or has no
// configurable rewards, preserving the prior private-server behavior.
//
// As a side effect, the configured vip_exp from tpl_shop_pid is added to the
// user's vip_exp so subsequent /vip/data calls reflect progression. Monthly
// card purchases extend monthly_card_exp (accumulating, not overwriting).
func (r *Router) shopRewardsForPid(uid int64, pid string) []gin.H {
	rewards := gamedata.ShopRewardsByPid(pid)
	if len(rewards) == 0 {
		// Fallback: 100k gold, matching the prior hardcoded behavior.
		if uid > 0 {
			_, _ = r.DB.ExecContext(context.Background(),
				`UPDATE users SET gold = gold + 100000 WHERE uid=$1`, uid)
			go func() {
				time.Sleep(200 * time.Millisecond)
				r.pushGoldUpdate(uid)
			}()
		}
		return []gin.H{{
			"major_type": 1,        // GMajorType.PROP
			"item_id":    10100001, // GPropId.Gold
			"num":        100000,
		}}
	}

	out := r.grantRewards(uid, rewards)

	if uid > 0 {
		// Accumulate vip_exp so VIP level advances per tpl_vip_level thresholds.
		if row, ok := gamedata.ShopPidByPid(pid); ok && row.VipExp > 0 {
			_, _ = r.DB.ExecContext(context.Background(),
				`UPDATE users SET vip_exp = vip_exp + $2 WHERE uid=$1`, uid, row.VipExp)
		}
		// Monthly card: extend expiry timestamp so /player/payInfo reads honestly.
		// Renewals accumulate (GREATEST(current, now) + days) so remaining time
		// is not lost when re-purchasing before expiry.
		if row, ok := gamedata.ShopPidByPid(pid); ok && row.ShopType == 2 {
			if card, ok := gamedata.MonthlyCardByBuyID(row.BuyID); ok && card.Days > 0 {
				now := time.Now().Unix()
				days := int64(card.Days) * 86400
				_, _ = r.DB.ExecContext(context.Background(),
					`UPDATE users
					 SET monthly_card_exp = GREATEST(COALESCE(monthly_card_exp, 0), $2) + $3
					 WHERE uid=$1`,
					uid, now, days)
			}
		}
	}

	return out
}
