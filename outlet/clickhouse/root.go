// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles flow exports to ClickHouse. This component is
// "inert" and does not track its spawned workers. It is the responsability of
// the dependent component to flush data before shutting down.
package clickhouse

import (
	"akvorado/common/clickhousedb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Component is the interface for the ClickHouse exporter component.
type Component interface {
	NewWorker(int, *schema.FlowMessage) Worker
}

// realComponent implements the ClickHouse exporter
type realComponent struct {
	r            *reporter.Reporter
	d            *Dependencies
	destinations []destinationConfig // destinations[0] is primary

	metrics metrics
}

// destinationConfig holds the normalized configuration for a single ClickHouse destination
type destinationConfig struct {
	name   string
	db     *clickhousedb.Component
	config Configuration
}

// DestinationDependency defines a ClickHouse destination with its configuration
type DestinationDependency struct {
	Name       string
	ClickHouse *clickhousedb.Component
	Config     Configuration
}

// Dependencies defines the dependencies of the ClickHouse exporter
type Dependencies struct {
	Schema       *schema.Component
	Destinations []DestinationDependency
}

// New creates a new core component.
func New(r *reporter.Reporter, dependencies Dependencies) (Component, error) {
	c := realComponent{
		r:            r,
		d:            &dependencies,
		destinations: make([]destinationConfig, 0, len(dependencies.Destinations)),
	}

	// Populate normalized destinations from dependencies
	for _, dest := range dependencies.Destinations {
		c.destinations = append(c.destinations, destinationConfig{
			name:   dest.Name,
			db:     dest.ClickHouse,
			config: dest.Config,
		})
	}

	c.initMetrics()
	return &c, nil
}

// primaryConfig returns the configuration of the primary destination
func (c *realComponent) primaryConfig() Configuration {
	if len(c.destinations) == 0 {
		return Configuration{}
	}
	return c.destinations[0].config
}
