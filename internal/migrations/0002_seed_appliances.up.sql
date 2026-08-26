INSERT INTO appliance (name, duration_hours) VALUES
    ('Dishwasher',       1.5),
    ('Washing Machine',  1.0),
    ('Dryer',            1.0),
    ('EV Charger',       8.0),
    ('Pool Pump',        4.0),
    ('Water Heater',     2.0)
ON CONFLICT (name) DO NOTHING;
