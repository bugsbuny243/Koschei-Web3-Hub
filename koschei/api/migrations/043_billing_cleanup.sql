DELETE FROM entitlements WHERE lower(COALESCE(payment_provider, '')) = 'paddle';
DELETE FROM orders WHERE lower(COALESCE(provider, '')) = 'paddle';
DELETE FROM payment_requests WHERE lower(COALESCE(payment_provider, '')) = 'paddle';

ALTER TABLE IF EXISTS orders ALTER COLUMN provider SET DEFAULT 'shopier';
ALTER TABLE IF EXISTS orders ALTER COLUMN currency SET DEFAULT 'TRY';
ALTER TABLE IF EXISTS api_keys DROP COLUMN IF EXISTS subscription_id;
ALTER TABLE IF EXISTS b2b_api_keys DROP COLUMN IF EXISTS subscription_id;

DROP TABLE IF EXISTS paddle_webhook_events;
DROP TABLE IF EXISTS paddle_subscriptions;
DROP TABLE IF EXISTS paddle_customers;
