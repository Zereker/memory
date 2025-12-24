package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
)

// Config 日志配置
type Config struct {
	Path           string `toml:"path"`
	RotationTime   string `toml:"rotation_time"`
	MaxAge         string `toml:"max_age"`
	DefaultPattern string `toml:"default_pattern"`
	Level          string `toml:"level"`
	Format         string `toml:"format"` // text 或 json
}

// Validate 验证配置
func (cfg *Config) Validate() error {
	if strings.TrimSpace(cfg.Path) == "" {
		return errors.New("path is required")
	}

	if _, err := time.ParseDuration(cfg.RotationTime); err != nil {
		return errors.New("rotation_time is invalid: " + err.Error())
	}

	if _, err := time.ParseDuration(cfg.MaxAge); err != nil {
		return errors.New("max_age is invalid: " + err.Error())
	}

	if strings.TrimSpace(cfg.DefaultPattern) == "" {
		return errors.New("default_pattern is required")
	}

	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, strings.ToLower(cfg.Level)) {
		return errors.New("invalid level: " + cfg.Level)
	}

	validFormats := []string{"text", "json"}
	if !contains(validFormats, strings.ToLower(cfg.Format)) {
		return errors.New("invalid format: " + cfg.Format)
	}

	return nil
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Init 初始化日志系统
func Init(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	fileWriter, err := configureFileLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to configure file logger: %w", err)
	}

	opts := &slog.HandlerOptions{
		Level: mapLevel(cfg.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(a.Key, t.Format("2006-01-02 15:04:05.000000"))
				}
			}
			return a
		},
	}

	out := io.MultiWriter(os.Stdout, fileWriter)

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(out, opts)
	} else {
		handler = slog.NewTextHandler(out, opts)
	}

	slog.SetDefault(slog.New(handler))
	return nil
}

func configureFileLogger(cfg Config) (io.Writer, error) {
	rotationTime, err := time.ParseDuration(cfg.RotationTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rotation_time: %v", err)
	}

	maxAge, err := time.ParseDuration(cfg.MaxAge)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max_age: %v", err)
	}

	if cfg.DefaultPattern == "" {
		cfg.DefaultPattern = "memory-%Y-%m-%d.log"
	}

	pattern := fmt.Sprintf("%s/%s", cfg.Path, cfg.DefaultPattern)

	return rotatelogs.New(
		pattern,
		rotatelogs.WithRotationTime(rotationTime),
		rotatelogs.WithMaxAge(maxAge),
	)
}

func mapLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Logger 返回带 module 字段的 logger
func Logger(module string) *slog.Logger {
	return slog.Default().With("module", module)
}
