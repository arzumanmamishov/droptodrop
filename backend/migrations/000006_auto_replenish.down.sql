ALTER TABLE reseller_imports DROP COLUMN IF EXISTS auto_replenish;
ALTER TABLE reseller_imports DROP COLUMN IF EXISTS replenish_threshold;
ALTER TABLE reseller_imports DROP COLUMN IF EXISTS replenish_quantity;
ALTER TABLE shops DROP COLUMN IF EXISTS parent_shop_id;
