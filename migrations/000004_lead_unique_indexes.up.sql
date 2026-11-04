CREATE UNIQUE INDEX leads_customer_phone_uidx ON leads (customer_id, phone);
CREATE UNIQUE INDEX leads_customer_email_uidx ON leads (customer_id, lower(email));
