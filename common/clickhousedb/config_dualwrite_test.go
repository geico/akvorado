// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"testing"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestConfigurationMaxRetries(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Pos:         helpers.Mark(),
			Description: "max-retries set to 3",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"servers":     []string{"clickhouse:9000"},
					"database":    "flows",
					"username":    "default",
					"max-retries": 3,
				}
			},
			Expected: Configuration{
				Servers:    []string{"clickhouse:9000"},
				Database:   "flows",
				Username:   "default",
				MaxRetries: 3,
			},
			SkipValidation: true,
		},
		{
			Pos:         helpers.Mark(),
			Description: "max-retries set to 0 (infinite)",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"servers":     []string{"clickhouse:9000"},
					"database":    "flows",
					"username":    "default",
					"max-retries": 0,
				}
			},
			Expected: Configuration{
				Servers:    []string{"clickhouse:9000"},
				Database:   "flows",
				Username:   "default",
				MaxRetries: 0,
			},
			SkipValidation: true,
		},
	})
}

func TestConfigurationMaxRetriesDefault(t *testing.T) {
	conf := DefaultConfiguration()
	if conf.MaxRetries != 0 {
		t.Errorf("expected MaxRetries to default to 0, got %d", conf.MaxRetries)
	}
}

func TestConfigurationMaxRetriesValidation(t *testing.T) {
	conf := Configuration{
		Servers:    []string{"clickhouse:9000"},
		Database:   "flows",
		Username:   "default",
		MaxRetries: -1,
	}

	err := helpers.Validate.Struct(conf)
	if err == nil {
		t.Error("expected validation error for negative max-retries")
	}
}
