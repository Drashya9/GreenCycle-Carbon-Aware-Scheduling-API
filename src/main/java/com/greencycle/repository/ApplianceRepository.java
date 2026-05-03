package com.greencycle.repository;

import com.greencycle.model.Appliance;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

/**
 * ApplianceRepository — data access layer for the APPLIANCE table.
 *
 * Spring Data JPA generates the implementation at startup.
 * We inherit standard CRUD methods (save, findById, findAll, deleteById, …)
 * from JpaRepository<Appliance, Long> for free.
 *
 * The custom query method below follows Spring Data's naming convention:
 *   findBy<FieldName>IgnoreCase  →  SELECT * FROM appliance WHERE LOWER(name) = LOWER(?)
 * No SQL or JPQL needed — Spring parses the method name and builds the query.
 */
@Repository
public interface ApplianceRepository extends JpaRepository<Appliance, Long> {

    /**
     * Case-insensitive look-up so "dryer", "Dryer", and "DRYER" all match.
     * Returns Optional.empty() when no row is found — avoids NullPointerException.
     */
    Optional<Appliance> findByNameIgnoreCase(String name);
}
