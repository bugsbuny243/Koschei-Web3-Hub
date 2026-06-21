package services

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"
)

func marshalSecurityRadarJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err == nil {
		return encoded, nil
	}
	sanitized := sanitizeSecurityRadarJSON(value, 0)
	encoded, retryErr := json.Marshal(sanitized)
	if retryErr != nil {
		return nil, fmt.Errorf("security radar json encoding failed: %w", err)
	}
	return encoded, nil
}

func sanitizeSecurityRadarJSON(value any, depth int) any {
	if depth > 16 {
		return "depth_limit"
	}
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil
		}
		return typed
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return nil
		}
		return typed
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	case *time.Time:
		if typed == nil {
			return nil
		}
		return typed.UTC().Format(time.RFC3339Nano)
	case error:
		return typed.Error()
	case fmt.Stringer:
		return typed.String()
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = sanitizeSecurityRadarJSON(item, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sanitizeSecurityRadarJSON(item, depth+1)
		}
		return out
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		out := map[string]any{}
		iter := rv.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = sanitizeSecurityRadarJSON(iter.Value().Interface(), depth+1)
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = sanitizeSecurityRadarJSON(rv.Index(i).Interface(), depth+1)
		}
		return out
	case reflect.Bool:
		return rv.Bool()
	case reflect.String:
		return rv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint()
	case reflect.Float32, reflect.Float64:
		value := rv.Float()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil
		}
		return value
	case reflect.Struct:
		return fmt.Sprint(value)
	default:
		return fmt.Sprint(value)
	}
}
