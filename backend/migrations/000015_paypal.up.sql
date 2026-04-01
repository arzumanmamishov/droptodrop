ALTER TABLE supplier_profiles ADD COLUMN IF NOT EXISTS paypal_email VARCHAR(255);
ALTER TABLE reseller_profiles ADD COLUMN IF NOT EXISTS paypal_email VARCHAR(255);
