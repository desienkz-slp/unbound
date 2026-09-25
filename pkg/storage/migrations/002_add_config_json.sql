-- Migration 002: Add config_json to nodes table for full desired state persistence
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS config_json JSONB;
