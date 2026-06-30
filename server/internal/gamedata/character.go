package gamedata

// Character/skin/item accessors over tpl_character, tpl_character_skin, tpl_props.

// CharacterRow is a typed view of a tpl_character entry.
type CharacterRow struct {
	ID     int32
	Name   string
	Avatar int32
	Sex    int32
	Star   int32
}

// AllCharacters returns every playable character row.
func AllCharacters() []CharacterRow {
	t := Get("tpl_character")
	if t == nil {
		return nil
	}
	out := make([]CharacterRow, 0, len(t.List))
	for _, row := range t.List {
		r := CharacterRow{}
		if v, ok := row["id"]; ok {
			r.ID, _ = asInt32(v)
		}
		if v, ok := row["name"]; ok {
			r.Name, _ = asString(v)
		}
		if v, ok := row["avatar"]; ok {
			r.Avatar, _ = asInt32(v)
		}
		if v, ok := row["sex"]; ok {
			r.Sex, _ = asInt32(v)
		}
		if v, ok := row["star"]; ok {
			r.Star, _ = asInt32(v)
		}
		out = append(out, r)
	}
	return out
}

// AllRoleIDs returns every character id from tpl_character.
// Replaces the hardcoded allRoles roster in ws/gamedata.go.
func AllRoleIDs() []int32 {
	t := Get("tpl_character")
	if t == nil {
		return nil
	}
	out := make([]int32, 0, len(t.List))
	for _, row := range t.List {
		if id, ok := asInt32(row["id"]); ok {
			out = append(out, id)
		}
	}
	return out
}

// CharacterWithSkins pairs a character id with the skin ids it owns.
type CharacterWithSkins struct {
	RoleID int32
	Skins  []int32 // first skin is the default (kind=0) skin
}

// AllRolesWithSkins returns every character paired with its owned skin ids.
// Replaces the hardcoded allRoles roster in ws/gamedata.go.
// Characters with no skins are skipped (data integrity issue).
func AllRolesWithSkins() []CharacterWithSkins {
	ids := AllRoleIDs()
	out := make([]CharacterWithSkins, 0, len(ids))
	for _, id := range ids {
		skins := SkinsForRole(id)
		if len(skins) == 0 {
			continue
		}
		out = append(out, CharacterWithSkins{RoleID: id, Skins: skins})
	}
	return out
}

// CharacterSkinRow is a typed view of a tpl_character_skin entry.
type CharacterSkinRow struct {
	ID     int32
	RoleID int32 // character this skin belongs to
	SkinID int32 // alias of ID for clarity
}

// AllCharacterSkins returns every skin row from tpl_character_skin.
func AllCharacterSkins() []CharacterSkinRow {
	t := Get("tpl_character_skin")
	if t == nil {
		return nil
	}
	out := make([]CharacterSkinRow, 0, len(t.List))
	for _, row := range t.List {
		r := CharacterSkinRow{}
		if v, ok := row["id"]; ok {
			r.ID, _ = asInt32(v)
			r.SkinID = r.ID
		}
		if v, ok := row["role"]; ok {
			r.RoleID, _ = asInt32(v)
		}
		out = append(out, r)
	}
	return out
}

// RoleBySkinID returns the role_id (character id) that owns the given skin id.
// Returns 0 if the skin id is not found in tpl_character_skin.
func RoleBySkinID(skinID int32) int32 {
	t := Get("tpl_character_skin")
	if t == nil {
		return 0
	}
	for _, row := range t.List {
		id, _ := asInt32(row["id"])
		if id == skinID {
			role, _ := asInt32(row["role"])
			return role
		}
	}
	return 0
}

// AllSkinIDs returns every skin id across all characters.
// Replaces the per-role Skins slice extraction in ws/gamedata.go.
func AllSkinIDs() []int32 {
	skins := AllCharacterSkins()
	out := make([]int32, 0, len(skins))
	for _, s := range skins {
		if s.SkinID != 0 {
			out = append(out, s.SkinID)
		}
	}
	return out
}

// SkinsForRole returns every skin id belonging to the given character id,
// ordered so the kind=0 default skin comes first (matching the prior
// hardcoded allRoles.Skins[0] convention used by HandleRoleListREQ).
func SkinsForRole(roleID int32) []int32 {
	t := Get("tpl_character_skin")
	if t == nil {
		return nil
	}
	var defaultSkin int32
	var others []int32
	for _, row := range t.List {
		role, _ := asInt32(row["role"])
		if role != roleID {
			continue
		}
		skinID, _ := asInt32(row["id"])
		if skinID == 0 {
			continue
		}
		kind, _ := asInt32(row["kind"])
		if kind == 0 && defaultSkin == 0 {
			defaultSkin = skinID
		} else {
			others = append(others, skinID)
		}
	}
	if defaultSkin == 0 && len(others) == 0 {
		return nil
	}
	out := make([]int32, 0, 1+len(others))
	if defaultSkin != 0 {
		out = append(out, defaultSkin)
	}
	out = append(out, others...)
	return out
}

// AllItemIDs returns every item id from tpl_props.
// Replaces the hardcoded allItemIDs list in ws/gamedata.go.
func AllItemIDs() []int32 {
	t := Get("tpl_props")
	if t == nil {
		return nil
	}
	out := make([]int32, 0, len(t.List))
	for _, row := range t.List {
		if id, ok := asInt32(row["id"]); ok {
			out = append(out, id)
		}
	}
	return out
}

// ItemExists reports whether the given item id is present in tpl_props.
func ItemExists(itemID int32) bool {
	t := Get("tpl_props")
	if t == nil {
		return false
	}
	_, ok := t.Map[idToString(itemID)]
	return ok
}

// ConstData returns the raw dict from tpl_constdata (e.g. DefaultCharacter, MaxLevel).
// Callers type-assert values as needed.
func ConstData() map[string]interface{} {
	t := Get("tpl_constdata")
	if t == nil {
		return nil
	}
	return t.Dict
}

// ConstInt returns an integer constant from tpl_constdata, or fallback if missing/non-numeric.
func ConstInt(key string, fallback int32) int32 {
	d := ConstData()
	if d == nil {
		return fallback
	}
	if v, ok := d[key]; ok {
		if n, ok := asInt32(v); ok {
			return n
		}
	}
	return fallback
}

// ConstString returns a string constant from tpl_constdata, or fallback if missing.
func ConstString(key, fallback string) string {
	d := ConstData()
	if d == nil {
		return fallback
	}
	if v, ok := d[key]; ok {
		if s, ok := asString(v); ok {
			return s
		}
	}
	return fallback
}
