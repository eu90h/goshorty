CREATE TABLE urlmap (
    short_url text primary key,
    true_url text,
    creation_time timestamp,
    clicks bigint
);