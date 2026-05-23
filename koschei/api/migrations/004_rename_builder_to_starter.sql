-- Rename Builder plan to Starter for existing databases.
INSERT INTO plans (id, name, price_try, monthly_credits, is_active)
VALUES ('starter', 'Starter', 899, 20000, true)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    price_try = EXCLUDED.price_try,
    monthly_credits = EXCLUDED.monthly_credits,
    is_active = EXCLUDED.is_active;

DELETE FROM plans WHERE id = 'builder';
