// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"testing"

	"akvorado/common/clickhousedb"
	"akvorado/common/helpers"
	"akvorado/outlet/clickhouse"
)

func TestOutletConfigurationDataDestinations(t *testing.T) {
	config := OutletConfiguration{}
	config.DataDestinations = []DataDestination{
		{
			Name: "azure",
			Connection: clickhousedb.Configuration{
				Servers:    []string{"azure:9440"},
				Database:   "flows",
				Username:   "user",
				MaxRetries: 3,
			},
		},
	}

	if len(config.DataDestinations) != 1 {
		t.Errorf("expected 1 data destination, got %d", len(config.DataDestinations))
	}
	if config.DataDestinations[0].Name != "azure" {
		t.Errorf("expected name 'azure', got %q", config.DataDestinations[0].Name)
	}
	if config.DataDestinations[0].Connection.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", config.DataDestinations[0].Connection.MaxRetries)
	}
}

func TestOutletConfigurationWithOverrides(t *testing.T) {
	defaultConfig := clickhouse.DefaultConfiguration()
	defaultConfig.MaximumBatchSize = 50000

	overrideConfig := clickhouse.DefaultConfiguration()
	overrideConfig.MaximumBatchSize = 30000

	config := OutletConfiguration{
		ClickHouse: defaultConfig,
		DataDestinations: []DataDestination{
			{
				Name: "azure",
				Connection: clickhousedb.Configuration{
					Servers:  []string{"azure:9440"},
					Database: "flows",
					Username: "user",
				},
				ClickHouse: &overrideConfig,
			},
		},
	}

	if config.ClickHouse.MaximumBatchSize != 50000 {
		t.Errorf("expected default batch size 50000, got %d", config.ClickHouse.MaximumBatchSize)
	}
	if config.DataDestinations[0].ClickHouse.MaximumBatchSize != 30000 {
		t.Errorf("expected override batch size 30000, got %d", config.DataDestinations[0].ClickHouse.MaximumBatchSize)
	}
}

func TestOutletConfigurationBackwardCompatibility(t *testing.T) {
	config := OutletConfiguration{
		ClickHouseDB: clickhousedb.Configuration{
			Servers:  []string{"127.0.0.1:9000"},
			Database: "flows",
			Username: "default",
		},
	}

	if len(config.ClickHouseDB.Servers) == 0 {
		t.Error("expected ClickHouseDB to be configured")
	}
	if len(config.DataDestinations) != 0 {
		t.Errorf("expected 0 data destinations, got %d", len(config.DataDestinations))
	}
}

func TestOutletConfigurationValidation(t *testing.T) {
	config := OutletConfiguration{
		DataDestinations: []DataDestination{
			{
				Connection: clickhousedb.Configuration{
					Servers:  []string{"azure:9440"},
					Database: "flows",
					Username: "user",
				},
			},
		},
	}

	err := helpers.Validate.Struct(config)
	if err == nil {
		t.Error("expected validation error for missing name")
	}
}
