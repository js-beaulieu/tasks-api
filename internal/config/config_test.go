package config_test

import (
	"log/slog"
	"testing"

	"github.com/js-beaulieu/tasks/internal/config"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want config.Config
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: config.Config{Port: "8080", LogFormat: "json", LogLevel: slog.LevelInfo, LogDetailed: false},
		},
		{
			name: "custom port",
			env:  map[string]string{"PORT": "9090"},
			want: config.Config{Port: "9090", LogFormat: "json", LogLevel: slog.LevelInfo},
		},
		{
			name: "pretty format",
			env:  map[string]string{"LOG_FORMAT": "pretty"},
			want: config.Config{Port: "8080", LogFormat: "pretty", LogLevel: slog.LevelInfo},
		},
		{
			name: "debug level",
			env:  map[string]string{"LOG_LEVEL": "debug"},
			want: config.Config{Port: "8080", LogFormat: "json", LogLevel: slog.LevelDebug},
		},
		{
			name: "warn level",
			env:  map[string]string{"LOG_LEVEL": "warn"},
			want: config.Config{Port: "8080", LogFormat: "json", LogLevel: slog.LevelWarn},
		},
		{
			name: "error level",
			env:  map[string]string{"LOG_LEVEL": "error"},
			want: config.Config{Port: "8080", LogFormat: "json", LogLevel: slog.LevelError},
		},
		{
			name: "detailed logging enabled",
			env:  map[string]string{"LOG_DETAILED": "true"},
			want: config.Config{Port: "8080", LogFormat: "json", LogLevel: slog.LevelInfo, LogDetailed: true},
		},
		{
			name: "invalid log level falls back to info",
			env:  map[string]string{"LOG_LEVEL": "nonsense"},
			want: config.Config{Port: "8080", LogFormat: "json", LogLevel: slog.LevelInfo},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", "")
			t.Setenv("LOG_FORMAT", "")
			t.Setenv("LOG_LEVEL", "")
			t.Setenv("LOG_DETAILED", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got := config.Load()
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
