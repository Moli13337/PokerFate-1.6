package ws

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/lib/pq"

	"poker-fate-server/internal/gamedata"
)

// Static game data for the private server. Roles, skins and items are now
// sourced from the embedded client config (see internal/gamedata). Only a few
// gameplay-only constants that are not part of the client config tables
// remain here.

const (
	maxRoleLevel        int32 = 99
	maxBondExp          int32 = 999999
	tutorialSkipStep    int   = 9999
	defaultLobbySceneID int32 = 20900001
	defaultLobbyBgmID   int32 = 20700001
	defaultAvatarID     int32 = 20110101
)

// defaultRoleID returns the default character id from tpl_constdata
// (DefaultCharacter), falling back to 1001 if the table is missing.
func defaultRoleID() int32 {
	return gamedata.ConstInt("DefaultCharacter", 1001)
}

// defaultSkinID returns the first skin of the default character from
// tpl_character_skin, falling back to 100101 if unavailable.
func defaultSkinID() int32 {
	roleID := defaultRoleID()
	if skins := gamedata.SkinsForRole(roleID); len(skins) > 0 {
		return skins[0]
	}
	return 100101
}

const itemGrantCount = 9999

// AllSkinIDs returns every skin id across the full character roster. Used by
// the gacha handler to pick a random skin as a draw reward.
func AllSkinIDs() []int32 {
	return gamedata.AllSkinIDs()
}

// AllRoleIDs returns every playable character id.
func AllRoleIDs() []int32 {
	return gamedata.AllRoleIDs()
}

// allRolesWithSkins returns every character paired with its owned skin ids,
// with the default (kind=0) skin first. Used by HandleRoleListREQ to build
// the full RoleInfo list.
func allRolesWithSkins() []gamedata.CharacterWithSkins {
	return gamedata.AllRolesWithSkins()
}

// allItemIDs returns every item id from tpl_props. Used by HandleItemListREQ
// to populate OwnedItemIdList.
func allItemIDs() []int32 {
	return gamedata.AllItemIDs()
}

// InitNewAccount seed a freshly created user with the full private-server
// entitlement: every character (max bond, starred, awakened), every item, and
// a welcome mail. It is best-effort; partial failures only log a warning.
func InitNewAccount(db *sql.DB, uid int64) {
	ctx := context.Background()

	// Roles: insert all characters with full skins, max bond, awakened.
	for _, r := range allRolesWithSkins() {
		if len(r.Skins) == 0 {
			continue
		}
		skins := make([]int, len(r.Skins))
		for i, s := range r.Skins {
			skins[i] = int(s)
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO user_roles (uid, role_id, star, bond, awakened, skins, using_skin)
			 VALUES ($1, $2, true, $3, true, $4, $5)
			 ON CONFLICT (uid, role_id) DO UPDATE SET star=true, bond=$3, awakened=true, skins=$4, using_skin=$5`,
			uid, r.RoleID, maxBondExp, pq.Array(skins), r.Skins[0])
		if err != nil {
			fmt.Printf("init role %d for uid %d: %v\n", r.RoleID, uid, err)
		}
	}

	// Items: grant the full catalogue.
	for _, itemID := range gamedata.AllItemIDs() {
		_, err := db.ExecContext(ctx,
			`INSERT INTO user_items (uid, item_id, count) VALUES ($1, $2, $3)
			 ON CONFLICT (uid, item_id) DO UPDATE SET count=$3`,
			uid, itemID, itemGrantCount)
		if err != nil {
			fmt.Printf("init item %d for uid %d: %v\n", itemID, uid, err)
		}
	}

	// Set default role/skin, bump gold, and mark the tutorial as complete.
	roleID := defaultRoleID()
	skinID := defaultSkinID()
	_, err := db.ExecContext(ctx,
		`UPDATE users SET using_role_id=$1, using_skin_id=$2, gold=gold+$3, newer_guide_step=$4 WHERE uid=$5`,
		roleID, skinID, int64(999000000), tutorialSkipStep, uid)
	if err != nil {
		fmt.Printf("set defaults for uid %d: %v\n", uid, err)
	}

	// Welcome mail with a summary of granted rewards.
	rewards, _ := json.Marshal([]map[string]interface{}{
		{"item_id": 10100001, "num": 999000000},
		{"item_id": 10200001, "num": itemGrantCount},
		{"item_id": 10200002, "num": itemGrantCount},
	})
	_, err = db.ExecContext(ctx,
		`INSERT INTO mails (uid, type, title, content, rewards, is_read, is_received)
		 VALUES ($1, 1, 'Welcome to Poker Fate Private Server',
		         'All characters, skins and items have been unlocked. Enjoy!', $2, false, false)`,
		uid, string(rewards))
	if err != nil {
		fmt.Printf("insert welcome mail for uid %d: %v\n", uid, err)
	}
}
