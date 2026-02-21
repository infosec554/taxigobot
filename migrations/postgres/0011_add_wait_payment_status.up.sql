-- Up Migration
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'wait_payment';
