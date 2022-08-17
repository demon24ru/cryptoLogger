SET allow_experimental_object_type = 1;
SET output_format_json_named_tuples_as_objects = 1;

CREATE TABLE ticker_
(
    `data` JSON,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);

CREATE TABLE trade_
(
    `data` JSON,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);

CREATE TABLE level2_
(
    `data` JSON,
    `timestamp` DateTime64(9, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);

CREATE TABLE ordersbook_
(
    `sequence` String,
    `bids` JSON,
    `asks` JSON,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);