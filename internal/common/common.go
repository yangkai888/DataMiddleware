package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// GenerateID 生成唯一ID
func GenerateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机生成失败，使用时间戳
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// IsEmpty 检查值是否为空
func IsEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case *string:
		return v == nil || strings.TrimSpace(*v) == ""
	case []interface{}:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
			return rv.Len() == 0
		case reflect.Ptr:
			if rv.IsNil() {
				return true
			}
			return IsEmpty(rv.Elem().Interface())
		default:
			return false
		}
	}
}

// ToString 将值转换为字符串
func ToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ToInt 将值转换为int
func ToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		if v > math.MaxInt || v < math.MinInt {
			return 0, ErrInvalidInput
		}
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, ErrInvalidInput
	}
}

// ToInt64 将值转换为int64
func ToInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, ErrInvalidInput
	}
}

// ToFloat64 将值转换为float64
func ToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	case bool:
		if v {
			return 1.0, nil
		}
		return 0.0, nil
	default:
		return 0, ErrInvalidInput
	}
}

// ToBool 将值转换为bool
func ToBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0.0, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, ErrInvalidInput
	}
}

// Contains 检查切片是否包含指定值
func Contains(slice interface{}, item interface{}) bool {
	if slice == nil {
		return false
	}

	sv := reflect.ValueOf(slice)
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		return false
	}

	for i := 0; i < sv.Len(); i++ {
		if reflect.DeepEqual(sv.Index(i).Interface(), item) {
			return true
		}
	}

	return false
}

// Unique 去重切片
func Unique(slice interface{}) interface{} {
	if slice == nil {
		return slice
	}

	sv := reflect.ValueOf(slice)
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		return slice
	}

	seen := make(map[interface{}]bool)
	result := reflect.MakeSlice(sv.Type(), 0, sv.Len())

	for i := 0; i < sv.Len(); i++ {
		item := sv.Index(i).Interface()
		if !seen[item] {
			seen[item] = true
			result = reflect.Append(result, sv.Index(i))
		}
	}

	return result.Interface()
}

// Filter 过滤切片
func Filter(slice interface{}, predicate func(interface{}) bool) interface{} {
	if slice == nil {
		return slice
	}

	sv := reflect.ValueOf(slice)
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		return slice
	}

	result := reflect.MakeSlice(sv.Type(), 0, sv.Len())

	for i := 0; i < sv.Len(); i++ {
		item := sv.Index(i).Interface()
		if predicate(item) {
			result = reflect.Append(result, sv.Index(i))
		}
	}

	return result.Interface()
}

// Map 映射切片元素
func Map(slice interface{}, mapper func(interface{}) interface{}) interface{} {
	if slice == nil {
		return slice
	}

	sv := reflect.ValueOf(slice)
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		return slice
	}

	elemType := reflect.TypeOf(mapper(sv.Index(0).Interface()))
	result := reflect.MakeSlice(reflect.SliceOf(elemType), sv.Len(), sv.Len())

	for i := 0; i < sv.Len(); i++ {
		item := sv.Index(i).Interface()
		mapped := mapper(item)
		result.Index(i).Set(reflect.ValueOf(mapped))
	}

	return result.Interface()
}

// Max 返回最大值
func Max(values ...interface{}) interface{} {
	if len(values) == 0 {
		return nil
	}

	max := values[0]
	for _, v := range values[1:] {
		if compare(v, max) > 0 {
			max = v
		}
	}
	return max
}

// Min 返回最小值
func Min(values ...interface{}) interface{} {
	if len(values) == 0 {
		return nil
	}

	min := values[0]
	for _, v := range values[1:] {
		if compare(v, min) < 0 {
			min = v
		}
	}
	return min
}

// compare 比较两个值
func compare(a, b interface{}) int {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	// 类型不同无法比较
	if aVal.Kind() != bVal.Kind() {
		return 0
	}

	switch aVal.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		aInt := aVal.Int()
		bInt := bVal.Int()
		if aInt > bInt {
			return 1
		} else if aInt < bInt {
			return -1
		}
		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		aUint := aVal.Uint()
		bUint := bVal.Uint()
		if aUint > bUint {
			return 1
		} else if aUint < bUint {
			return -1
		}
		return 0
	case reflect.Float32, reflect.Float64:
		aFloat := aVal.Float()
		bFloat := bVal.Float()
		if aFloat > bFloat {
			return 1
		} else if aFloat < bFloat {
			return -1
		}
		return 0
	case reflect.String:
		aStr := aVal.String()
		bStr := bVal.String()
		return strings.Compare(aStr, bStr)
	default:
		return 0
	}
}

// Clamp 将值限制在指定范围内
func Clamp(value, min, max interface{}) interface{} {
	if compare(value, min) < 0 {
		return min
	}
	if compare(value, max) > 0 {
		return max
	}
	return value
}

// Round 四舍五入
func Round(value float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(value*ratio) / ratio
}

// Truncate 截断小数位
func Truncate(value float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Trunc(value*ratio) / ratio
}
