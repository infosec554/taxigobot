-- Up Migration
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'wait_confirm';
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'cancelled_by_admin';
