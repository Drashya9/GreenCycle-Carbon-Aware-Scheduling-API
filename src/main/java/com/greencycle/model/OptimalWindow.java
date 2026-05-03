package com.greencycle.model;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.ZonedDateTime;

/**
 * OptimalWindow — the algorithm's answer: the cleanest time block found.
 *
 * Returned as JSON from the REST endpoint and also used internally
 * when building the human-readable recommendation string.
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class OptimalWindow {

    /** When the user should press Start. */
    private ZonedDateTime startTime;

    /** When the cycle will finish. */
    private ZonedDateTime endTime;

    /**
     * Average carbon intensity across the window, in gCO₂eq/kWh.
     * Presented to the user so they can understand "how clean" the window is.
     */
    private double averageCarbonIntensity;

    /** The appliance this recommendation is for, e.g. "Dryer". */
    private String applianceName;

    /**
     * Plain-English recommendation ready to display in a UI.
     * Example: "Run your Dryer at 2:00 AM — avg. 85 gCO₂/kWh"
     */
    private String recommendation;
}
