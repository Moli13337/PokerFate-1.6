package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"poker-fate-server/internal/gamedata"
	"poker-fate-server/internal/model"
)

// Task module handlers. Covers the daily/weekly/challenge cycle, the seven-day
// newbie chapter progression, theme/festival/activity task lists and the
// achievement system. All state is persisted in the user_tasks /
// user_task_points / user_seven_day_progress / user_achievement_progress tables
// introduced by migration 002.
//
// Task instance shape consumed by TaskModel.lua:
//   {id, task_id, status(1/2/3), value[], current_value, sort, monthly_card_task}
// status: 1=in progress, 2=completed (claimable), 3=reward received.

// TaskListHandler returns the daily/weekly/challenge task lists plus the active
// point totals. TaskModel.lua iterates the three lists and reads point_data
// unconditionally, so empty arrays and the point object are mandatory.
func (r *Router) TaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	daily := r.loadTaskList(uid, model.TaskCateDaily)
	weekly := r.loadTaskList(uid, model.TaskCateWeekly)
	challenge := r.loadTaskList(uid, model.TaskCateChallenge)

	c.JSON(http.StatusOK, gin.H{
		"code":                      0,
		"daily_list":                daily,
		"weekly_list":               weekly,
		"challenge_list":            challenge,
		"point_data":                r.loadPointData(uid),
		"daily_point_conf_list":     pointConfList("tpl_dailychest"),
		"weekly_point_conf_list":    pointConfList("tpl_weeklychest"),
		"challenge_point_conf_list": pointConfList("tpl_challengechest"),
	})
}

// pointConfList returns the active-point reward configuration (milestone chests)
// from gamedata. TaskModel.lua:201-203 does `pointData[maxId].id` without nil-
// checking, so an empty array causes "attempt to index nil value" on claim.
// The tpl_*chest tables provide {id, reward_point, rewards_list, sort} rows.
func pointConfList(table string) interface{} {
	t := gamedata.Get(table)
	if t == nil || len(t.List) == 0 {
		return []interface{}{}
	}
	return t.List
}

// TaskReportHandler accepts a client-side progress update for a task. The
// private server persists the reported current_value and flips status to
// completed when a target is reached (target supplied by the client as `value`).
func (r *Router) TaskReportHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	taskCate := intVal(params, "task_cate")
	taskID := intVal(params, "task_id")
	currentValue := intVal(params, "current_value")

	if uid > 0 && taskCate > 0 && taskID > 0 {
		r.DB.ExecContext(context.Background(),
			`INSERT INTO user_tasks (uid, task_cate, task_id, current_value, updated_at)
			 VALUES ($1, $2, $3, $4, NOW())
			 ON CONFLICT (uid, task_cate, task_id)
			 DO UPDATE SET current_value=$4, updated_at=NOW()`,
			uid, taskCate, taskID, currentValue)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// TaskRecRewardHandler marks task rewards as received (status=3) and returns
// the aggregated reward payload. The client (TaskModel.lua:333 receiveTaskReward)
// sends {task_cate, id_arr:[taskID...], all} and expects {reward_chips,
// reward_point, item_list} in the response to build the reward UI.
func (r *Router) TaskRecRewardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	taskCate := intVal(params, "task_cate")
	idArr := toIntSlice(params, "id_arr")

	var totalChips, totalPoint int
	itemList := make([]gin.H, 0)

	if uid > 0 && taskCate > 0 && len(idArr) > 0 {
		for _, taskID := range idArr {
			if taskID <= 0 {
				continue
			}
			// Upsert status=3 (reward claimed). Handles gamedata-fallback tasks
			// that were never persisted in user_tasks.
			_, _ = r.DB.ExecContext(context.Background(),
				`INSERT INTO user_tasks (uid, task_cate, task_id, status, current_value, updated_at)
				 VALUES ($1, $2, $3, 3, 0, NOW())
				 ON CONFLICT (uid, task_cate, task_id)
				 DO UPDATE SET status=3, updated_at=NOW()`,
				uid, taskCate, taskID)

			// Look up rewards from gamedata template.
			tpl, ok := findTaskTemplate(taskCate, taskID)
			if !ok {
				continue
			}
			totalPoint += int(tpl.RewardPoint)
			rewards := gamedata.ParseFlatRewardsInt32(tpl.Rewards)
			for _, rw := range rewards {
				if rw.ItemID == 10100001 && rw.Num > 0 {
					totalChips += int(rw.Num)
				} else {
					itemList = append(itemList, gin.H{
						"major_type": rw.MajorType,
						"item_id":    rw.ItemID,
						"num":        rw.Num,
					})
				}
			}
		}
		// Apply gold to DB.
		if totalChips > 0 {
			_, _ = r.DB.ExecContext(context.Background(),
				`UPDATE users SET gold = gold + $2 WHERE uid=$1`, uid, totalChips)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":         0,
		"reward_chips": totalChips,
		"reward_point": totalPoint,
		"item_list":    itemList,
	})
	if totalChips > 0 {
		go func() {
			time.Sleep(200 * time.Millisecond)
			r.pushGoldUpdate(uid)
		}()
	}
}

// TaskRecPointRewardHandler records a claimed point-reward id so it cannot be
// claimed again. The reward itself is granted client-side from static config.
func (r *Router) TaskRecPointRewardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	taskCate := intVal(params, "task_cate")
	rewardID := intVal(params, "reward_id")

	if uid > 0 && taskCate > 0 && rewardID > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE user_task_points
			 SET claimed_reward_ids = claimed_reward_ids || to_jsonb($3::int),
			     updated_at = NOW()
			 WHERE uid=$1 AND task_cate=$2`,
			uid, taskCate, rewardID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// TaskRecChapterRewardHandler records a claimed seven-day chapter reward and
// returns the reward payload. The client (SevenDayTaskModel:315
// receiveSevenDayChapterReward) sends a nil body and expects data.item_list
// in the response; if item_list is absent the closeCb never calls
// requestSevenDayTaskList, leaving _showCertificationAnim=true and the UI in
// a stale state that causes overlapping dialogs.
//
// State transitions skip the Rewarded(2) state entirely for chapters 1-6:
// the client's setChapterInfo creates a per-second _leftTimeTag scheduler
// whenever getChapterStatus returns Completed (which happens whenever the
// overall status != Running), and preHide never removes that scheduler — so
// closing the view leaves it ticking against destroyed UI nodes and freezes
// the client. To avoid that, we advance cur_day and reset status to Running
// in the same request. Chapter 7 has no "next chapter" so it goes to
// Completed(3), which the client handles via the CertificateIcon click flow
// (no _leftTimeTag is created because CertificationButton is hidden).
func (r *Router) TaskRecChapterRewardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	chapter := intVal(params, "chapter")
	if chapter == 0 {
		chapter = intVal(params, "day")
	}

	// Client sends nil body; fall back to the DB cur_day.
	if chapter == 0 && uid > 0 {
		_ = r.DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(cur_day,1) FROM user_seven_day_progress WHERE uid=$1`, uid,
		).Scan(&chapter)
		if chapter == 0 {
			chapter = 1
		}
	}

	if uid == 0 || chapter == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "item_list": []interface{}{}})
		return
	}

	_, _ = r.DB.ExecContext(context.Background(),
		`INSERT INTO user_seven_day_progress (uid, chapter_rewards_claimed, updated_at)
		 VALUES ($1, to_jsonb(ARRAY[$2::int]), NOW())
		 ON CONFLICT (uid) DO UPDATE
		 SET chapter_rewards_claimed =
		       user_seven_day_progress.chapter_rewards_claimed || to_jsonb($2::int),
		     updated_at = NOW()`,
		uid, chapter)

	// Advance state. Chapter 7 -> Completed(3). Chapters 1-6 advance
	// cur_day to chapter+1 and reset status to Running(1), skipping the
	// Rewarded(2) state that triggers the client's leaky _leftTimeTag
	// scheduler (see comment above). cur_day is clamped to 7 and only moves
	// forward to avoid regressions on duplicate claims.
	if chapter >= 7 {
		_, _ = r.DB.ExecContext(context.Background(),
			`UPDATE user_seven_day_progress SET status=3, updated_at=NOW() WHERE uid=$1`,
			uid)
	} else {
		_, _ = r.DB.ExecContext(context.Background(),
			`UPDATE user_seven_day_progress
			 SET cur_day = LEAST(GREATEST(cur_day, $2 + 1), 7),
			     status = 1,
			     updated_at = NOW()
			 WHERE uid=$1`,
			uid, chapter)
	}

	// Build item_list from gamedata chapter_rewards.
	rewards := sevenDayChapterRewards(chapter)
	itemList := make([]gin.H, 0, len(rewards))
	var totalChips int64
	for _, rw := range rewards {
		if rw.ItemID == 10100001 && rw.Num > 0 {
			totalChips += int64(rw.Num)
		} else {
			itemList = append(itemList, gin.H{
				"major_type": rw.MajorType,
				"item_id":    rw.ItemID,
				"num":        rw.Num,
			})
		}
	}
	if totalChips > 0 {
		_, _ = r.DB.ExecContext(context.Background(),
			`UPDATE users SET gold = gold + $2 WHERE uid=$1`, uid, totalChips)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"item_list": itemList,
	})
	if totalChips > 0 {
		go func() {
			time.Sleep(200 * time.Millisecond)
			r.pushGoldUpdate(uid)
		}()
	}
}

// TaskOpenNextChapterHandler advances the seven-day chapter cursor. The client
// (SevenDayTaskModel:332 requestOpenNextChapter) only calls this when status !=
// Running, so we must reset status to Running(1) on conflict, otherwise the
// monthly-card auto-unlock in requestSevenDayTaskList would loop forever.
func (r *Router) TaskOpenNextChapterHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	if uid > 0 {
		r.DB.ExecContext(context.Background(),
			`INSERT INTO user_seven_day_progress (uid, cur_day, status, updated_at)
			 VALUES ($1, 2, 1, NOW())
			 ON CONFLICT (uid) DO UPDATE
			 SET cur_day = LEAST(user_seven_day_progress.cur_day + 1, 7),
			     status = 1,
			     updated_at = NOW()`,
			uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// sevenDayChapterRewards returns the chapter_rewards list for a given chapter
// id from tpl_seven_day_tasks_stage gamedata.
func sevenDayChapterRewards(chapter int) []gamedata.Reward {
	stages := gamedata.SevenDayStages()
	for _, s := range stages {
		if id, ok := s["id"].(float64); ok && int(id) == chapter {
			if rewards, ok := s["chapter_rewards"].([]interface{}); ok {
				return gamedata.ParseFlatRewards(rewards)
			}
		}
	}
	return nil
}

// TaskUploadAuthCertHandler accepts a base64-encoded signature image from the
// seven-day identity-verification step. The client (SevenDayTaskModel:343
// sendCertificationSign) sends {body: base64img, suffix: "png"} and expects
// data.url in the response to download and display the signature on the
// certificate UI. We persist the image to ./uploads/, return its URL, and
// upsert both user_seven_day_progress (status=4 Upload) and users (so
// SelfUserInfoRSP can read auth_cert_url back).
func (r *Router) TaskUploadAuthCertHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	bodyStr := strVal(params, "body")
	suffix := strVal(params, "suffix")
	if suffix == "" {
		suffix = "png"
	}

	r.Logger.Info("TaskUploadAuthCertHandler",
		zap.Int64("uid", uid),
		zap.Int("body_len", len(bodyStr)),
		zap.String("suffix", suffix),
		zap.String("host", c.Request.Host))

	if uid == 0 || bodyStr == "" {
		r.Logger.Warn("TaskUploadAuthCert: empty uid or body",
			zap.Int64("uid", uid),
			zap.Int("body_len", len(bodyStr)))
		c.JSON(http.StatusOK, gin.H{"code": 0, "url": ""})
		return
	}

	// Clean the base64 input: strip whitespace, data URL prefix, and fix padding.
	// The client sometimes sends base64 with trailing newlines or a data: prefix,
	// and 41441 % 4 = 1 indicates extra characters that break StdEncoding.
	cleaned := strings.TrimSpace(bodyStr)
	cleaned = strings.TrimPrefix(cleaned, "data:image/png;base64,")
	cleaned = strings.TrimPrefix(cleaned, "data:image/jpeg;base64,")
	cleaned = strings.TrimPrefix(cleaned, "data:image/jpg;base64,")
	cleaned = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, cleaned)
	if pad := len(cleaned) % 4; pad != 0 {
		cleaned += strings.Repeat("=", 4-pad)
	}

	imgData, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		// Fall back to RawStdEncoding (no padding) then URLEncoding as last resort.
		imgData, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(cleaned, "="))
	}
	if err != nil {
		r.Logger.Warn("TaskUploadAuthCert: base64 decode failed",
			zap.Int64("uid", uid),
			zap.Int("body_len", len(bodyStr)),
			zap.Int("cleaned_len", len(cleaned)),
			zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"code": -1, "url": ""})
		return
	}

	filename := fmt.Sprintf("auth_cert_%d_%d.%s", uid, time.Now().Unix(), suffix)
	uploadsDir := "uploads"
	_ = os.MkdirAll(uploadsDir, 0755)
	if err := os.WriteFile(filepath.Join(uploadsDir, filename), imgData, 0644); err != nil {
		r.Logger.Warn("TaskUploadAuthCert: write file failed",
			zap.Int64("uid", uid),
			zap.String("filename", filename),
			zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"code": -1, "url": ""})
		return
	}

	url := fmt.Sprintf("http://%s/uploads/%s", c.Request.Host, filename)
	now := time.Now().Unix()

	_, _ = r.DB.ExecContext(context.Background(),
		`INSERT INTO user_seven_day_progress (uid, auth_cert_url, auth_cert_time, status, updated_at)
		 VALUES ($1, $2, $3, 4, NOW())
		 ON CONFLICT (uid) DO UPDATE
		 SET auth_cert_url=$2, auth_cert_time=$3, status=4, updated_at=NOW()`,
		uid, url, now)

	_, _ = r.DB.ExecContext(context.Background(),
		`UPDATE users SET auth_cert_url=$1, auth_cert_time=$2 WHERE uid=$3`,
		url, now, uid)

	r.Logger.Info("TaskUploadAuthCert: success",
		zap.Int64("uid", uid),
		zap.String("url", url),
		zap.Int("img_size", len(imgData)))

	c.JSON(http.StatusOK, gin.H{"code": 0, "url": url})
}

// SevenTaskConfHandler returns the seven-day task stage config. The stage
// definitions live in the client's static tpl_seven_day config, so the server
// only needs to expose the per-user unlock state. SevenDayTaskModel:34 iterates
// data.seven_day_tasks_stage unconditionally.
func (r *Router) SevenTaskConfHandler(c *gin.Context) {
	stages := gamedata.SevenDayStages()
	if stages == nil {
		stages = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{
		"code":                  0,
		"seven_day_tasks_stage": stages,
	})
}

// SevenTaskListHandler returns the seven-day task progress. SevenDayTaskModel:73
// iterates data.list; data.cur_day/status drive the chapter UI.
func (r *Router) SevenTaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	curDay, status := 1, 1
	if uid > 0 {
		r.DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(cur_day,1), COALESCE(status,1) FROM user_seven_day_progress WHERE uid=$1`,
			uid).Scan(&curDay, &status)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"cur_day": curDay,
			"status":  status,
		},
		"list": r.loadTaskList(uid, model.TaskCateSevenDay),
	})
}

// FestivalTaskListHandler returns the festival-activity task list.
func (r *Router) FestivalTaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"list": r.loadTaskList(uid, model.TaskCateFestival),
	})
}

// ActivityTaskListHandler returns the theme-activity task list.
func (r *Router) ActivityTaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"list": r.loadTaskList(uid, model.TaskCateTheme),
	})
}

// AchTaskListHandler returns the achievement list for a theme. AchievementModel
// iterates data.list and reads each entry's id/status/current_value/rate/finish.
// It also reads data.ids_arr (theme IDs whose theme reward has been claimed).
func (r *Router) AchTaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	themeID := intVal(params, "theme_id")

	list := r.loadAchList(uid, themeID)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"list":    list,
		"ids_arr": r.loadClaimedThemeIDs(uid),
	})
}

// AchTaskCountHandler returns the total/cleared achievement counts.
// AchievementModel:requestAchTaskClearCount reads:
//   - data.list       -> refreshLobbyRedPoint: each entry needs {ach=theme_id, count}
//   - data.level_list -> clearDic: each entry needs {ach=ach_level(1/2/3), count}
func (r *Router) AchTaskCountHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	var total, cleared int
	themeList := make([]gin.H, 0)
	if uid > 0 {
		r.DB.QueryRowContext(context.Background(),
			`SELECT COUNT(*), COUNT(*) FILTER (WHERE finish=true)
		 FROM user_achievement_progress WHERE uid=$1`, uid).Scan(&total, &cleared)

		// Per-theme counts (ach = theme_id: 101/201/301/401/501)
		rows, _ := r.DB.QueryContext(context.Background(),
			`SELECT theme_id, COUNT(*) FILTER (WHERE finish=true)
			 FROM user_achievement_progress WHERE uid=$1 GROUP BY theme_id ORDER BY theme_id`, uid)
		if rows != nil {
			for rows.Next() {
				var tid, cnt int
				_ = rows.Scan(&tid, &cnt)
				themeList = append(themeList, gin.H{"ach": tid, "count": cnt})
			}
			rows.Close()
		}
	}

	// When DB has no achievement rows, use gamedata template totals.
	achByLevel := gamedata.AchievementCountByLevel()
	if total == 0 {
		for _, cnt := range achByLevel {
			total += cnt
		}
		cleared = total
		// Build per-theme list from gamedata.
		themeCounts := make(map[int32]int)
		for _, t := range gamedata.AchievementTasks() {
			themeCounts[t.AchType]++
		}
		for tid, cnt := range themeCounts {
			themeList = append(themeList, gin.H{"ach": tid, "count": cnt})
		}
	}

	// Per-level counts (ach = ach_level: 1/2/3) from tpl_achievement_task.
	levelList := []gin.H{
		{"ach": 1, "count": achByLevel[1]},
		{"ach": 2, "count": achByLevel[2]},
		{"ach": 3, "count": achByLevel[3]},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":       0,
		"total":      total,
		"cleared":    cleared,
		"list":       themeList,
		"level_list": levelList,
	})
}

// RecentlyAchTaskListHandler returns the most recently completed achievements.
func (r *Router) RecentlyAchTaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT task_id, status, current_value, rate
		 FROM user_achievement_progress WHERE uid=$1 AND finish=true
		 ORDER BY updated_at DESC LIMIT 20`, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var taskID, status, currentValue, rate int
		_ = rows.Scan(&taskID, &status, &currentValue, &rate)
		list = append(list, gin.H{
			"id":            taskID,
			"task_id":       taskID,
			"status":        status,
			"current_value": currentValue,
			"rate":          rate,
			"finish":        1,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// RecAchRewardHandler handles achievement reward claims. The client sends
// either:
//
//	{ids_arr: [themeId]}     -> getThemeRewared: claiming a theme reward
//	{task_id_arr: [achId]}   -> getAchievementReward: claiming individual reward
//
// Both paths expect data.item_list in the response (not nil).
func (r *Router) RecAchRewardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	idsArr := toIntSlice(params, "ids_arr")
	taskIDArr := toIntSlice(params, "task_id_arr")

	if uid > 0 {
		// Theme reward claim - record the theme as claimed
		for _, themeID := range idsArr {
			if themeID > 0 {
				r.DB.ExecContext(context.Background(),
					`INSERT INTO user_achievement_meta (uid, theme_id, claimed_theme_reward_ids, updated_at)
					 VALUES ($1, $2, '[]'::jsonb, NOW())
					 ON CONFLICT (uid, theme_id) DO UPDATE SET updated_at = NOW()`,
					uid, themeID)
			}
		}
		// Individual achievement reward claim - mark status=3 (reward claimed)
		for _, taskID := range taskIDArr {
			if taskID > 0 {
				r.DB.ExecContext(context.Background(),
					`UPDATE user_achievement_progress SET status=3, updated_at=NOW()
					 WHERE uid=$1 AND task_id=$2`,
					uid, taskID)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"item_list": []interface{}{},
	})
}

// RoleTaskListHandler returns the per-role task list.
func (r *Router) RoleTaskListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	roleID := intVal(params, "role_id")

	list := r.loadRoleTaskList(uid, roleID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// RecRoleTaskRwHandler marks a role-task reward as received.
func (r *Router) RecRoleTaskRwHandler(c *gin.Context) {
	r.TaskRecRewardHandler(c)
}

// --- helpers ---

// loadTaskList returns task instances for one category. Merges gamedata
// templates (status=2 claimable) with DB rows so that claiming one task does
// not make the others disappear. DB rows override gamedata entries for the
// same task_id; DB-only tasks (not in gamedata) are appended.
func (r *Router) loadTaskList(uid int64, taskCate int) []gin.H {
	if uid == 0 {
		return []gin.H{}
	}

	// Start with gamedata templates as the base list (status=2 claimable).
	list := loadTaskListFromGamedata(taskCate)

	// Build a task_id -> index map for O(1) override lookups.
	taskIdx := make(map[int32]int, len(list))
	for i, t := range list {
		if tid, ok := t["task_id"].(int32); ok {
			taskIdx[tid] = i
		}
	}

	// Override with DB rows (status=3 claimed, status=1 in-progress, etc.).
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT id, task_id, status, current_value, target_values::TEXT, sort, monthly_card_task
		 FROM user_tasks WHERE uid=$1 AND task_cate=$2 ORDER BY sort, task_id`, uid, taskCate)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var taskID, status, currentValue, sortVal int
			var targets []byte
			var monthlyCard bool
			_ = rows.Scan(&id, &taskID, &status, &currentValue, &targets, &sortVal, &monthlyCard)

			monthlyVal := 0
			if monthlyCard {
				monthlyVal = 1
			}
			valueSlice := jsonRawList(targets)
			// Fall back to gamedata template value when DB target_values is empty
			// (happens when rows were inserted by TaskRecReward/TaskReport without
			// setting target_values — the column defaults to '[]'::jsonb).
			if isEmptyList(valueSlice) {
				if idx, ok := taskIdx[int32(taskID)]; ok {
					if gv, ok := list[idx]["value"].([]int32); ok && len(gv) > 0 {
						valueSlice = gv
					}
				}
			}
			entry := gin.H{
				"id":                id,
				"task_id":           taskID,
				"status":            status,
				"value":             valueSlice,
				"current_value":     currentValue,
				"sort":              sortVal,
				"monthly_card_task": monthlyVal,
			}
			if idx, ok := taskIdx[int32(taskID)]; ok {
				list[idx] = entry
			} else {
				list = append(list, entry)
			}
		}
	}
	return list
}

// loadTaskListFromGamedata builds task instances from gamedata templates with
// status=2 (completed/claimable), used when the DB has no user_tasks rows.
func loadTaskListFromGamedata(taskCate int) []gin.H {
	var templates []gamedata.TaskTemplateRow
	switch taskCate {
	case model.TaskCateDaily:
		templates = gamedata.DailyTasks()
	case model.TaskCateWeekly:
		templates = gamedata.WeeklyTasks()
	case model.TaskCateChallenge:
		templates = gamedata.ChallengeTasks()
	case model.TaskCateSevenDay:
		templates = gamedata.SevenDayTasks()
	case model.TaskCateTheme:
		templates = gamedata.ThemeTasks()
	case model.TaskCateFestival:
		templates = gamedata.FestivalTasks()
	default:
		return []gin.H{}
	}
	list := make([]gin.H, 0, len(templates))
	for _, t := range templates {
		curVal := int32(0)
		if len(t.Value) > 0 {
			curVal = t.Value[0]
		}
		valSlice := make([]int32, len(t.Value))
		copy(valSlice, t.Value)
		list = append(list, gin.H{
			"id":                t.TaskID,
			"task_id":           t.TaskID,
			"status":            2,
			"value":             valSlice,
			"current_value":     curVal,
			"sort":              t.Sort,
			"monthly_card_task": 0,
		})
	}
	return list
}

// loadRoleTaskList returns role-task instances filtered by role_id.
func (r *Router) loadRoleTaskList(uid int64, roleID int) []gin.H {
	if uid == 0 {
		return []gin.H{}
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT id, task_id, status, current_value, target_values::TEXT, sort
		 FROM user_tasks WHERE uid=$1 AND task_cate=$2 AND role_id=$3 ORDER BY sort, task_id`,
		uid, model.TaskCateRole, roleID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var taskID, status, currentValue, sort int
		var targets []byte
		_ = rows.Scan(&id, &taskID, &status, &currentValue, &targets, &sort)
		list = append(list, gin.H{
			"id":            id,
			"task_id":       taskID,
			"status":        status,
			"value":         jsonRawList(targets),
			"current_value": currentValue,
			"sort":          sort,
		})
	}
	return list
}

// loadAchList returns achievement progress rows for a theme. theme_id filter
// was added in migration 004; themeID<=0 returns all (backward compat).
// When the DB has no rows, falls back to gamedata templates with status=2.
func (r *Router) loadAchList(uid int64, themeID int) []gin.H {
	if uid == 0 {
		return []gin.H{}
	}
	var rows *sql.Rows
	var err error
	if themeID > 0 {
		rows, err = r.DB.QueryContext(context.Background(),
			`SELECT task_id, status, current_value, rate, finish, theme_id
			 FROM user_achievement_progress WHERE uid=$1 AND theme_id=$2 ORDER BY task_id`,
			uid, themeID)
	} else {
		rows, err = r.DB.QueryContext(context.Background(),
			`SELECT task_id, status, current_value, rate, finish, theme_id
			 FROM user_achievement_progress WHERE uid=$1 ORDER BY task_id`, uid)
	}
	if err != nil {
		return loadAchListFromGamedata(themeID)
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var taskID, status, currentValue, rate, dbThemeID int
		var finish bool
		_ = rows.Scan(&taskID, &status, &currentValue, &rate, &finish, &dbThemeID)
		finishVal := 0
		if finish {
			finishVal = 1
		}
		list = append(list, gin.H{
			"id":            taskID,
			"task_id":       taskID,
			"status":        status,
			"current_value": currentValue,
			"rate":          rate,
			"finish":        finishVal,
			"theme_id":      dbThemeID,
		})
	}
	if len(list) > 0 {
		return list
	}
	return loadAchListFromGamedata(themeID)
}

// loadAchListFromGamedata builds achievement instances from gamedata templates
// with status=2 (completed/claimable), used when DB has no achievement rows.
func loadAchListFromGamedata(themeID int) []gin.H {
	templates := gamedata.AchievementTasks()
	list := make([]gin.H, 0)
	for _, t := range templates {
		if themeID > 0 && t.AchType != int32(themeID) {
			continue
		}
		curVal := int32(0)
		if len(t.Value) > 0 {
			curVal = t.Value[0]
		}
		list = append(list, gin.H{
			"id":            t.TaskID,
			"task_id":       t.TaskID,
			"status":        2,
			"current_value": curVal,
			"rate":          100,
			"finish":        1,
			"theme_id":      t.AchType,
		})
	}
	return list
}

// loadClaimedThemeIDs returns the list of theme_ids whose theme reward has been
// claimed by the user. AchievementModel reads data.ids_arr to suppress
// "theme reward available" red dots.
func (r *Router) loadClaimedThemeIDs(uid int64) []int {
	if uid == 0 {
		return []int{}
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT theme_id FROM user_achievement_meta WHERE uid=$1 ORDER BY theme_id`, uid)
	if err != nil {
		return []int{}
	}
	defer rows.Close()
	out := make([]int, 0)
	for rows.Next() {
		var tid int
		_ = rows.Scan(&tid)
		out = append(out, tid)
	}
	return out
}

// toIntSlice extracts an int slice from a params map. JSON arrays decode as
// []interface{} with float64 elements.
func toIntSlice(m map[string]interface{}, key string) []int {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]int, 0, len(arr))
	for _, v := range arr {
		switch n := v.(type) {
		case float64:
			out = append(out, int(n))
		case int:
			out = append(out, n)
		}
	}
	return out
}

// loadPointData returns the active-point totals for the three cycle categories.
func (r *Router) loadPointData(uid int64) gin.H {
	out := gin.H{"daily_point": 0, "weekly_point": 0, "challenge_point": 0}
	if uid == 0 {
		return out
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT task_cate, point FROM user_task_points WHERE uid=$1`, uid)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var cate, point int
		_ = rows.Scan(&cate, &point)
		switch cate {
		case model.TaskCateDaily:
			out["daily_point"] = point
		case model.TaskCateWeekly:
			out["weekly_point"] = point
		case model.TaskCateChallenge:
			out["challenge_point"] = point
		}
	}
	return out
}

// findTaskTemplate looks up a task template by category and task_id. Used by
// TaskRecRewardHandler to resolve the reward payload from gamedata.
func findTaskTemplate(taskCate int, taskID int) (gamedata.TaskTemplateRow, bool) {
	var templates []gamedata.TaskTemplateRow
	switch taskCate {
	case model.TaskCateDaily:
		templates = gamedata.DailyTasks()
	case model.TaskCateWeekly:
		templates = gamedata.WeeklyTasks()
	case model.TaskCateChallenge:
		templates = gamedata.ChallengeTasks()
	case model.TaskCateSevenDay:
		templates = gamedata.SevenDayTasks()
	case model.TaskCateTheme:
		templates = gamedata.ThemeTasks()
	case model.TaskCateFestival:
		templates = gamedata.FestivalTasks()
	default:
		return gamedata.TaskTemplateRow{}, false
	}
	for _, t := range templates {
		if int(t.TaskID) == taskID {
			return t, true
		}
	}
	return gamedata.TaskTemplateRow{}, false
}
