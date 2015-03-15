BEGIN;

DROP TABLE IF EXISTS searches;
DROP TABLE IF EXISTS words;
DROP TABLE IF EXISTS titles;
DROP TABLE IF EXISTS keys;
DROP TABLE IF EXISTS domains;

CREATE TABLE domains (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE INDEX domains_name_idx ON domains(name);
CREATE TABLE keys (
  key TEXT PRIMARY KEY,
  domain_id INTEGER NOT NULL REFERENCES domains ON DELETE CASCADE
);
CREATE TABLE titles (
  id SERIAL PRIMARY KEY,
  domain_id INTEGER NOT NULL REFERENCES domains ON DELETE CASCADE,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  popularity REAL DEFAULT 0.0 NOT NULL,
  group_ TEXT NOT NULL DEFAULT '',
  UNIQUE(domain_id, url)
);
CREATE TABLE words (
  id SERIAL PRIMARY KEY,
  prefix TEXT NOT NULL,
  title_id INTEGER NOT NULL REFERENCES titles ON DELETE CASCADE,
  domain_id INTEGER NOT NULL REFERENCES domains ON DELETE CASCADE,
  UNIQUE(prefix, title_id, domain_id)
);

CREATE UNLOGGED TABLE searches (
  id SERIAL PRIMARY KEY,
  domain_id INTEGER NOT NULL REFERENCES domains ON DELETE CASCADE,
  term TEXT NOT NULL,
  results INTEGER NOT NULL DEFAULT 0,
  date_ TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

COMMIT;


INSERT INTO domains(name)values('peterbe');
INSERT INTO titles(domain_id,title,url)values(1,'Page', '/url');
INSERT INTO words(prefix,title_id,domain_id)values('p',1,1);
INSERT INTO words(prefix,title_id,domain_id)values('pe',1,1);
