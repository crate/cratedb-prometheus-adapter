package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
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
	pool     *pgx.ConnPool
	poolConf pgx.ConnPoolConfig
}

func newCrateEndpoint(ep *endpointConfig) *crateEndpoint {
	connConf := pgx.ConnConfig{
		Host:     ep.Host,
		Port:     ep.Port,
		User:     ep.User,
		Password: ep.Password,
		Database: ep.Schema,
	}
	if ep.EnableTLS {
		connConf.TLSConfig = &tls.Config{
			ServerName:         ep.Host,
			InsecureSkipVerify: ep.AllowInsecureTLS,
		}
	}
	poolConf := pgx.ConnPoolConfig{
		ConnConfig:     connConf,
		MaxConnections: ep.MaxConnections,
	}
	return &crateEndpoint{poolConf: poolConf}
}

func (c *crateEndpoint) endpoint() endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		// We initialize the connection pool lazily here instead of in newCrateEndpoint() so
		// that the adapter does not crash on startup if an endpoint is unavailable.
		if c.pool == nil {
			pool, err := pgx.NewConnPool(c.poolConf)
			if err != nil {
				return nil, fmt.Errorf("error opening connection to CrateDB: %v", err)
			}
			c.pool = pool
		}

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

func (c crateEndpoint) write(ctx context.Context, r *crateWriteRequest) error {
	_, err := c.pool.PrepareEx(ctx, "write_statement", crateWriteStatement, nil)
	if err != nil {
		return fmt.Errorf("error preparing write statement: %v", err)
	}

	batch := c.pool.BeginBatch()
	for _, a := range r.rows {
		batch.Queue(
			"write_statement",
			[]interface{}{
				a.labels,
				a.labelsHash,
				// TODO: Find non-string way of encoding timestamps.
				a.timestamp.Format("2006-01-02 15:04:05.000-07"),
				a.value,
				a.valueRaw,
			},
			[]pgtype.OID{
				pgtype.JSONOID,
				pgtype.VarcharOID,
				pgtype.TimestamptzOID,
				pgtype.Float8OID,
				pgtype.Int8OID,
			},
			nil,
		)
	}

	err = batch.Send(ctx, nil)
	if err != nil {
		return fmt.Errorf("error executing write batch: %v", err)
	}

	err = batch.Close()
	if err != nil {
		return fmt.Errorf("error closing write batch: %v", err)
	}
	return nil
}

func (c crateEndpoint) read(ctx context.Context, r *crateReadRequest) (*crateReadResponse, error) {
	rows, err := c.pool.QueryEx(ctx, r.stmt, nil)
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
