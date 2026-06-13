DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'products' AND column_name = 'price_try_cents'
  ) THEN
    ALTER TABLE products ALTER COLUMN price_try_cents SET DEFAULT 0;
    UPDATE products SET price_try_cents = 0 WHERE price_try_cents IS NULL;
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'products' AND column_name = 'price_usd_cents'
  ) THEN
    ALTER TABLE products ALTER COLUMN price_usd_cents SET DEFAULT 0;
    UPDATE products SET price_usd_cents = 0 WHERE price_usd_cents IS NULL;
  END IF;
END $$;
