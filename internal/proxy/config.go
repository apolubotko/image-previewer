package proxy

import "github.com/spf13/viper"

type Config struct {
	Port      string `mapstructure:"PORT"`
	Name      string `mapstructure:"NAME"`
	CacheSize int    `mapstructure:"CACHE_SIZE"`
	LogLevel  string `mapstructure:"LOG_LEVEL"`
}

func NewConfig() (config *Config, err error) {
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.app")
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
