package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadValidConfiguration(t *testing.T) {
	values := map[string]string{
		CompendiumBaseURLEnv: "http://compendium.internal",
		CompendiumAPIPathEnv: "/w/api.php",
		OutputDirEnv:         "/tmp/output",
		ManualInputDirEnv:    "/tmp/manual",
		AWSRegionEnv:         "us-east-2",
		DataBucketEnv:        "yourddo-data-prod",
		CDNBaseURLEnv:        "https://data.example.com",
		GameVersionEnv:       "81.3.0",
		PublishEnabledEnv:    "true",
		WarningsAsErrorsEnv:  "true",
	}
	cfg, err := load(mapLookup(values))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CompendiumAPIURL() != "http://compendium.internal/w/api.php" || !cfg.PublishEnabled || !cfg.WarningsAsErrors {
		t.Fatalf("configuration = %#v", cfg)
	}
}

func TestLoadUsesSafeLocalDefaults(t *testing.T) {
	cfg, err := load(mapLookup(map[string]string{GameVersionEnv: "81.3.0"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublishEnabled || cfg.DataBucket != "" || cfg.AWSRegion != "" || cfg.CDNBaseURL != "" {
		t.Fatalf("unsafe publication defaults = %#v", cfg)
	}
	if cfg.OutputDir != defaultOutputDir || cfg.ManualInputDir != defaultManualInputDir || cfg.CompendiumAPIPath != defaultCompendiumAPIPath {
		t.Fatalf("local defaults = %#v", cfg)
	}
	if want := []string{"All"}; !reflect.DeepEqual(cfg.Categories, want) {
		t.Fatalf("default categories = %v, want %v", cfg.Categories, want)
	}
}

func TestLoadRejectsInvalidConfigurations(t *testing.T) {
	tests := []struct {
		name        string
		values      map[string]string
		wantInError string
	}{
		{name: "missing game version", values: map[string]string{}, wantInError: GameVersionEnv},
		{name: "invalid game version", values: map[string]string{GameVersionEnv: "Update 81"}, wantInError: GameVersionEnv},
		{name: "invalid boolean", values: map[string]string{GameVersionEnv: "81.3.0", PublishEnabledEnv: "sometimes"}, wantInError: PublishEnabledEnv},
		{name: "invalid warning policy", values: map[string]string{GameVersionEnv: "81.3.0", WarningsAsErrorsEnv: "sometimes"}, wantInError: WarningsAsErrorsEnv},
		{name: "empty manual input", values: map[string]string{GameVersionEnv: "81.3.0", ManualInputDirEnv: ""}, wantInError: ManualInputDirEnv},
		{name: "relative API path", values: map[string]string{GameVersionEnv: "81.3.0", CompendiumAPIPathEnv: "api.php"}, wantInError: CompendiumAPIPathEnv},
		{name: "publish fields required", values: map[string]string{GameVersionEnv: "81.3.0", PublishEnabledEnv: "true"}, wantInError: AWSRegionEnv},
		{name: "invalid bucket", values: map[string]string{GameVersionEnv: "81.3.0", DataBucketEnv: "Not_A_Bucket"}, wantInError: DataBucketEnv},
		{name: "publishing CDN must use HTTPS", values: map[string]string{
			GameVersionEnv: "81.3.0", PublishEnabledEnv: "true", AWSRegionEnv: "us-east-2",
			DataBucketEnv: "yourddo-data-prod", CDNBaseURLEnv: "http://data.example.com",
		}, wantInError: CDNBaseURLEnv},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := load(mapLookup(test.values))
			if err == nil || !strings.Contains(err.Error(), test.wantInError) {
				t.Fatalf("error = %v, want one containing %q", err, test.wantInError)
			}
		})
	}
}

func TestValidationDoesNotExposeURLCredentials(t *testing.T) {
	secret := "do-not-log-this"
	values := map[string]string{
		GameVersionEnv: "81.3.0", CompendiumBaseURLEnv: "https://user:" + secret + "@example.com",
	}
	_, err := load(mapLookup(values))
	if err == nil {
		t.Fatal("load succeeded, want embedded credentials rejected")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed a secret: %v", err)
	}
}

func TestValidateS3Publication(t *testing.T) {
	t.Parallel()
	if err := (Config{AWSRegion: "us-east-2", DataBucket: "yourddo-data-prod"}).ValidateS3Publication(); err != nil {
		t.Fatal(err)
	}
	err := (Config{}).ValidateS3Publication()
	if err == nil || !strings.Contains(err.Error(), AWSRegionEnv) || !strings.Contains(err.Error(), DataBucketEnv) {
		t.Fatalf("error = %v", err)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, exists := values[name]
		return value, exists
	}
}
