package common

import (
	"testing"
)

func TestConvertImageURLsInContent(t *testing.T) {
	baseURL := "http://localhost:8000"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Markdown格式图片",
			input:    "Some text\n![image-0](http://127.0.0.1:8002/images/abc123def.jpeg)\nMore text",
			expected: "Some text\n![image-0](http://localhost:8000/api/v1/images/abc123def.jpeg)\nMore text",
		},
		{
			name:     "HTTP URL格式图片",
			input:    "Some text\nhttp://127.0.0.1:8002/images/c9672c2038744a51894de48ae915f900.jpeg\nMore text",
			expected: "Some text\n![image](http://localhost:8000/api/v1/images/c9672c2038744a51894de48ae915f900.jpeg)\nMore text",
		},
		{
			name:     "混合格式 - Markdown和HTTP URL",
			input:    "Text\n![image-0](http://127.0.0.1:8002/images/abc123def.jpeg)\nhttp://127.0.0.1:8002/images/111222333.jpeg\nMore",
			expected: "Text\n![image-0](http://localhost:8000/api/v1/images/abc123def.jpeg)\n![image](http://localhost:8000/api/v1/images/111222333.jpeg)\nMore",
		},
		{
			name:     "多个HTTP URL格式图片",
			input:    "http://127.0.0.1:8002/images/c9672c2038744a51894de48ae915f900.jpeg\n\nhttp://localhost:8002/images/abc123def456.jpeg\n\nhttp://127.0.0.1:8002/images/0a1b2c3d4e5f.jpeg",
			expected: "![image](http://localhost:8000/api/v1/images/c9672c2038744a51894de48ae915f900.jpeg)\n\n![image](http://localhost:8000/api/v1/images/abc123def456.jpeg)\n\n![image](http://localhost:8000/api/v1/images/0a1b2c3d4e5f.jpeg)",
		},
		{
			name:     "真实UUID格式的HTTP URL",
			input:    "http://127.0.0.1:8002/images/c163171299174a18b1b820530aa355e4.jpeg\n\nhttp://127.0.0.1:8002/images/299adad9912a4097991fe2c01c9740e2.jpeg",
			expected: "![image](http://localhost:8000/api/v1/images/c163171299174a18b1b820530aa355e4.jpeg)\n\n![image](http://localhost:8000/api/v1/images/299adad9912a4097991fe2c01c9740e2.jpeg)",
		},
		{
			name:     "不匹配的格式不应改变",
			input:    "This is not an image path: images/abc.jpeg",
			expected: "This is not an image path: images/abc.jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertImageURLsInContent(tt.input, baseURL)
			if result != tt.expected {
				t.Errorf("ConvertImageURLsInContent() failed\nInput:    %q\nExpected: %q\nGot:      %q", tt.input, tt.expected, result)
			}
		})
	}
}
