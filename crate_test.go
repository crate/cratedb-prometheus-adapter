package main

import (
	"context"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var CPU_COUNT = int32(runtime.NumCPU())

func TestNewCrateEndpoint(t *testing.T) {
	conf := builtinConfig()
	conf.Endpoints[0].Password = "foobar+&%"
	conf.Endpoints[0].Schema = "testdrive"
	endpoint := newCrateEndpoint(&conf.Endpoints[0])
	require.Equal(t,
		"host=localhost port=5432 user=crate password=foobar+&% database=testdrive connect_timeout=10",
		endpoint.poolConf.ConnString(),
	)
	require.GreaterOrEqual(t, endpoint.poolConf.MaxConns, CPU_COUNT)
}

func TestConfigurePoolDefault(t *testing.T) {
	/**
	 * Verify that, by default, the pool size got configured with one connection per core by default.
	**/
	poolConf, _ := pgxpool.ParseConfig("postgres://crate:foo@localhost:5432/")
	require.Equal(t, "localhost", poolConf.ConnConfig.Host)
	require.GreaterOrEqual(t, poolConf.MaxConns, CPU_COUNT)

	ctx := context.Background()
	pool, err := createPool(ctx, poolConf)
	require.IsType(t, &pgxpool.Pool{}, pool)
	require.Equal(t, err, nil)
	require.GreaterOrEqual(t, pool.Config().MaxConns, CPU_COUNT)
}

func TestConfigurePoolWithPoolSizeFromConnectionString(t *testing.T) {
	/**
	 * Verify that the pool size gets obtained from the database connection string.
	**/
	poolConf, _ := pgxpool.ParseConfig("postgres://crate:foo@localhost:5432/?pool_max_conns=42")
	require.Equal(t, "localhost", poolConf.ConnConfig.Host)
	require.Equal(t, int32(42), poolConf.MaxConns)

	ctx := context.Background()
	pool, err := createPool(ctx, poolConf)
	require.IsType(t, &pgxpool.Pool{}, pool)
	require.Equal(t, err, nil)
	require.Equal(t, int32(42), pool.Config().MaxConns)
}

func TestConfigurePoolWithPoolSizeFromSettingsVanilla(t *testing.T) {
	/**
	 * Verify that the pool size can be configured using a configuration setting.
	**/
	poolConf, _ := pgxpool.ParseConfig("postgres://crate:foo@localhost:5432/")
	require.Equal(t, "localhost", poolConf.ConnConfig.Host)
	require.GreaterOrEqual(t, poolConf.MaxConns, CPU_COUNT)

	ctx := context.Background()
	pool, err := createPoolWithPoolSize(ctx, poolConf, 42)
	require.IsType(t, &pgxpool.Pool{}, pool)
	require.Equal(t, err, nil)
	require.Equal(t, int32(42), pool.Config().MaxConns)
}

func TestConfigurePoolWithPoolSizeFromSettingsPrecedence(t *testing.T) {
	/**
	 * Verify that the pool size configuration setting takes precedence over the connection string.
	**/
	poolConf, _ := pgxpool.ParseConfig("postgres://crate:foo@localhost:5432/?pool_max_conns=33")
	require.Equal(t, "localhost", poolConf.ConnConfig.Host)
	require.Equal(t, int32(33), poolConf.MaxConns)

	ctx := context.Background()
	pool, err := createPoolWithPoolSize(ctx, poolConf, 42)
	require.IsType(t, &pgxpool.Pool{}, pool)
	require.Equal(t, err, nil)
	require.Equal(t, int32(42), pool.Config().MaxConns)
}

func TestPoolsDefault(t *testing.T) {
	/**
	 * Verify connection pool sizes when not configured explicitly.
	**/
	conf := builtinConfig()
	endpoint := newCrateEndpoint(&conf.Endpoints[0])
	ctx := context.Background()
	endpoint.createPools(ctx)
	require.IsType(t, &pgxpool.Pool{}, endpoint.readPool)
	require.Equal(t,
		"host=localhost port=5432 user=crate connect_timeout=10",
		endpoint.poolConf.ConnString(),
	)
	require.GreaterOrEqual(t, endpoint.readPool.Config().MaxConns, CPU_COUNT)
	require.GreaterOrEqual(t, endpoint.writePool.Config().MaxConns, CPU_COUNT)
}

func TestPoolsWithMaxConnections(t *testing.T) {
	/**
	 * Verify connection pool sizes when configured using `MaxConnections`.
	**/
	conf := builtinConfig()
	conf.Endpoints[0].MaxConnections = 42
	endpoint := newCrateEndpoint(&conf.Endpoints[0])
	ctx := context.Background()
	endpoint.createPools(ctx)
	require.IsType(t, &pgxpool.Pool{}, endpoint.readPool)
	require.Equal(t,
		"host=localhost port=5432 user=crate connect_timeout=10 pool_max_conns=42",
		endpoint.poolConf.ConnString(),
	)
	require.Equal(t, int32(42), endpoint.readPool.Config().MaxConns)
	require.Equal(t, int32(42), endpoint.writePool.Config().MaxConns)
}

func TestPoolsWithIndividualPoolSizes(t *testing.T) {
	/**
	 * Verify connection pool sizes when configured using `ReadPoolSize` and `WritePoolSize`.
	**/
	conf := builtinConfig()
	conf.Endpoints[0].ReadPoolSize = 11
	conf.Endpoints[0].WritePoolSize = 22
	endpoint := newCrateEndpoint(&conf.Endpoints[0])
	ctx := context.Background()
	endpoint.createPools(ctx)
	require.IsType(t, &pgxpool.Pool{}, endpoint.readPool)
	require.Equal(t,
		"host=localhost port=5432 user=crate connect_timeout=10",
		endpoint.poolConf.ConnString(),
	)
	require.Equal(t, int32(11), endpoint.readPool.Config().MaxConns)
	require.Equal(t, int32(22), endpoint.writePool.Config().MaxConns)
}

func TestPoolsWithMaxConnectionsAndIndividualPoolSizes(t *testing.T) {
	/**
	 * Verify connection pool sizes when configured using `MaxConnections` and `ReadPoolSize`.
	**/
	conf := builtinConfig()
	conf.Endpoints[0].MaxConnections = 5
	conf.Endpoints[0].ReadPoolSize = 40
	endpoint := newCrateEndpoint(&conf.Endpoints[0])
	ctx := context.Background()
	endpoint.createPools(ctx)
	require.IsType(t, &pgxpool.Pool{}, endpoint.readPool)
	require.Equal(t,
		"host=localhost port=5432 user=crate connect_timeout=10 pool_max_conns=5",
		endpoint.poolConf.ConnString(),
	)
	require.Equal(t, int32(40), endpoint.readPool.Config().MaxConns)
	require.Equal(t, int32(5), endpoint.writePool.Config().MaxConns)
}
