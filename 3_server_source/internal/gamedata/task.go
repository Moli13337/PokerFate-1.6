package gamedata

// Task template accessors over tpl_*tasks JSON files.

// TaskTemplateRow is a typed view of a tpl_*tasks entry.
type TaskTemplateRow struct {
	TaskID      int32
	TaskType    int32
	AddType     int32
	Value       []int32
	Rewards     []int32
	RewardPoint int32
	Dec         string
	Jump        int32
	Sort        int32
	Group       int32
	AchType     int32 // achievement only
	AchLevel    int32 // achievement only
}

func parseInt32List(v interface{}) []int32 {
	arr, ok := v.([]interface{})
	if ok {
		out := make([]int32, 0, len(arr))
		for _, e := range arr {
			if n, ok := asInt32(e); ok {
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

func parseTaskTemplate(row map[string]interface{}, idField string) TaskTemplateRow {
	r := TaskTemplateRow{}
	if v, ok := row[idField]; ok {
		r.TaskID, _ = asInt32(v)
	}
	if v, ok := row["task_type"]; ok {
		r.TaskType, _ = asInt32(v)
	}
	if v, ok := row["add_type"]; ok {
		r.AddType, _ = asInt32(v)
	}
	if v, ok := row["value"]; ok {
		r.Value = parseInt32List(v)
	}
	if v, ok := row["rewards"]; ok {
		r.Rewards = parseInt32List(v)
	}
	if v, ok := row["reward_point"]; ok {
		r.RewardPoint, _ = asInt32(v)
	}
	if v, ok := row["dec"]; ok {
		r.Dec, _ = asString(v)
	}
	if v, ok := row["jump"]; ok {
		r.Jump, _ = asInt32(v)
	}
	if v, ok := row["sort"]; ok {
		r.Sort, _ = asInt32(v)
	}
	if v, ok := row["group"]; ok {
		r.Group, _ = asInt32(v)
	}
	if v, ok := row["ach_type"]; ok {
		r.AchType, _ = asInt32(v)
	}
	if v, ok := row["ach_level"]; ok {
		r.AchLevel, _ = asInt32(v)
	}
	return r
}

func loadTaskTemplates(tableName, idField string) []TaskTemplateRow {
	t := Get(tableName)
	if t == nil {
		return nil
	}
	out := make([]TaskTemplateRow, 0, len(t.List))
	for _, row := range t.List {
		out = append(out, parseTaskTemplate(row, idField))
	}
	return out
}

func DailyTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_dailytasks", "task_id")
}

func WeeklyTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_weeklytasks", "task_id")
}

func ChallengeTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_challengetasks", "task_id")
}

func SevenDayTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_seven_day_tasks", "task_id")
}

func ThemeTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_theme_task", "task_id")
}

func FestivalTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_festival_task", "task_id")
}

func AchievementTasks() []TaskTemplateRow {
	return loadTaskTemplates("tpl_achievement_task", "id")
}

func SevenDayStages() []map[string]interface{} {
	t := Get("tpl_seven_day_tasks_stage")
	if t == nil {
		return nil
	}
	return t.List
}

func AchievementCountByLevel() map[int32]int {
	tasks := AchievementTasks()
	out := make(map[int32]int)
	for _, task := range tasks {
		out[task.AchLevel]++
	}
	return out
}
