-- Migration: 002_add_disk_columns_to_nodes.sql
-- Adds disk_total_gb and disk_free_gb to the nodes table for disk-aware tuning recommendations.
-- Safe to run multiple times (uses IF NOT EXISTS via DO block).

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'nodes' AND column_name = 'disk_total_gb'
    ) THEN
        ALTER TABLE nodes ADD COLUMN disk_total_gb INT NOT NULL DEFAULT 0;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'nodes' AND column_name = 'disk_free_gb'
    ) THEN
        ALTER TABLE nodes ADD COLUMN disk_free_gb INT NOT NULL DEFAULT 0;
    END IF;
END $$;
