// Package config loads and validates deployment configuration.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"yourddo-data-tools/v2/internal/domain/registry"
)

const (
	CompendiumBaseURLEnv = "COMPENDIUM_BASE_URL"
	CompendiumAPIPathEnv = "COMPENDIUM_API_PATH"
	OutputDirEnv         = "OUTPUT_DIR"
	AWSRegionEnv         = "AWS_REGION"
	DataBucketEnv        = "DATA_BUCKET"
	CDNBaseURLEnv        = "CDN_BASE_URL"
	GameVersionEnv       = "GAME_VERSION"
	PublishEnabledEnv    = "PUBLISH_ENABLED"
	WarningsAsErrorsEnv  = "VALIDATION_WARNINGS_AS_ERRORS"
)

const (
	defaultCompendiumBaseURL = "https://ddocompendium.com"
	defaultCompendiumAPIPath = "/api.php"
	defaultOutputDir         = "build/output"
)

var (
	gameVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	awsRegionPattern   = regexp.MustCompile(`^[a-z]{2}(?:-[a-z0-9]+)+-[0-9]+$`)
	bucketPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
)

type Config struct {
	CompendiumBaseURL string
	CompendiumAPIPath string
	OutputDir         string
	AWSRegion         string
	DataBucket        string
	CDNBaseURL        string
	GameVersion       string
	PublishEnabled    bool
	WarningsAsErrors  bool

	// These are local pipeline selections and inputs, not deployment settings.
	Categories []string
	Domains    []string
}

// Defaults contains only safe local defaults. In particular, publication is
// disabled and no bucket, region, CDN, or game version is guessed.
func Defaults() Config {
	return Config{
		CompendiumBaseURL: defaultCompendiumBaseURL,
		CompendiumAPIPath: defaultCompendiumAPIPath,
		OutputDir:         defaultOutputDir,
		Categories:        []string{"All"},
		Domains:           registry.Names(),
	}
}

// Load reads the process environment and validates the complete configuration.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	cfg, err := readEnvironment(lookup)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ReadEnvironment applies environment values without performing whole-pipeline
// validation. It is used by independently runnable stages, which validate only
// the settings they consume after parsing local command-line overrides.
func ReadEnvironment() (Config, error) {
	return readEnvironment(os.LookupEnv)
}

func readEnvironment(lookup func(string) (string, bool)) (Config, error) {
	cfg := Defaults()
	setString(lookup, CompendiumBaseURLEnv, &cfg.CompendiumBaseURL)
	setString(lookup, CompendiumAPIPathEnv, &cfg.CompendiumAPIPath)
	setString(lookup, OutputDirEnv, &cfg.OutputDir)
	setString(lookup, AWSRegionEnv, &cfg.AWSRegion)
	setString(lookup, DataBucketEnv, &cfg.DataBucket)
	setString(lookup, CDNBaseURLEnv, &cfg.CDNBaseURL)
	setString(lookup, GameVersionEnv, &cfg.GameVersion)
	for _, setting := range []struct {
		name        string
		destination *bool
	}{{PublishEnabledEnv, &cfg.PublishEnabled}, {WarningsAsErrorsEnv, &cfg.WarningsAsErrors}} {
		if raw, exists := lookup(setting.name); exists {
			value, err := strconv.ParseBool(strings.TrimSpace(raw))
			if err != nil {
				return Config{}, fmt.Errorf("%s must be true or false", setting.name)
			}
			*setting.destination = value
		}
	}
	return cfg, nil
}

func setString(lookup func(string) (string, bool), name string, destination *string) {
	if value, exists := lookup(name); exists {
		*destination = strings.TrimSpace(value)
	}
}

// Validate checks all settings needed by the pipeline. Error messages name the
// setting but never include its value, so a URL containing credentials cannot
// be copied into logs accidentally.
func (c Config) Validate() error {
	var failures []string
	if err := validateHTTPBaseURL(c.CompendiumBaseURL, false); err != nil {
		failures = append(failures, CompendiumBaseURLEnv+": "+err.Error())
	}
	if c.CompendiumAPIPath == "" || !strings.HasPrefix(c.CompendiumAPIPath, "/") || strings.ContainsAny(c.CompendiumAPIPath, "?#") {
		failures = append(failures, CompendiumAPIPathEnv+" must be an absolute URL path without a query or fragment")
	}
	if strings.TrimSpace(c.OutputDir) == "" {
		failures = append(failures, OutputDirEnv+" must not be empty")
	}
	if !gameVersionPattern.MatchString(c.GameVersion) {
		failures = append(failures, GameVersionEnv+" must use numeric major.minor.patch form (for example 81.3.0)")
	}
	if c.AWSRegion != "" && !awsRegionPattern.MatchString(c.AWSRegion) {
		failures = append(failures, AWSRegionEnv+" is not a valid AWS region name")
	}
	if c.DataBucket != "" && !validBucket(c.DataBucket) {
		failures = append(failures, DataBucketEnv+" is not a valid S3 bucket name")
	}
	if c.CDNBaseURL != "" {
		if err := validateHTTPBaseURL(c.CDNBaseURL, c.PublishEnabled); err != nil {
			failures = append(failures, CDNBaseURLEnv+": "+err.Error())
		}
	}
	if c.PublishEnabled {
		for _, required := range []struct {
			name  string
			value string
		}{{AWSRegionEnv, c.AWSRegion}, {DataBucketEnv, c.DataBucket}, {CDNBaseURLEnv, c.CDNBaseURL}} {
			if required.value == "" {
				failures = append(failures, required.name+" is required when "+PublishEnabledEnv+" is true")
			}
		}
	}
	if len(failures) != 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(failures, "; "))
	}
	return nil
}

// ValidateS3Publication checks only the deployment settings consumed by the
// independently runnable S3 publication command.
func (c Config) ValidateS3Publication() error {
	var failures []string
	if !awsRegionPattern.MatchString(c.AWSRegion) {
		failures = append(failures, AWSRegionEnv+" is required and must be a valid AWS region name")
	}
	if !validBucket(c.DataBucket) {
		failures = append(failures, DataBucketEnv+" is required and must be a valid S3 bucket name")
	}
	if len(failures) != 0 {
		return fmt.Errorf("invalid S3 publication configuration: %s", strings.Join(failures, "; "))
	}
	return nil
}

func validateHTTPBaseURL(value string, requireHTTPS bool) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("must be an absolute HTTP(S) URL")
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return fmt.Errorf("must use HTTPS when publishing is enabled")
	}
	if parsed.User != nil {
		return fmt.Errorf("must not contain embedded credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("must not contain a query or fragment")
	}
	return nil
}

func validBucket(value string) bool {
	if !bucketPattern.MatchString(value) || strings.Contains(value, "..") {
		return false
	}
	return net.ParseIP(value) == nil
}

func (c Config) CompendiumAPIURL() string {
	return strings.TrimRight(c.CompendiumBaseURL, "/") + c.CompendiumAPIPath
}

func SplitList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (c Config) ValidateFetch() error {
	if err := validateHTTPBaseURL(c.CompendiumBaseURL, false); err != nil {
		return fmt.Errorf("invalid configuration: %s: %s", CompendiumBaseURLEnv, err)
	}
	if c.CompendiumAPIPath == "" || !strings.HasPrefix(c.CompendiumAPIPath, "/") || strings.ContainsAny(c.CompendiumAPIPath, "?#") {
		return fmt.Errorf("invalid configuration: %s must be an absolute URL path without a query or fragment", CompendiumAPIPathEnv)
	}
	if strings.TrimSpace(c.OutputDir) == "" || len(c.Categories) == 0 {
		return fmt.Errorf("invalid configuration: %s and at least one category are required", OutputDirEnv)
	}
	return nil
}

func (c Config) MasterDir() string    { return filepath.Join(c.OutputDir, "master") }
func (c Config) DomainDir() string    { return c.OutputDir }
func (c Config) CandidateDir() string { return filepath.Join(c.OutputDir, "candidate") }
