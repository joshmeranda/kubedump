package kubedump

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	defaultEncoderConfig = zapcore.EncoderConfig{
		MessageKey:       "msg",
		LevelKey:         "level",
		TimeKey:          "time",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		StacktraceKey:    "stacktrace",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.LowercaseLevelEncoder,
		EncodeTime:       zapcore.RFC3339TimeEncoder,
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
		EncodeName:       zapcore.FullNameEncoder,
		ConsoleSeparator: " ",
	}
)

type LoggerOption func(*zap.Config)

func WithLevel(level zap.AtomicLevel) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.Level = level
	}
}

func WithDevelopment(isDevelopment bool) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.Development = isDevelopment
	}
}

func WithDisableCaller(disableCaller bool) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.DisableCaller = disableCaller
	}
}

func WithDisableStacktrace(disableStacktrace bool) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.DisableStacktrace = disableStacktrace
	}
}

func WithSampling(sampling *zap.SamplingConfig) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.Sampling = sampling
	}
}

func WithEncoding(encoding string) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.Encoding = encoding
	}
}

func WithEncoderConfig(encoderCfg zapcore.EncoderConfig) LoggerOption {
	return func(cfg *zap.Config) {
		cfg.EncoderConfig = encoderCfg
	}
}

func WithPaths(paths ...string) LoggerOption {
	return func(cfg *zap.Config) {
		for _, path := range paths {
			cfg.OutputPaths = append(cfg.OutputPaths, path)
		}
	}
}

func NewLogger(opts ...LoggerOption) *zap.SugaredLogger {
	loggerCfg := zap.Config{
		Level:             zap.NewAtomicLevel(),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: true,
		Sampling:          nil,
		Encoding:          "console",
		EncoderConfig:     defaultEncoderConfig,
		OutputPaths:       []string{"stdout"},
	}

	for _, opt := range opts {
		opt(&loggerCfg)
	}

	logger, err := loggerCfg.Build()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}

	return logger.Sugar().Named("kubedump")
}
