package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/common/model"
)

const crateWriteStatement = `INSERT INTO metrics ("labels", "labels_hash", "timestamp", "value", "valueRaw") VALUES ($1, $2, $3, $4, $5)`

type crateRow struct {
	labels     model.Metric
	labelsHash string
	timestamp  time.Time
	value      float64
	valueRaw   int64
}

type crateWriteRequest struct {
	rows []*crateRow
}

type crateReadRequest struct {
	stmt string
}

type crateReadResponse struct {
	rows []*crateRow
}

type crateEndpoint struct {
	poolConf      *pgxpool.Config
	readPoolSize  int
	writePoolSize int
	readTimeout   time.Duration
	writeTimeout  time.Duration
	readPool      *pgxpool.Pool
	writePool     *pgxpool.Pool
}

func newCrateEndpoint(ep *endpointConfig) *crateEndpoint {

	// pgx4 starts using connection strings exclusively, in both URL and DSN formats.
	// The single entrypoint to obtain a valid configuration object, is `pgx.ParseConfig`,
	// which aims to be compatible with libpq.

	// ParseConfig builds a *Config from connString with similar behavior to the PostgreSQL
	// standard C library libpq. It uses the same defaults as libpq (e.g. port=5432), and
	// understands most PG* environment variables.
	//
	// ParseConfig closely matches the parsing behavior of libpq. connString may either be
	// in URL or DSN format. connString also may be empty to only read from the environment.
	// If a password is not supplied it will attempt to read the .pgpass file.
	//
	// See https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING for details.
	//
	//   # Example DSN
	//   user=jack password=secret host=pg.example.com port=5432 dbname=mydb sslmode=verify-ca
	//
	//   # Example URL
	//   postgres://jack:secret@pg.example.com:5432/mydb?sslmode=verify-ca

	// Create configuration object from DSN-style connection string.
	poolConf, err := pgxpool.ParseConfig(ep.toDSN())
	if err != nil {
		return nil
	}

	// Configure TLS settings.
	if ep.EnableTLS {
		poolConf.ConnConfig.TLSConfig = &tls.Config{
			ServerName:         ep.Host,
			InsecureSkipVerify: ep.AllowInsecureTLS,
		}
	}

	// pgx v4
	// If you are using `pgxpool`, then you can use `AfterConnect` to prepare statements. That will
	// ensure that they are available on every connection. Otherwise, you will have to acquire
	// a connection from the pool manually and prepare it there before use.
	// https://github.com/jackc/pgx/issues/791#issuecomment-660508309
	poolConf.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {

		// Switch to different database schema when requested.
		if ep.Schema != "" {
			_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO '%s';", ep.Schema))
			if err != nil {
				return fmt.Errorf("error setting search path: %v", err)
			}
		}

		_, err := conn.Prepare(ctx, "write_statement", crateWriteStatement)
		if err != nil {
			return fmt.Errorf("error preparing write statement: %v", err)
		}
		return err
	}
	return &crateEndpoint{
		poolConf:      poolConf,
		readPoolSize:  ep.ReadPoolSize,
		writePoolSize: ep.WritePoolSize,
		readTimeout:   time.Duration(ep.ReadTimeout) * time.Second,
		writeTimeout:  time.Duration(ep.WriteTimeout) * time.Second,
	}
}

func (c *crateEndpoint) endpoint() endpoint.Endpoint {
	/**
	 * Initialize connection pools lazily here instead of in `newCrateEndpoint()`,
	 * so that the adapter does not crash on startup if the endpoint is unavailable.
	**/
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {

		// Initialize database connection pools.
		err = c.createPools(ctx)

		// Dispatch by request type.
		switch r := request.(type) {
		case *crateWriteRequest:
			return nil, c.write(ctx, r)
		case *crateReadRequest:
			return c.read(ctx, r)
		default:
			panic("unknown request type")
		}
	}
}

func (c *crateEndpoint) createPools(ctx context.Context) (err error) {
	/**
	 * Initialize two connection pools, one for read/write each.
	**/
	c.readPool, err = createPoolWithPoolSize(ctx, c.poolConf.Copy(), c.readPoolSize)
	if c.readPool == nil {
		c.readPool, err = createPoolWithPoolSize(ctx, c.poolConf.Copy(), c.readPoolSize)
		if err != nil {
			return err
		}
	}
	if c.writePool == nil {
		c.writePool, err = createPoolWithPoolSize(ctx, c.poolConf.Copy(), c.writePoolSize)
		if err != nil {
			return err
		}
	}
	return nil
}

func createPool(ctx context.Context, poolConf *pgxpool.Config) (pool *pgxpool.Pool, err error) {
	pool, err = pgxpool.NewWithConfig(ctx, poolConf)
	if err != nil {
		return nil, fmt.Errorf("error opening connection to CrateDB: %v", err)
	} else {
		return pool, nil
	}
}

func createPoolWithPoolSize(ctx context.Context, poolConf *pgxpool.Config, maxConns int) (pool *pgxpool.Pool, err error) {
	if maxConns != 0 {
		poolConf.MaxConns = int32(maxConns)
	}
	return createPool(ctx, poolConf)
}

func (c crateEndpoint) write(ctx context.Context, r *crateWriteRequest) error {
	batch := &pgx.Batch{}
	for _, a := range r.rows {
		batch.Queue(
			"write_statement",
			a.labels,
			a.labelsHash,
			// TODO: Find non-string way of encoding timestamps.
			//       Maybe it is more efficient to submit timestamp as Unixtime,
			//       instead of converting it into a string?
			a.timestamp.Format("2006-01-02 15:04:05.000-07"),
			a.value,
			a.valueRaw,
		)
	}

	// pgx4 implements query timeouts using context cancellation.

	// In production applications, it is *always* preferred to have timeouts for all queries:
	// A sudden increase in throughput or a network issue can lead to queries slowing down by
	// orders of magnitude.
	//
	// Slow queries block the connections that they are running on, preventing other queries
	// from running on them. We should always set a timeout after which to cancel a running
	// query, to unblock connections in these cases.
	//
	// -- https://www.sohamkamani.com/golang/sql-database/#query-timeouts---using-context-cancellation

	// `Send` sends all queued queries to the server at once. If the batch is created from a `conn`
	// Object, then *all* queries are wrapped in a transaction. The transaction can optionally be
	// configured with `txOptions`. The context is in effect until the Batch is closed.
	//
	// Warning: `Send` writes all queued queries before reading any results. This can cause a
	// deadlock if an excessive number of queries are queued. It is highly advisable to use a
	// timeout context to protect against this possibility. Unfortunately, this excessive number
	// can vary based on operating system, connection type (TCP or Unix domain socket), and type
	// of query. Unix domain sockets seem to be much more susceptible to this issue than TCP
	// connections. However, it is usually at least several thousand.
	//
	// The deadlock occurs when the batched queries to be sent are so large that the PostgreSQL
	// server cannot receive it all at once. PostgreSQL received some queued queries and starts
	// executing them. As PostgreSQL executes the queries it sends responses back. pgx will not
	// read any of these responses until it has finished sending. Therefore, if all network
	// buffers are full, pgx will not be able to finish sending the queries, and PostgreSQL will
	// not be able to finish sending the responses.
	//
	// -- https://github.com/jackc/pgx/blob/v3.6.2/batch.go#L58-L79
	//
	ctx, _ = context.WithTimeout(ctx, c.writeTimeout)

	batchResults := c.writePool.SendBatch(ctx, batch)
	var qerr error
	if qerr != nil {
		return fmt.Errorf("error executing write batch: %v", qerr)
	}

	err := batchResults.Close()
	if err != nil {
		return fmt.Errorf("error closing write batch: %v", err)
	}
	return nil
}

func (c crateEndpoint) read(ctx context.Context, r *crateReadRequest) (*crateReadResponse, error) {
	// pgx4 implements query timeouts using context cancellation.
	// See `write` function for more details.
	ctx, _ = context.WithTimeout(ctx, c.readTimeout)
	rows, err := c.readPool.Query(ctx, r.stmt)
	if err != nil {
		return nil, fmt.Errorf("error executing read request query: %v", err)
	}
	defer rows.Close()

	resp := &crateReadResponse{}

	for rows.Next() {
		rr := &crateRow{}
		timestamp := pgtype.Timestamptz{}
		if err := rows.Scan(&rr.labels, &rr.labelsHash, &timestamp, &rr.value, &rr.valueRaw); err != nil {
			return nil, fmt.Errorf("error scanning read request rows: %v", err)
		}
		rr.timestamp = timestamp.Time
		resp.rows = append(resp.rows, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through read request rows: %v", err)
	}
	return resp, nil
}
