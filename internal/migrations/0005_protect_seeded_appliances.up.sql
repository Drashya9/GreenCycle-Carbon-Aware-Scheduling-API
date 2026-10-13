-- The appliances seeded by 0002 are demo fixtures. On a public deployment anyone could
-- delete them for every visitor, so they're flagged here and DELETE refuses them.
ALTER TABLE appliance ADD COLUMN is_seed BOOLEAN NOT NULL DEFAULT false;

UPDATE appliance SET is_seed = true
WHERE name IN ('Dishwasher', 'Washing Machine', 'Dryer', 'EV Charger', 'Pool Pump', 'Water Heater');
