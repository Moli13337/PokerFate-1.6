package gamedata

// Shop-related accessors over tpl_shop_pid, tpl_shop_recharge, tpl_shop_pm_type.

// ShopPidRow is a typed view of a tpl_shop_pid entry.
type ShopPidRow struct {
	ID             int32
	BuyID          int32
	AvenueID       int32
	Pid            string
	Pri            string
	PriInt         int32
	Pri1           string
	Pri2           string
	Pri3           string
	Pri4           string
	OriginalPri    string
	Discount       interface{}
	ShopType       int32
	VipExp         int32
	Name           string
	WebstoreRemove interface{}
}

// toShopPidRow converts a generic map row into a typed ShopPidRow.
func toShopPidRow(m map[string]interface{}) ShopPidRow {
	r := ShopPidRow{}
	if v, ok := m["id"]; ok {
		r.ID, _ = asInt32(v)
	}
	if v, ok := m["buy_id"]; ok {
		r.BuyID, _ = asInt32(v)
	}
	if v, ok := m["avenue_id"]; ok {
		r.AvenueID, _ = asInt32(v)
	}
	if v, ok := m["pid"]; ok {
		r.Pid, _ = asString(v)
	}
	if v, ok := m["pri"]; ok {
		r.Pri, _ = asString(v)
	}
	if v, ok := m["pri_int"]; ok {
		r.PriInt, _ = asInt32(v)
	}
	if v, ok := m["pri_1"]; ok {
		r.Pri1, _ = asString(v)
	}
	if v, ok := m["pri_2"]; ok {
		r.Pri2, _ = asString(v)
	}
	if v, ok := m["pri_3"]; ok {
		r.Pri3, _ = asString(v)
	}
	if v, ok := m["pri_4"]; ok {
		r.Pri4, _ = asString(v)
	}
	if v, ok := m["original_pri"]; ok {
		r.OriginalPri, _ = asString(v)
	}
	if v, ok := m["discount"]; ok {
		r.Discount = v
	}
	if v, ok := m["shop_type"]; ok {
		r.ShopType, _ = asInt32(v)
	}
	if v, ok := m["vip_exp"]; ok {
		r.VipExp, _ = asInt32(v)
	}
	if v, ok := m["name"]; ok {
		r.Name, _ = asString(v)
	}
	if v, ok := m["webstore_remove"]; ok {
		r.WebstoreRemove = v
	}
	return r
}

// ShopPidByPid looks up a single product by its pid string. Returns ok=false if not found.
func ShopPidByPid(pid string) (ShopPidRow, bool) {
	t := Get("tpl_shop_pid")
	if t == nil {
		return ShopPidRow{}, false
	}
	for _, row := range t.List {
		if p, ok := row["pid"].(string); ok && p == pid {
			return toShopPidRow(row), true
		}
	}
	return ShopPidRow{}, false
}

// ShopPidByID looks up a product by its numeric id (first column of tpl_shop_pid).
func ShopPidByID(id int32) (ShopPidRow, bool) {
	t := Get("tpl_shop_pid")
	if t == nil {
		return ShopPidRow{}, false
	}
	if row, ok := t.Map[idToString(id)]; ok {
		return toShopPidRow(row), true
	}
	return ShopPidRow{}, false
}

// ShopPidList returns every product row, in file order.
func ShopPidList() []ShopPidRow {
	t := Get("tpl_shop_pid")
	if t == nil {
		return nil
	}
	out := make([]ShopPidRow, 0, len(t.List))
	for _, row := range t.List {
		out = append(out, toShopPidRow(row))
	}
	return out
}

// ShopPidRawList returns the raw underlying list (shared map rows).
// Callers MUST NOT mutate the returned maps.
func ShopPidRawList() []map[string]interface{} {
	t := Get("tpl_shop_pid")
	if t == nil {
		return nil
	}
	return t.List
}

// ShopRechargeRow is a typed view of a tpl_shop_recharge entry (recharge reward details).
type ShopRechargeRow struct {
	ID       int32
	Pid      string
	ShopType int32
	Gold     int32 // major_type=1, item_id=10100001 (gold) amount
	// Raw kept for callers that need extra fields.
	Raw map[string]interface{}
}

// ShopRechargeByPid returns the recharge reward row for a pid, or ok=false if not found.
func ShopRechargeByPid(pid string) (ShopRechargeRow, bool) {
	t := Get("tpl_shop_recharge")
	if t == nil {
		return ShopRechargeRow{}, false
	}
	for _, row := range t.List {
		if p, ok := row["pid"].(string); ok && p == pid {
			r := ShopRechargeRow{Raw: row}
			if v, ok := row["id"]; ok {
				r.ID, _ = asInt32(v)
			}
			if v, ok := row["pid"]; ok {
				r.Pid, _ = asString(v)
			}
			if v, ok := row["shop_type"]; ok {
				r.ShopType, _ = asInt32(v)
			}
			// tpl_shop_recharge.lua has fields like rewards / item_list / gold; capture generically.
			if v, ok := row["gold"]; ok {
				r.Gold, _ = asInt32(v)
			}
			return r, true
		}
	}
	return ShopRechargeRow{}, false
}

// Reward is a normalized {major_type, item_id, num} tuple matching the
// client's item_list shape returned by shop pay endpoints.
type Reward struct {
	MajorType int32 // always 1 (GMajorType.PROP) for now
	ItemID    int32
	Num       int32
}

// ShopRewardsByPid resolves a product id to its configured reward list by
// consulting tpl_shop_pid and then routing to the appropriate reward table
// based on shop_type:
//
//	shop_type=2         -> tpl_monthly_card.activate_rewards
//	shop_type=4,5       -> tpl_shop_gifts.props
//	shop_type=7,8,9     -> tpl_shop_recharge.props
//
// Returns nil if the pid is unknown or has no configurable rewards; callers
// should apply their own fallback in that case.
func ShopRewardsByPid(pid string) []Reward {
	row, ok := ShopPidByPid(pid)
	if !ok {
		return nil
	}
	switch row.ShopType {
	case 2:
		return rewardsFromMonthlyCard(row.BuyID)
	case 4, 5:
		return rewardsFromShopGifts(row.BuyID)
	case 7, 8, 9:
		return rewardsFromShopRecharge(row.BuyID)
	}
	return nil
}

// rewardsFromMonthlyCard reads activate_rewards from tpl_monthly_card by buy_id.
// activate_rewards is a flat {item_id, count, item_id, count, ...} array.
func rewardsFromMonthlyCard(buyID int32) []Reward {
	t := Get("tpl_monthly_card")
	if t == nil {
		return nil
	}
	for _, row := range t.List {
		if id, ok := asInt32(row["buy_id"]); ok && id == buyID {
			if arr, ok := row["activate_rewards"].([]interface{}); ok {
				return parseFlatRewardArray(arr)
			}
		}
	}
	return nil
}

// rewardsFromShopGifts reads props from tpl_shop_gifts by buy_id.
// props is a flat {item_id, count, ...} array.
func rewardsFromShopGifts(buyID int32) []Reward {
	t := Get("tpl_shop_gifts")
	if t == nil {
		return nil
	}
	for _, row := range t.List {
		if id, ok := asInt32(row["buy_id"]); ok && id == buyID {
			if arr, ok := row["props"].([]interface{}); ok {
				return parseFlatRewardArray(arr)
			}
		}
	}
	return nil
}

// rewardsFromShopRecharge reads props from tpl_shop_recharge by buy_id.
// props is a flat {item_id, count, ...} array.
func rewardsFromShopRecharge(buyID int32) []Reward {
	t := Get("tpl_shop_recharge")
	if t == nil {
		return nil
	}
	for _, row := range t.List {
		if id, ok := asInt32(row["buy_id"]); ok && id == buyID {
			if arr, ok := row["props"].([]interface{}); ok {
				return parseFlatRewardArray(arr)
			}
		}
	}
	return nil
}

// ParseFlatRewards parses {item_id, count, item_id, count, ...} flat arrays
// into a slice of Reward. Exported for use by httpapi (e.g. VIP rewards).
func ParseFlatRewards(arr []interface{}) []Reward {
	return parseFlatRewardArray(arr)
}

// ParseFlatRewardsInt32 is the []int32 variant of ParseFlatRewards, used by
// task templates where Rewards is stored as []int32.
func ParseFlatRewardsInt32(arr []int32) []Reward {
	if len(arr) < 2 || len(arr)%2 != 0 {
		return nil
	}
	out := make([]Reward, 0, len(arr)/2)
	for i := 0; i+1 < len(arr); i += 2 {
		out = append(out, Reward{MajorType: 1, ItemID: arr[i], Num: arr[i+1]})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseFlatRewardArray parses {item_id, count, item_id, count, ...} flat arrays
// into a slice of Reward. Returns nil if the array is empty or malformed.
func parseFlatRewardArray(arr []interface{}) []Reward {
	if len(arr) < 2 || len(arr)%2 != 0 {
		return nil
	}
	out := make([]Reward, 0, len(arr)/2)
	for i := 0; i+1 < len(arr); i += 2 {
		itemID, ok1 := asInt32(arr[i])
		num, ok2 := asInt32(arr[i+1])
		if !ok1 || !ok2 {
			continue
		}
		out = append(out, Reward{MajorType: 1, ItemID: itemID, Num: num})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MonthlyCardRow is a typed view of a tpl_monthly_card entry.
type MonthlyCardRow struct {
	ID              int32
	ShopType        int32
	Days            int32
	ActivateRewards []interface{}
	BuyID           int32
}

// MonthlyCardByBuyID looks up a monthly card product by its buy_id. Returns
// ok=false if the table is missing or the buy_id is not configured.
func MonthlyCardByBuyID(buyID int32) (MonthlyCardRow, bool) {
	t := Get("tpl_monthly_card")
	if t == nil {
		return MonthlyCardRow{}, false
	}
	for _, row := range t.List {
		if id, ok := asInt32(row["buy_id"]); ok && id == buyID {
			r := MonthlyCardRow{BuyID: id}
			r.ID, _ = asInt32(row["id"])
			r.ShopType, _ = asInt32(row["shop_type"])
			r.Days, _ = asInt32(row["days"])
			if arr, ok := row["activate_rewards"].([]interface{}); ok {
				r.ActivateRewards = arr
			}
			return r, true
		}
	}
	return MonthlyCardRow{}, false
}

// idToString converts an int32 id to the same string form used as JSON map key.
func idToString(id int32) string {
	if id == 0 {
		return "0"
	}
	// JSON object keys are always strings; lua_to_json.py uses str(first_col).
	// For ints, str(int) in Python == fmt.Sprintf("%d"). Use base-10 conversion.
	return fmtInt32(id)
}

func fmtInt32(n int32) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
