// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestConfigurationManageSchema(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Pos:         helpers.Mark(),
			Description: "manage-schema set to false",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"manage-schema": false,
					"resolutions": []gin.H{
						{
							"interval": "0",
							"ttl":      "360h",
						},
					},
				}
			},
			Expected: Configuration{
				ManageSchema: false,
				Resolutions: []ResolutionConfiguration{
					{
						Interval: 0,
						TTL:      360 * time.Hour,
					},
				},
			},
			SkipValidation: true,
		},
		{
			Pos:         helpers.Mark(),
			Description: "manage-schema set to true",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"manage-schema": true,
					"resolutions": []gin.H{
						{
							"interval": "0",
							"ttl":      "360h",
						},
					},
				}
			},
			Expected: Configuration{
				ManageSchema: true,
				Resolutions: []ResolutionConfiguration{
					{
						Interval: 0,
						TTL:      360 * time.Hour,
					},
				},
			},
			SkipValidation: true,
		},
	})
}

func TestConfigurationManageSchemaDefault(t *testing.T) {
	conf := DefaultConfiguration()
	if !conf.ManageSchema {
		t.Error("expected ManageSchema to default to true")
	}
}
