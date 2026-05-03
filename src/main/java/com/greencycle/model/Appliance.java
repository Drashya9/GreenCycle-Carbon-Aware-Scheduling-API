package com.greencycle.model;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * Appliance — a household device that consumes electricity in a single cycle.
 *
 * JPA annotations:
 *   @Entity      — tells Hibernate this class maps to a database table
 *   @Table       — names the table explicitly (optional but good practice)
 *   @Id          — marks the primary key field
 *   @GeneratedValue — Hibernate auto-increments the ID on INSERT
 *
 * Lombok annotations:
 *   @Data        — generates getters, setters, equals, hashCode, toString
 *   @NoArgsConstructor / @AllArgsConstructor — generate constructors
 */
@Entity
@Table(name = "appliance")
@Data
@NoArgsConstructor
@AllArgsConstructor
public class Appliance {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    /**
     * Human-readable name, e.g. "Washing Machine".
     * Must be unique so look-up by name works reliably.
     */
    @Column(nullable = false, unique = true)
    private String name;

    /**
     * How long one full cycle takes, expressed in fractional hours.
     * Examples: 1.5 = 90 minutes, 0.5 = 30 minutes.
     * The sliding-window algorithm converts this to an integer slot count
     * by rounding up: Math.ceil(durationHours).
     */
    @Column(nullable = false)
    private double durationHours;
}
