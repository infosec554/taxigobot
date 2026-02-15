-- Up Migration
ALTER TYPE order_status ADD VALUE 'pending' BEFORE 'active';
