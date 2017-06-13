create table metrics (
  "value" double,
  "valueRaw" long,
  "timestamp" Timestamp
) WITH(column_policy = "dynamic");
