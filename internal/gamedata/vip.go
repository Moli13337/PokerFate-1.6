package gamedata

// VIP-related accessors over tpl_vip_level and tpl_vip_exp_path.

// VipLevelRow is a typed view of a tpl_vip_level entry.
type VipLevelRow struct {
	ID              int32   // 0..7
	UpgradeExp      int32   // exp needed to reach the next level
	Icon            string
	Title           interface{} // nil or int (title id)
	Rewards         []interface{}
	DailyGiftCounts int32
	FriendLimit     int32
	BlacklistLimit  int32
	SheetLimit      int32
	VipAdd          int32
	ExpHands        int32
	FriendshipHands int32
	PlanLimit       int32
	ExpTournament   int32
	FriendshipTournament int32
	RecentHistoryLimit int32
}

// toVipLevelRow converts a generic map row into a typed VipLevelRow.
func toVipLevelRow(m map[string]interface{}) VipLevelRow {
	r := VipLevelRow{}
	if v, ok := m["id"]; ok { r.ID, _ = asInt32(v) }
	if v, ok := m["upgrade_exp"]; ok { r.UpgradeExp, _ = asInt32(v) }
	if v, ok := m["icon"]; ok { r.Icon, _ = asString(v) }
	if v, ok := m["title"]; ok { r.Title = v }
	if v, ok := m["rewards"]; ok {
		if arr, ok := v.([]interface{}); ok { r.Rewards = arr }
	}
	if v, ok := m["daily_gift_counts"]; ok { r.DailyGiftCounts, _ = asInt32(v) }
	if v, ok := m["friend_limit"]; ok { r.FriendLimit, _ = asInt32(v) }
	if v, ok := m["blacklist_limit"]; ok { r.BlacklistLimit, _ = asInt32(v) }
	if v, ok := m["sheet_limit"]; ok { r.SheetLimit, _ = asInt32(v) }
	if v, ok := m["vip_add"]; ok { r.VipAdd, _ = asInt32(v) }
	if v, ok := m["exp_hands"]; ok { r.ExpHands, _ = asInt32(v) }
	if v, ok := m["friendship_hands"]; ok { r.FriendshipHands, _ = asInt32(v) }
	if v, ok := m["plan_limit"]; ok { r.PlanLimit, _ = asInt32(v) }
	if v, ok := m["exp_tournament"]; ok { r.ExpTournament, _ = asInt32(v) }
	if v, ok := m["friendship_tournament"]; ok { r.FriendshipTournament, _ = asInt32(v) }
	if v, ok := m["recent_history_limit"]; ok { r.RecentHistoryLimit, _ = asInt32(v) }
	return r
}

// VipLevels returns every VIP level row, sorted by ID ascending (0..7).
func VipLevels() []VipLevelRow {
	t := Get("tpl_vip_level")
	if t == nil {
		return nil
	}
	out := make([]VipLevelRow, 0, len(t.List))
	for _, row := range t.List {
		out = append(out, toVipLevelRow(row))
	}
	return out
}

// VipLevelByExp returns the highest VIP level whose upgrade_exp is <= exp.
// Returns the level-0 row if exp is below the level-1 threshold.
// Returns (zero, false) only if the table is missing.
func VipLevelByExp(exp int) (VipLevelRow, bool) {
	levels := VipLevels()
	if len(levels) == 0 {
		return VipLevelRow{}, false
	}
	current := levels[0]
	for _, lv := range levels {
		if int(lv.UpgradeExp) <= exp {
			current = lv
		} else {
			break
		}
	}
	return current, true
}

// VipLevelByID returns the row for a specific VIP level id (0..7).
func VipLevelByID(id int32) (VipLevelRow, bool) {
	t := Get("tpl_vip_level")
	if t == nil {
		return VipLevelRow{}, false
	}
	if row, ok := t.Map[idToString(id)]; ok {
		return toVipLevelRow(row), true
	}
	return VipLevelRow{}, false
}
