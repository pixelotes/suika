package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

type Config struct {
	App       AppConfig `yaml:"app"`
	Libraries []Library `yaml:"libraries"`
	Users     []User    `yaml:"users"`
}

type AppConfig struct {
	Port       int    `yaml:"port"`
	UIPassword string `yaml:"ui_password"`
	JWTSecret  string `yaml:"jwt_secret"`
	Debug      bool   `yaml:"debug"`
}

type Library struct {
	FriendlyName    string `yaml:"friendly_name"`
	Path            string `yaml:"path"`
	ReadingDirection string `yaml:"reading_direction"` // "rtl" or "ltr", default "rtl"
}

type User struct {
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Libraries []string `yaml:"libraries"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	expanded := envVarRegex.ReplaceAllStringFunc(string(data), func(match string) string {
		groups := envVarRegex.FindStringSubmatch(match)
		name := groups[1]
		if name == "" {
			name = groups[2]
		}
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return match
	})

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.setDefaults()
	cfg.resolvePaths()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.App.Port == 0 {
		c.App.Port = 8080
	}
	if c.App.JWTSecret == "" {
		c.App.JWTSecret = "change-me-in-production"
	}
}

func (c *Config) resolvePaths() {
	for i, lib := range c.Libraries {
		if abs, err := filepath.Abs(lib.Path); err == nil {
			c.Libraries[i].Path = abs
		}
	}
}

func (c *Config) validate() error {
	if len(c.Users) == 0 && c.App.UIPassword == "" {
		return fmt.Errorf("either app.ui_password or users must be configured")
	}
	if len(c.Libraries) == 0 {
		return fmt.Errorf("at least one library must be configured")
	}
	for i, lib := range c.Libraries {
		if lib.Path == "" {
			return fmt.Errorf("library[%d].path is required", i)
		}
		if lib.FriendlyName == "" {
			return fmt.Errorf("library[%d].friendly_name is required", i)
		}
	}
	return nil
}

func (c *Config) AuthenticateUser(username, password string) *User {
	if len(c.Users) == 0 {
		if password == c.App.UIPassword {
			return &User{Username: "suika", Password: c.App.UIPassword}
		}
		return nil
	}
	for _, u := range c.Users {
		if u.Username == username && u.Password == password {
			return &u
		}
	}
	return nil
}

func (c *Config) LibrariesForUser(username string) []Library {
	user := c.FindUser(username)
	if user == nil || len(user.Libraries) == 0 {
		return c.Libraries
	}
	allowed := make(map[string]bool, len(user.Libraries))
	for _, name := range user.Libraries {
		allowed[name] = true
	}
	var libs []Library
	for _, lib := range c.Libraries {
		if allowed[lib.FriendlyName] {
			libs = append(libs, lib)
		}
	}
	return libs
}

func (c *Config) FindUser(username string) *User {
	if len(c.Users) == 0 {
		return &User{Username: "suika"}
	}
	for _, u := range c.Users {
		if u.Username == username {
			return &u
		}
	}
	return nil
}

func (c *Config) IsPathAllowed(reqPath string, username string) bool {
	libs := c.LibrariesForUser(username)
	abs, err := filepath.Abs(reqPath)
	if err != nil {
		return false
	}
	for _, lib := range libs {
		rel, err := filepath.Rel(lib.Path, abs)
		if err == nil && len(rel) >= 1 && rel[0] != '.' {
			return true
		}
		if abs == lib.Path {
			return true
		}
	}
	return false
}
