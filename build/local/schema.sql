CREATE TABLE IF NOT EXISTS messages (
       uid    text,
       bid    text,             -- bodies.bid
       sender text,
       receivers text,
       subject text,
       status integer,
       last_update bigint,
       last_error text
);

CREATE TABLE IF NOT EXISTS bodies (
       bid    text,
       packet text
);
