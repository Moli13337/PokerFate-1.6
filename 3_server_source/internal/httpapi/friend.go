package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Friend module handlers. The private server keeps a real friend graph so the
// social UI (FriendModel.lua) works: list/apply/accept/block/search plus an
// online-aware game list. State lives in the friends table (migration 001/004):
//   - status: 1=pending apply, 2=accepted friend
//   - blocked: per-owner block flag
//   - blocked_until: optional expiry (0 = permanent)

// friendEntry is the shape consumed by FriendModel.lua per list item.
func friendEntry(uid int64, name string, avatar, roleID, skinID int, online bool, mark string) gin.H {
	return gin.H{
		"uid":     uid,
		"name":    name,
		"avatar":  avatar,
		"role_id": roleID,
		"skin_id": skinID,
		"online":  online,
		"mark":    mark,
	}
}

// FriendListHandler returns accepted, non-blocked friends.
func (r *Router) FriendListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)
	list := r.loadFriendEntries(c, uid, `status=2 AND blocked=false`)
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// FriendApplyListHandler returns pending apply requests the user received.
func (r *Router) FriendApplyListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)
	list := r.loadFriendEntriesReverse(c, uid, `status=1`)
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// FriendBlockedListHandler returns friends the user has blocked.
func (r *Router) FriendBlockedListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)
	list := r.loadFriendEntries(c, uid, `blocked=true`)
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// FriendGameListHandler returns accepted friends who are currently online.
func (r *Router) FriendGameListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)
	all := r.loadFriendEntries(c, uid, `status=2 AND blocked=false`)
	online := make([]gin.H, 0, len(all))
	for _, e := range all {
		if e["online"].(bool) {
			online = append(online, e)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": online})
}

// FriendSearchListHandler searches users by name (excluding self).
func (r *Router) FriendSearchListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	keyword := strVal(params, "keyword")
	if keyword == "" {
		keyword = strVal(params, "name")
	}
	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
		return
	}

	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT uid, name, avatar, using_role_id, using_skin_id FROM users
		 WHERE name ILIKE '%' || $1 || '%' AND uid <> $2 LIMIT 20`, keyword, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var fid int64
		var name string
		var avatar, roleID, skinID int
		_ = rows.Scan(&fid, &name, &avatar, &roleID, &skinID)
		list = append(list, friendEntry(fid, name, avatar, roleID, skinID, r.WSSrv.IsOnline(fid), ""))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// FriendApplyHandler creates a pending apply from the user to friend_uid.
// If the reverse edge already exists as pending, auto-accept both sides.
func (r *Router) FriendApplyHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	friendUID := int64Val(params, "friend_uid")
	if uid == 0 || friendUID == 0 || uid == friendUID {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Check if reverse pending exists → auto-accept.
	var reversePending bool
	r.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM friends WHERE uid=$1 AND friend_uid=$2 AND status=1)`,
		friendUID, uid).Scan(&reversePending)

	if reversePending {
		// Accept the reverse edge and create accepted forward edge.
		r.DB.ExecContext(ctx,
			`UPDATE friends SET status=2 WHERE uid=$1 AND friend_uid=$2`,
			friendUID, uid)
		r.DB.ExecContext(ctx,
			`INSERT INTO friends (uid, friend_uid, status, blocked) VALUES ($1, $2, 2, false)
			 ON CONFLICT (uid, friend_uid) DO UPDATE SET status=2`,
			uid, friendUID)
	} else {
		r.DB.ExecContext(ctx,
			`INSERT INTO friends (uid, friend_uid, status, blocked) VALUES ($1, $2, 1, false)
			 ON CONFLICT (uid, friend_uid) DO NOTHING`,
			uid, friendUID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// FriendMarkHandler updates the remark/note for a friend.
func (r *Router) FriendMarkHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	friendUID := int64Val(params, "friend_uid")
	mark := strVal(params, "mark")
	if uid == 0 || friendUID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid"})
		return
	}
	r.DB.ExecContext(context.Background(),
		`UPDATE friends SET mark=$1 WHERE uid=$2 AND friend_uid=$3`, mark, uid, friendUID)
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// FriendDelHandler removes a friend edge.
func (r *Router) FriendDelHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	friendUID := int64Val(params, "friend_uid")
	if uid == 0 || friendUID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid"})
		return
	}
	r.DB.ExecContext(context.Background(),
		`DELETE FROM friends WHERE uid=$1 AND friend_uid=$2`, uid, friendUID)
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// FriendBlockedHandler toggles the block flag on a friend edge.
func (r *Router) FriendBlockedHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	friendUID := int64Val(params, "friend_uid")
	blocked := intVal(params, "blocked") != 0
	if uid == 0 || friendUID == 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid"})
		return
	}
	r.DB.ExecContext(context.Background(),
		`INSERT INTO friends (uid, friend_uid, status, blocked) VALUES ($1, $2, 2, $3)
		 ON CONFLICT (uid, friend_uid) DO UPDATE SET blocked=$3`,
		uid, friendUID, blocked)
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// loadFriendEntries loads friend rows where the user is the owner (uid=$1)
// matching the extra WHERE fragment.
func (r *Router) loadFriendEntries(c *gin.Context, uid int64, where string) []gin.H {
	if uid == 0 {
		return []gin.H{}
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT f.friend_uid, u.name, u.avatar, u.using_role_id, u.using_skin_id, f.mark
		 FROM friends f JOIN users u ON u.uid = f.friend_uid
		 WHERE f.uid=$1 AND `+where+` ORDER BY f.created_at DESC`, uid)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var fid int64
		var name, mark string
		var avatar, roleID, skinID int
		_ = rows.Scan(&fid, &name, &avatar, &roleID, &skinID, &mark)
		list = append(list, friendEntry(fid, name, avatar, roleID, skinID, r.WSSrv.IsOnline(fid), mark))
	}
	return list
}

// loadFriendEntriesReverse loads friend rows where the user is the target
// (friend_uid=$1) — used for the received-apply list.
func (r *Router) loadFriendEntriesReverse(c *gin.Context, uid int64, where string) []gin.H {
	if uid == 0 {
		return []gin.H{}
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT f.uid, u.name, u.avatar, u.using_role_id, u.using_skin_id, f.mark
		 FROM friends f JOIN users u ON u.uid = f.uid
		 WHERE f.friend_uid=$1 AND `+where+` ORDER BY f.created_at DESC`, uid)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var fid int64
		var name, mark string
		var avatar, roleID, skinID int
		_ = rows.Scan(&fid, &name, &avatar, &roleID, &skinID, &mark)
		list = append(list, friendEntry(fid, name, avatar, roleID, skinID, r.WSSrv.IsOnline(fid), mark))
	}
	return list
}
