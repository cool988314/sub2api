-- Seed new affiliate program settings for signup reward and transfer threshold.
INSERT INTO settings (key, value, updated_at)
VALUES
  ('affiliate_signup_reward', '3.00000000', NOW()),
  ('affiliate_transfer_threshold', '5.00000000', NOW())
ON CONFLICT (key) DO NOTHING;

-- Adjust the legacy untouched default rebate rate from 20% to 10%.
-- This only updates common representations of the historical default.
UPDATE settings
SET value = '10.00000000',
    updated_at = NOW()
WHERE key = 'affiliate_rebate_rate'
  AND TRIM(value) IN ('20', '20.0', '20.00', '20.00000000');
