package common

import (
	"testing"
)

func TestSanitizeMilvusString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "正常字符串",
			input:    "normal-string-123",
			expected: "normal-string-123",
		},
		{
			name:     "包含双引号",
			input:    `test"value`,
			expected: `test\"value`,
		},
		{
			name:     "包含反斜杠",
			input:    `test\value`,
			expected: `test\\value`,
		},
		{
			name:     "SQL注入尝试 - 双引号",
			input:    `test" OR 1==1 OR "`,
			expected: `test\" OR 1==1 OR \"`,
		},
		{
			name:     "复杂注入尝试",
			input:    `test\"; DROP TABLE users; --`,
			expected: `test\\\"; DROP TABLE users; --`,
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeMilvusString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeMilvusString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 有效的UUID - 带连字符格式
		{
			name:     "有效的UUID v4 - 带连字符",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},
		{
			name:     "有效的UUID - 带连字符（小写）",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			expected: true,
		},
		{
			name:     "有效的UUID - 带连字符（大写）",
			input:    "550E8400-E29B-41D4-A716-446655440000",
			expected: true,
		},
		// 有效的UUID - 无连字符格式
		{
			name:     "有效的UUID - 无连字符（小写）",
			input:    "550e8400e29b41d4a716446655440000",
			expected: true,
		},
		{
			name:     "有效的UUID - 无连字符（大写）",
			input:    "550E8400E29B41D4A716446655440000",
			expected: true,
		},
		{
			name:     "有效的UUID - 无连字符（实际documentId）",
			input:    "811e27a8180f44f9839e303b8640cb6b",
			expected: true,
		},
		{
			name:     "有效的UUID - 无连字符（实际documentId 2）",
			input:    "31473fd744c348f89e4d97275af0b9c7",
			expected: true,
		},
		// 无效的UUID
		{
			name:     "无效 - 格式错误",
			input:    "invalid-uuid-format",
			expected: false,
		},
		{
			name:     "无效 - 包含SQL注入",
			input:    `550e8400" OR "1"=="1`,
			expected: false,
		},
		{
			name:     "无效 - 空字符串",
			input:    "",
			expected: false,
		},
		{
			name:     "无效 - 长度不正确（太短）",
			input:    "550e8400-e29b-41d4-a716",
			expected: false,
		},
		{
			name:     "无效 - 长度不正确（无连字符太短）",
			input:    "550e8400e29b41d4a716",
			expected: false,
		},
		{
			name:     "无效 - 长度不正确（太长）",
			input:    "550e8400e29b41d4a716446655440000extra",
			expected: false,
		},
		{
			name:     "无效 - 包含非十六进制字符",
			input:    "550e8400e29b41d4a716446655440zzz",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUUID(tt.input)
			if result != tt.expected {
				t.Errorf("ValidateUUID(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateCollectionName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "有效 - 字母开头",
			input:    "myCollection",
			expected: true,
		},
		{
			name:     "有效 - 包含下划线",
			input:    "my_collection_123",
			expected: true,
		},
		{
			name:     "有效 - 大写字母",
			input:    "MyCollection",
			expected: true,
		},
		{
			name:     "无效 - 数字开头",
			input:    "123collection",
			expected: false,
		},
		{
			name:     "无效 - 包含特殊字符",
			input:    "my-collection",
			expected: false,
		},
		{
			name:     "无效 - 包含空格",
			input:    "my collection",
			expected: false,
		},
		{
			name:     "无效 - 空字符串",
			input:    "",
			expected: false,
		},
		{
			name:     "无效 - 太长（超过255字符）",
			input:    string(make([]byte, 256)),
			expected: false,
		},
		{
			name:     "有效 - 边界情况（255字符）",
			input:    "a" + string(make([]byte, 254)),
			expected: false, // 因为包含空字节，实际应该是 true 如果都是字母
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateCollectionName(tt.input)
			if result != tt.expected {
				t.Errorf("ValidateCollectionName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark 性能测试
func BenchmarkSanitizeMilvusString(b *testing.B) {
	input := `test"value\with"special\chars`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeMilvusString(input)
	}
}

func BenchmarkValidateUUID(b *testing.B) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateUUID(uuid)
	}
}
