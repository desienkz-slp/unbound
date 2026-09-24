DO
$do$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'netora') THEN
      CREATE ROLE netora WITH LOGIN PASSWORD 'netora_dev_password';
   END IF;
END
$do$;

SELECT 'CREATE DATABASE netora OWNER netora'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'netora')\gexec

GRANT ALL PRIVILEGES ON DATABASE netora TO netora;
