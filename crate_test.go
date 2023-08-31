package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewCrateEndpoint(t *testing.T) {
	conf := builtinConfig()
	endpoint := newCrateEndpoint(&conf.Endpoints[0])
	require.Equal(t,
		endpoint.poolConf.ConnString(),
		"postgres://crate:@localhost:5432/?connect_timeout=10&pool_max_conns=5")
}
