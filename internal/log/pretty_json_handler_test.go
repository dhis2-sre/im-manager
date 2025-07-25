package log

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestPrettyJSONHandler(t *testing.T) {
	tests := []struct {
		name        string
		prettyPrint bool
	}{
		{
			name:        "pretty print enabled",
			prettyPrint: true,
		},
		{
			name:        "pretty print disabled",
			prettyPrint: false,
		},
	}

	fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedData := map[string]interface{}{
		"level": "INFO",
		"msg":   "test message",
		"time":  "2024-01-01T00:00:00Z",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			opts := &PrettyJSONHandlerOptions{
				HandlerOptions: slog.HandlerOptions{
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == "time" {
							return slog.Time(a.Key, fixedTime)
						}
						return a
					},
				},
				PrettyPrint: tt.prettyPrint,
			}

			logger := slog.New(NewPrettyJSONHandler(buf, opts))
			logger.Info("test message")

			got := buf.String()

			// Verify it ends with a newline
			if !strings.HasSuffix(got, "\n") {
				t.Error("Output should end with newline")
			}

			// Parse the JSON and compare the data
			var gotData map[string]interface{}
			if err := json.Unmarshal([]byte(got), &gotData); err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			// Compare fields
			for k, want := range expectedData {
				if got := gotData[k]; got != want {
					t.Errorf("Field %q = %v, want %v", k, got, want)
				}
			}

			// Verify pretty printing format
			if tt.prettyPrint {
				if !strings.Contains(got, "\n  ") {
					t.Error("Pretty print enabled but output is not indented")
				}
			} else {
				if strings.Contains(got, "\n  ") {
					t.Error("Pretty print disabled but output is indented")
				}
			}
		})
	}
}

func TestNilOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(NewPrettyJSONHandler(buf, nil))
	logger.Info("test message")

	// Simply verify it doesn't panic and produces some output
	if buf.Len() == 0 {
		t.Error("Expected output, got nothing")
	}
}

func TestInvalidJSON(t *testing.T) {
	opts := &PrettyJSONHandlerOptions{
		HandlerOptions: slog.HandlerOptions{
			// Create invalid JSON by replacing quotes with invalid characters
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "msg" {
					return slog.String(a.Key, string([]byte{0xFF, 0xFE}))
				}
				return a
			},
		},
		PrettyPrint: true,
	}

	buf := &bytes.Buffer{}
	logger := slog.New(NewPrettyJSONHandler(buf, opts))
	logger.Info("test message")

	// Should still produce some output even with invalid JSON
	if buf.Len() == 0 {
		t.Error("Expected output, got nothing")
	}
}

func TestAttributes(t *testing.T) {
	buf := &bytes.Buffer{}
	opts := &PrettyJSONHandlerOptions{PrettyPrint: true}
	logger := slog.New(NewPrettyJSONHandler(buf, opts))

	logger.Info("test message",
		"string", "value",
		"number", 42,
		"bool", true,
	)

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	expected := map[string]interface{}{
		"string": "value",
		"number": float64(42), // JSON numbers are always float64
		"bool":   true,
	}

	for k, v := range expected {
		if got := result[k]; got != v {
			t.Errorf("Attribute %q = %v, want %v", k, got, v)
		}
	}
}
