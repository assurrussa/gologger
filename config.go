package gologger

// Config is the portable logging configuration. Tags describe configuration and
// environment mappings; New does not read the environment itself.
type Config struct {
	Env        string  `toml:"app_env" long:"app-env" env:"APP_ENV" value-default:"production" validate:"required,oneof=local development staging production"` //nolint:lll // it's config
	Rate       float64 `toml:"rate" long:"log-rate" env:"LOG_RATE" value-default:"0" validate:"min=0,max=1"`
	Level      string  `toml:"level" long:"log-level" env:"LOG_LEVEL" value-default:"info" validate:"required,oneof=debug info warn error"` //nolint:lll // it's config
	AddSource  bool    `toml:"add_source" long:"add-source" env:"LOG_ADD_SOURCE" validate:"boolean"`
	AddVerbose bool    `toml:"add_verbose" long:"add-verbose" env:"LOG_ADD_VERBOSE" validate:"boolean"`
	JSON       bool    `toml:"json" long:"log-json" env:"LOG_JSON" validate:"boolean"`
	Output     string  `toml:"output" long:"log-output" env:"LOG_OUTPUT"`
}

func (c Config) IsProduction() bool {
	return c.Env == "production"
}

func (c Config) IsStaging() bool {
	return c.Env == "stage"
}

func (c Config) IsDev() bool {
	return c.Env == "development"
}

func (c Config) IsLocal() bool {
	return c.Env == "local" || c.Env == "development"
}
