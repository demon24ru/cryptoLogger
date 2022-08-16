CREATE TABLE ticker_
(
    `data` String,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);

CREATE TABLE trade_
(
    `data` String,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);

CREATE TABLE level2_
(
    `data` String,
    `timestamp` DateTime64(9, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);

CREATE TABLE ordersbook_
(
    `bids` String,
    `asks` String,
    `timestamp` DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp);