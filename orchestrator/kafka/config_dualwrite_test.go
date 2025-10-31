// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
)

func TestConfigurationManageTopic(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Pos:         helpers.Mark(),
			Description: "manage-topic set to false",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"brokers":      []string{"kafka:9092"},
					"topic":        "flows",
					"manage-topic": false,
					"topic-configuration": gin.H{
						"num-partitions":     8,
						"replication-factor": 3,
					},
				}
			},
			Expected: Configuration{
				Configuration: kafka.Configuration{
					Brokers: []string{"kafka:9092"},
					Topic:   "flows",
				},
				ManageTopic: false,
				TopicConfiguration: TopicConfiguration{
					NumPartitions:     8,
					ReplicationFactor: 3,
				},
			},
			SkipValidation: true,
		},
		{
			Pos:         helpers.Mark(),
			Description: "manage-topic set to true",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"brokers":      []string{"kafka:9092"},
					"topic":        "flows",
					"manage-topic": true,
				}
			},
			Expected: Configuration{
				Configuration: kafka.Configuration{
					Brokers: []string{"kafka:9092"},
					Topic:   "flows",
				},
				ManageTopic: true,
			},
			SkipValidation: true,
		},
	})
}

func TestConfigurationManageTopicDefault(t *testing.T) {
	conf := DefaultConfiguration()
	if !conf.ManageTopic {
		t.Error("expected ManageTopic to default to true")
	}
}
