package com.greencycle.service;

import com.greencycle.model.GridDataPoint;
import com.greencycle.model.OptimalWindow;
import org.springframework.stereotype.Service;

import java.time.format.DateTimeFormatter;
import java.util.List;

/**
 * SlidingWindowService — the heart of the application.
 *
 * Given a 24-hour carbon forecast and a window size (in hours), this service
 * finds the contiguous block of hours with the lowest average carbon intensity.
 *
 * ─── Algorithm (Sliding Window) ──────────────────────────────────────────────
 *
 *  Forecast: [180, 160, 145, 135, 130, 140, 180, 220, ...]  (24 values)
 *  Window size: 3 hours
 *
 *  Step 1 — Compute initial window sum: hours[0] + hours[1] + hours[2]
 *  Step 2 — "Slide" right: subtract hours[i-1], add hours[i+windowSize-1]
 *            This avoids re-summing the entire window each iteration → O(n)
 *  Step 3 — Track the minimum sum and the starting index of that minimum.
 *  Step 4 — Return the OptimalWindow anchored at that starting index.
 *
 *  Time complexity:  O(n)  — a single pass through the forecast
 *  Space complexity: O(1)  — only a handful of variables regardless of input size
 *
 * ─────────────────────────────────────────────────────────────────────────────
 */
@Service
public class SlidingWindowService {

    private static final DateTimeFormatter DISPLAY_FORMAT =
            DateTimeFormatter.ofPattern("h:mm a z");

    /**
     * Find the optimal start time for an appliance cycle.
     *
     * @param forecast      24 GridDataPoint objects, one per hour, in chronological order
     * @param durationHours appliance cycle length in (possibly fractional) hours
     * @param applianceName display name for the recommendation string
     * @return OptimalWindow describing the best time slot found
     * @throws IllegalArgumentException if the forecast is too short for the window
     */
    public OptimalWindow findOptimalWindow(List<GridDataPoint> forecast,
                                           double durationHours,
                                           String applianceName) {

        // Convert fractional hours to integer slot count (ceiling).
        // e.g. 1.5 hours → 2 slots,  3.0 hours → 3 slots
        int windowSize = (int) Math.ceil(durationHours);

        if (forecast == null || forecast.size() < windowSize) {
            throw new IllegalArgumentException(
                "Forecast has " + (forecast == null ? 0 : forecast.size()) +
                " hours but appliance needs " + windowSize + " hours.");
        }

        // ── Phase 1: compute sum of the first window ──────────────────────────
        double windowSum = 0;
        for (int i = 0; i < windowSize; i++) {
            windowSum += forecast.get(i).getCarbonIntensity();
        }

        double minSum      = windowSum;
        int    minStartIdx = 0;

        // ── Phase 2: slide the window through the rest of the forecast ────────
        for (int i = 1; i <= forecast.size() - windowSize; i++) {
            // Remove the element leaving the window from the left
            windowSum -= forecast.get(i - 1).getCarbonIntensity();
            // Add the new element entering the window from the right
            windowSum += forecast.get(i + windowSize - 1).getCarbonIntensity();

            if (windowSum < minSum) {
                minSum      = windowSum;
                minStartIdx = i;
            }
        }

        // ── Phase 3: build the result ─────────────────────────────────────────
        double avgIntensity = Math.round((minSum / windowSize) * 10.0) / 10.0;

        GridDataPoint startSlot = forecast.get(minStartIdx);
        GridDataPoint endSlot   = forecast.get(minStartIdx + windowSize - 1);

        String startFormatted = startSlot.getTimestamp().format(DISPLAY_FORMAT);
        String endFormatted   = endSlot.getTimestamp().plusHours(1).format(DISPLAY_FORMAT);

        String recommendation = String.format(
            "Run your %s at %s — avg. %.0f gCO₂/kWh (done by %s).",
            applianceName, startFormatted, avgIntensity, endFormatted);

        return new OptimalWindow(
            startSlot.getTimestamp(),
            endSlot.getTimestamp().plusHours(1),
            avgIntensity,
            applianceName,
            recommendation
        );
    }
}
