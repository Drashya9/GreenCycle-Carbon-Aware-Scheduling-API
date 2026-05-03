package com.greencycle.service;

import com.greencycle.model.GridDataPoint;

import java.util.List;

/**
 * GridService — contract for fetching a 24-hour carbon intensity forecast.
 *
 * Using an interface here is intentional: it lets us swap providers
 * without changing any other code.  The active implementation is chosen
 * by the "grid.provider" property in application.properties.
 *
 * Current implementations:
 *   MockGridService        — generates synthetic data, no API key needed
 *   ElectricityMapsService — calls api.electricitymap.org  (real data)
 *   WattTimeService        — calls api2.watttime.org        (real data)
 */
public interface GridService {

    /**
     * Fetch a 24-hour carbon forecast for the region that contains zipCode.
     *
     * @param zipCode  US ZIP code entered by the user (e.g. "85281")
     * @return         list of 24 GridDataPoint objects, one per hour,
     *                 ordered chronologically starting from the current hour
     */
    List<GridDataPoint> getForecast(String zipCode);
}
