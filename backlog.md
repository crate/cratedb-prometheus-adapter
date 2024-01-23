# Backlog

## Iteration +1
- Make `tablename` setting configurable. Right now, it is hard-coded to `schema`
- Expose metrics about both database connection pools
  https://github.com/crate/cratedb-prometheus-adapter/pull/105

## Iteration +2
- Document how to connect to CrateDB Cloud
- Log flooding:
  ```log
  ts=2024-01-14T00:27:24.941Z caller=server.go:349 level=error msg="Failed to write data to CrateDB" err="error closing write batch: error preparing write statement: ERROR: Relation 'metrics' unknown (SQLSTATE 42P01)"
  ```
- Implement "Subquery": https://prometheus.io/docs/prometheus/latest/querying/examples/#subquery
- Improve documentation
  https://community.crate.io/t/storing-long-term-metrics-with-prometheus-in-cratedb/1012

## Iteration +3
- Add more defaults from xxx
- Adjust default timeout values to best practices
- Dissolve discrete connection parameters, use connection string only
- Check how configuration per environment variables works
- Think about using `prometheus.Labels` instead of metric prefixes
- Also prepare SQL statement used for reading?
- Refactor metric names
  > Generated/dynamic metric names are a sign that you should be using labels instead.
  > -- https://prometheus.io/docs/instrumenting/writing_clientlibs/#metric-names


## Done
- Regular expressions not working?
  - https://github.com/crate/cratedb-prometheus-adapter/issues/24
  - https://prometheus.io/docs/prometheus/latest/querying/basics/
  - https://prometheus.io/docs/prometheus/latest/querying/examples/
- Make `schema` setting configurable
