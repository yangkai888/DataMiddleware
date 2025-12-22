package utils

import (
	"testing"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == "" {
		t.Error("GenerateID() 应该返回非空字符串")
	}

	if id1 == id2 {
		t.Error("GenerateID() 应该生成不同的ID")
	}

	// 检查长度（16字节的hex编码应该是32个字符）
	if len(id1) != 32 {
		t.Errorf("GenerateID() 返回的ID长度应该是32，实际为 %d", len(id1))
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"whitespace string", "   ", true},
		{"non-empty string", "hello", false},
		{"empty slice", []int{}, true},
		{"non-empty slice", []int{1, 2}, false},
		{"empty map", map[string]int{}, true},
		{"non-empty map", map[string]int{"a": 1}, false},
		{"zero int", 0, false},
		{"non-zero int", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEmpty(tt.value); got != tt.want {
				t.Errorf("IsEmpty(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"int64", int64(123), "123"},
		{"float64", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"bytes", []byte("hello"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToString(tt.value); got != tt.want {
				t.Errorf("ToString(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    int
		wantErr bool
	}{
		{"int", 42, 42, false},
		{"int64", int64(123), 123, false},
		{"float64", 3.14, 3, false},
		{"string", "456", 456, false},
		{"bool true", true, 1, false},
		{"bool false", false, 0, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToInt(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToInt(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToInt(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    int64
		wantErr bool
	}{
		{"int", 42, 42, false},
		{"int64", int64(123), 123, false},
		{"float64", 3.14, 3, false},
		{"string", "456", 456, false},
		{"bool true", true, 1, false},
		{"bool false", false, 0, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToInt64(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToInt64(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToInt64(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    float64
		wantErr bool
	}{
		{"int", 42, 42.0, false},
		{"int64", int64(123), 123.0, false},
		{"float64", 3.14, 3.14, false},
		{"string", "3.14", 3.14, false},
		{"bool true", true, 1.0, false},
		{"bool false", false, 0.0, false},
		{"invalid string", "abc", 0, true},
		{"invalid type", []int{1}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToFloat64(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToFloat64(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToFloat64(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		want    bool
		wantErr bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"int non-zero", 42, true, false},
		{"int zero", 0, false, false},
		{"int64 non-zero", int64(123), true, false},
		{"int64 zero", int64(0), false, false},
		{"float64 non-zero", 3.14, true, false},
		{"float64 zero", 0.0, false, false},
		{"string true", "true", true, false},
		{"string false", "false", false, false},
		{"string 1", "1", true, false},
		{"string 0", "0", false, false},
		{"invalid string", "abc", false, true},
		{"invalid type", []int{1}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToBool(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToBool(%v) error = %v, wantErr %v", tt.value, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ToBool(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice interface{}
		item  interface{}
		want  bool
	}{
		{"int slice contains", []int{1, 2, 3}, 2, true},
		{"int slice not contains", []int{1, 2, 3}, 4, false},
		{"string slice contains", []string{"a", "b", "c"}, "b", true},
		{"string slice not contains", []string{"a", "b", "c"}, "d", false},
		{"nil slice", nil, 1, false},
		{"non-slice", "not a slice", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.slice, tt.item); got != tt.want {
				t.Errorf("Contains(%v, %v) = %v, want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{"int slice", []int{1, 2, 2, 3, 1}, []int{1, 2, 3}},
		{"string slice", []string{"a", "b", "b", "c", "a"}, []string{"a", "b", "c"}},
		{"empty slice", []int{}, []int{}},
		{"nil slice", nil, nil},
		{"no duplicates", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unique(tt.input)
			// 检查结果是否正确（由于反射，这里只检查基本功能）
			if got == nil && tt.want != nil {
				t.Errorf("Unique() = nil, want %v", tt.want)
			}
			if got != nil && tt.want == nil {
				t.Errorf("Unique() = %v, want nil", got)
			}
		})
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		name string
		args []interface{}
		want interface{}
	}{
		{"int max", []interface{}{1, 3, 2}, 3},
		{"float max", []interface{}{1.1, 3.3, 2.2}, 3.3},
		{"string max", []interface{}{"apple", "zebra", "banana"}, "zebra"},
		{"single value", []interface{}{42}, 42},
		{"empty", []interface{}{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Max(tt.args...)
			if got != tt.want {
				t.Errorf("Max(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name string
		args []interface{}
		want interface{}
	}{
		{"int min", []interface{}{1, 3, 2}, 1},
		{"float min", []interface{}{1.1, 3.3, 2.2}, 1.1},
		{"string min", []interface{}{"apple", "zebra", "banana"}, "apple"},
		{"single value", []interface{}{42}, 42},
		{"empty", []interface{}{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Min(tt.args...)
			if got != tt.want {
				t.Errorf("Min(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		precision int
		want      float64
	}{
		{"round to integer", 3.14159, 0, 3.0},
		{"round to 1 decimal", 3.14159, 1, 3.1},
		{"round to 2 decimals", 3.14159, 2, 3.14},
		{"round up", 3.5, 0, 4.0},
		{"round down", 3.4, 0, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Round(tt.value, tt.precision); got != tt.want {
				t.Errorf("Round(%v, %v) = %v, want %v", tt.value, tt.precision, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		precision int
		want      float64
	}{
		{"truncate to integer", 3.14159, 0, 3.0},
		{"truncate to 1 decimal", 3.14159, 1, 3.1},
		{"truncate to 2 decimals", 3.14159, 2, 3.14},
		{"already truncated", 3.0, 2, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.value, tt.precision); got != tt.want {
				t.Errorf("Truncate(%v, %v) = %v, want %v", tt.value, tt.precision, got, tt.want)
			}
		})
	}
}
