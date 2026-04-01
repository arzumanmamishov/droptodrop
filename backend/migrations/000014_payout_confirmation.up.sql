-- Update payout status to support confirmation flow
ALTER TABLE payout_records DROP CONSTRAINT IF EXISTS payout_records_status_check;
ALTER TABLE payout_records ADD CONSTRAINT payout_records_status_check
  CHECK (status IN ('pending', 'payment_sent', 'paid', 'disputed', 'failed'));
