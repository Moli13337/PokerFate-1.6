package httpapi

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// JSON helpers shared across the module handlers.

// jsonMarshal serialises v to a JSON byte slice.
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// jsonUnmarshal deserialises raw into v.
func jsonUnmarshal(raw []byte, v interface{}) error {
	return json.Unmarshal(raw, v)
}

// jsonRawList parses a JSON array/text blob coming from a JSONB column into a
// generic slice. Empty/nil input yields an empty []interface{} so the response
// always carries a valid JSON array (never null) for the client's pairs().
func jsonRawList(raw []byte) interface{} {
	if len(raw) == 0 {
		return []interface{}{}
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return []interface{}{}
	}
	if v == nil {
		return []interface{}{}
	}
	return v
}

// isEmptyList reports whether v is an empty []interface{} (the shape returned
// by jsonRawList for empty/NULL JSONB arrays).
func isEmptyList(v interface{}) bool {
	s, ok := v.([]interface{})
	return ok && len(s) == 0
}

// jsonRawMap parses a JSON object blob into a map. Empty/nil input yields an
// empty map so client field access never hits nil.
func jsonRawMap(raw []byte) gin.H {
	if len(raw) == 0 {
		return gin.H{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return gin.H{}
	}
	if m == nil {
		return gin.H{}
	}
	return m
}
