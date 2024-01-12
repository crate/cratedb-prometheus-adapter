# Backlog

## Iteration +1
- Make schema **and** table name configurable
- Regular expressions not working
  - https://github.com/crate/cratedb-prometheus-adapter/issues/24
  - https://prometheus.io/docs/prometheus/latest/querying/basics/
  - https://prometheus.io/docs/prometheus/latest/querying/examples/
- Expose metrics about both database connection pools
  https://github.com/crate/cratedb-prometheus-adapter/pull/105
 
## Iteration +2
- Document how to connect to CrateDB Cloud
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
