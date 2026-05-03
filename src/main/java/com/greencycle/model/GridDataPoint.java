package com.greencycle.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.ZonedDateTime;

/**
 * GridDataPoint — one hourly slot in the 24-hour carbon forecast.
 *
 * This is a plain Java object (no @Entity) because we never persist
 * grid data to the database; it's fetched live from the external API
 * and discarded after the sliding-window calculation completes.
 *
 * Fields:
 *   timestamp      — the start of this hourly slot (timezone-aware)
 *   carbonIntensity — grams of CO₂ per kilowatt-hour for this slot.
 *                     Lower  = cleaner (more solar/wind on the grid).
 *                     Higher = dirtier (more coal/gas in the mix).
 *
 * Typical real-world range: 50 g/kWh (very green) to 600 g/kWh (very dirty).
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class GridDataPoint {

    private ZonedDateTime timestamp;

    /**
     * Carbon intensity in grams of CO₂ equivalent per kilowatt-hour (gCO₂eq/kWh).
     */
    private double carbonIntensity;
}
