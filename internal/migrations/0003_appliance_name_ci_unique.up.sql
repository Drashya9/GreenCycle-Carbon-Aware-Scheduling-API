-- Appliance names are looked up case-insensitively and trimmed, so they must also be unique
-- that way: "Dryer", "dryer" and " Dryer " previously coexisted and made lookups ambiguous.
-- Fails if such duplicates already exist; remove them before applying.
CREATE UNIQUE INDEX appliance_name_ci_key ON appliance (LOWER(TRIM(name)));
