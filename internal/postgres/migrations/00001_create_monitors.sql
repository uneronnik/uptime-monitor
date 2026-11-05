-- +goose Up
CREATE TABLE monitors (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
    expected_status integer NOT NULL DEFAULT 200 CHECK (expected_status BETWEEN 100 AND 599),
    interval_seconds integer NOT NULL DEFAULT 60 CHECK (interval_seconds >= 30),
    timeout_ms integer NOT NULL DEFAULT 5000 CHECK (timeout_ms > 0 AND timeout_ms::bigint < interval_seconds::bigint * 1000),
    paused boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE monitors;
