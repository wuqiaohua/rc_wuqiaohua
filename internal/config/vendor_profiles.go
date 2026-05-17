package config

import (
	"fmt"
	"maps"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type VendorProfile struct {
	Vendor         string
	TargetURL      string
	Method         string
	DefaultHeaders map[string]string
	MaxRetries     int
}

var vendorProfiles = map[string]VendorProfile{
	"crm": {
		Vendor:    "crm",
		TargetURL: "http://mock-vendor:9000/ok",
		Method:    "POST",
		DefaultHeaders: map[string]string{
			"Content-Type": "application/json",
		},
		MaxRetries: 5,
	},
}

type vendorProfilesFile struct {
	Vendors map[string]vendorProfileFile `toml:"vendors"`
}

type vendorProfileFile struct {
	TargetURL      string            `toml:"target_url"`
	Method         string            `toml:"method"`
	DefaultHeaders map[string]string `toml:"default_headers"`
	MaxRetries     int               `toml:"max_retries"`
}

func DefaultVendorProfiles() map[string]VendorProfile {
	profiles := make(map[string]VendorProfile, len(vendorProfiles))
	for vendor, profile := range vendorProfiles {
		profile.DefaultHeaders = maps.Clone(profile.DefaultHeaders)
		profiles[vendor] = profile
	}
	return profiles
}

func LoadVendorProfiles(path string) (map[string]VendorProfile, error) {
	if path == "" {
		return nil, fmt.Errorf("vendor profiles path is empty")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file vendorProfilesFile
	if err := toml.Unmarshal(content, &file); err != nil {
		return nil, err
	}
	if len(file.Vendors) == 0 {
		return nil, fmt.Errorf("vendor profiles file %s has no vendors", path)
	}

	profiles := make(map[string]VendorProfile, len(file.Vendors))
	for vendor, profile := range file.Vendors {
		profiles[vendor] = VendorProfile{
			Vendor:         vendor,
			TargetURL:      profile.TargetURL,
			Method:         profile.Method,
			DefaultHeaders: maps.Clone(profile.DefaultHeaders),
			MaxRetries:     profile.MaxRetries,
		}
	}
	return profiles, nil
}
