package config

// AppEnv, LogLevel and SSLMode are distinct types rather than strings so FX
// cannot hand one bare string where another is expected.
type AppEnv string

const (
	EnvDevelopment AppEnv = "development"
	EnvStaging     AppEnv = "staging"
	EnvProduction  AppEnv = "production"
)

func (e AppEnv) IsDevelopment() bool { return e == EnvDevelopment }
func (e AppEnv) IsProduction() bool  { return e == EnvProduction }

func AllowedEnvironments() []AppEnv {
	return []AppEnv{EnvDevelopment, EnvStaging, EnvProduction}
}

type LogLevel string

const (
	LogDebug  LogLevel = "debug"
	LogInfo   LogLevel = "info"
	LogWarn   LogLevel = "warn"
	LogError  LogLevel = "error"
	LogDPanic LogLevel = "dpanic"
	LogPanic  LogLevel = "panic"
	LogFatal  LogLevel = "fatal"
)

func AllowedLogLevels() []LogLevel {
	return []LogLevel{LogDebug, LogInfo, LogWarn, LogError, LogDPanic, LogPanic, LogFatal}
}

type SSLMode string

const (
	SSLModeDisable    SSLMode = "disable"
	SSLModeAllow      SSLMode = "allow"
	SSLModePrefer     SSLMode = "prefer"
	SSLModeRequire    SSLMode = "require"
	SSLModeVerifyCA   SSLMode = "verify-ca"
	SSLModeVerifyFull SSLMode = "verify-full"
)

func AllowedSSLModes() []SSLMode {
	return []SSLMode{
		SSLModeDisable,
		SSLModeAllow,
		SSLModePrefer,
		SSLModeRequire,
		SSLModeVerifyCA,
		SSLModeVerifyFull,
	}
}
