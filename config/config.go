package config

import (
	"fmt"
	"os"
	"io/ioutil"
	"encoding/json"
)

// Auth represents an authentication by a tuple (username, password hash).
type Auth struct {
	/* user name to authenticate. If empty, no authentication */
	Username string `json:"username"`

	/* hash of the password. Use revproxyhashry to hash it */
	PasswordHash string `json:"password_hash"`
}

// Route represents a route of a reverse proxy.
type Route struct {
	/* Route prefix */
	Prefix string `json:"prefix"`

	/*
	path to the target.
	If a directory, everything beneath it will be served beneath the prefix.
	If an URL, redirects to that URL after stripping the prefix.
	*/
	Target  string   `json:"target"`
	AuthIDs []string `json:"auths"`
}

// Config represents a parsed config JSON file.
type Config struct {
	Auths          map[string]*Auth `json:"auths"`
	Domain         string           `json:"domain"`
	Routes         []Route          `json:"routes"`
	SslKeyPath     string           `json:"ssl_key_path"`
	SslCertPath    string           `json:"ssl_cert_path"`
	LetsencryptDir string           `json:"letsencrypt_dir"`
	HttpAddress    string           `json:"http_address"`
	HttpsAddress   string           `json:"https_address"`
}

// Validate validates the parsed config.
func Validate(cfg *Config) error {
	for _, route := range cfg.Routes {
		for _, authID := range route.AuthIDs {
			_, ok := cfg.Auths[authID]

			if !ok {
				return fmt.Errorf(
					"Auth could not be found in the list of auths for the Route with prefix %s: %#v",
					route.Prefix, authID)
			}
		}
	}

	if (cfg.SslCertPath != "" && cfg.SslKeyPath == "") ||
		(cfg.SslCertPath == "" && cfg.SslKeyPath != "") {
		return fmt.Errorf("either both SSL cert and key are empty, or none: %#v and %#v",
			cfg.SslCertPath, cfg.SslKeyPath)
	}

	useSSL := (cfg.SslCertPath != "" && cfg.SslKeyPath == "") || cfg.LetsencryptDir != ""

	if cfg.LetsencryptDir != "" && cfg.SslCertPath != "" {
		return fmt.Errorf("both letsencrypt_dir and ssl_cert_path were specified in cfg: %#v and %#v",
			cfg.LetsencryptDir, cfg.SslCertPath)
	}

	if cfg.LetsencryptDir != "" && cfg.Domain == "" {
		return fmt.Errorf("letsencrypt_dir was specified in cfg, but no domain: %#v",
			cfg.LetsencryptDir)
	}

	if useSSL && cfg.HttpsAddress == "" {
		return fmt.Errorf("cfg needs to use SSL, but https_address was not specified")
	}

	if cfg.HttpAddress == "" {
		return fmt.Errorf("http_address was not specified in cfg")
	}

	return nil
}

// Load loads and parses the config file from the given path.
func Load(path string) (cfg *Config, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	text, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	cfg = &Config{}
	err = json.Unmarshal(text, cfg)
	if err != nil {
		return nil, err
	}

	err = Validate(cfg)
	if err != nil {
		return
	}

	return
}
