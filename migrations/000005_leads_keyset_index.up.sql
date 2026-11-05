CREATE INDEX leads_customer_created_id_idx
    ON leads (customer_id, created_at DESC, id DESC);
