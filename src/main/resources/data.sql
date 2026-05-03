-- data.sql is executed automatically by Spring Boot after Hibernate creates the schema.
-- This seeds the H2 database with a set of common household appliances.

INSERT INTO appliance (name, duration_hours) VALUES ('Dishwasher',    1.5);
INSERT INTO appliance (name, duration_hours) VALUES ('Washing Machine', 1.0);
INSERT INTO appliance (name, duration_hours) VALUES ('Dryer',          1.0);
INSERT INTO appliance (name, duration_hours) VALUES ('EV Charger',     8.0);
INSERT INTO appliance (name, duration_hours) VALUES ('Pool Pump',      4.0);
INSERT INTO appliance (name, duration_hours) VALUES ('Water Heater',   2.0);
