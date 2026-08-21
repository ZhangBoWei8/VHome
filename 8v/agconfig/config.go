package agconfig

import (
	"os"
	"strings"
)

const maxProviders = 5

type AGConfig struct {
	DefaultProvider string
	Providers       map[string]ProviderConfig
}

type ProviderConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func Load() AGConfig {
	cfg := defaults()
	applyEnv(&cfg)
	return cfg
}

func defaults() AGConfig {
	return AGConfig{
		DefaultProvider: "DeepSeek",
		Providers: map[string]ProviderConfig{
			"DeepSeek": {
				APIKey:  "",
				BaseURL: "",
				Model:   "deepseek-v4",
			},
			// 简化写法，声明时知道了value中都是ProviderrConfig类型，所以不需要关心后续的类型名
			"Glm": {
				APIKey:  "",
				BaseURL: "",
				Model:   "",
			},
		},
	}
}

func applyEnv(cfg *AGConfig) {
	for name, provider := range cfg.Providers {
		envKey := strings.ToUpper(name) + "_APIKEY"
		envModel := strings.ToUpper(name) + "_MODEL"
		envBsaeURL := strings.ToUpper(name) + "_BASEURL"
		if k, m, b := os.Getenv(envKey), os.Getenv(envModel), os.Getenv(envBsaeURL); k != "" && m != "" && b != "" {
			provider.APIKey = k
			provider.BaseURL = b
			provider.Model = m
			cfg.Providers[name] = provider
		}
	}
}

func (c AGConfig) Provider(name string) ProviderConfig {
	if p, ok := c.Providers[strings.ToLower(name)]; ok {
		return p
	}
	return ProviderConfig{}
}
