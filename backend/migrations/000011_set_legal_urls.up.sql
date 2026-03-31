UPDATE app_settings SET
  privacy_policy_url = 'https://droptodrop.osc-fr1.scalingo.io/privacy',
  terms_url = 'https://droptodrop.osc-fr1.scalingo.io/terms'
WHERE privacy_policy_url IS NULL OR privacy_policy_url = '';
