package paw

func newDefaultConfig() *Config {
	return &Config{
		Password: PasswordConfig{
			Passphrase: PassphrasePasswordConfig{
				DefaultLength: PassphrasePasswordDefaultLength,
				MaxLength:     PassphrasePasswordMaxLength,
				MinLength:     PassphrasePasswordMinLength,
			},
			Pin: PinPasswordConfig{
				DefaultLength: PinPasswordDefaultLength,
				MaxLength:     PinPasswordMaxLength,
				MinLength:     PinPasswordMinLength,
			},
			Random: RandomPasswordConfig{
				DefaultLength: RandomPasswordDefaultLength,
				DefaultFormat: RandomPasswordDefaultFormat,
				MaxLength:     RandomPasswordMaxLength,
				MinLength:     RandomPasswordMinLength,
			},
		},
		TOTP: TOTPConfig{
			Digits:   TOTPDigitsDefault,
			Hash:     TOTPHashDefault,
			Interval: TOTPIntervalDefault,
		},
	}
}

type Config struct {
	TOTP     TOTPConfig     `json:"totp,omitempty"`
	Password PasswordConfig `json:"password,omitempty"`
}

type PasswordConfig struct {
	Passphrase PassphrasePasswordConfig `json:"passphrase,omitempty"`
	Pin        PinPasswordConfig        `json:"pin,omitempty"`
	Random     RandomPasswordConfig     `json:"random,omitempty"`
}

type PassphrasePasswordConfig struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}

type PinPasswordConfig struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}
type RandomPasswordConfig struct {
	DefaultLength int    `json:"default_length,omitempty"`
	DefaultFormat Format `json:"default_format,omitempty"`
	MaxLength     int    `json:"max_length,omitempty"`
	MinLength     int    `json:"min_length,omitempty"`
}

type TOTPConfig struct {
	Digits   int      `json:"digits,omitempty"`
	Hash     TOTPHash `json:"hash,omitempty"`
	Interval int      `json:"interval,omitempty"`
}
