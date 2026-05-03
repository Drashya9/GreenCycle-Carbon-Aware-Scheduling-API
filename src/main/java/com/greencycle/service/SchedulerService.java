package com.greencycle.service;

import com.greencycle.model.Appliance;
import com.greencycle.model.GridDataPoint;
import com.greencycle.model.OptimalWindow;
import com.greencycle.repository.ApplianceRepository;
import org.springframework.stereotype.Service;

import java.util.List;

/**
 * SchedulerService — the orchestrator (the "use case" layer).
 *
 * This class knows nothing about HTTP or the database schema.
 * Its only job is to wire the three sub-systems together in the right order:
 *
 *  1. Look up the appliance by name  → ApplianceRepository
 *  2. Fetch a 24-hour carbon forecast → GridService (mock or real)
 *  3. Find the best time window       → SlidingWindowService
 *  4. Return the result to the caller → ScheduleController
 *
 * Keeping this logic in a service (rather than the controller) makes it
 * easy to test without starting an HTTP server.
 */
@Service
public class SchedulerService {

    private final ApplianceRepository  applianceRepo;
    private final GridService          gridService;
    private final SlidingWindowService slidingWindow;

    // Constructor injection — the recommended Spring pattern (no @Autowired needed)
    public SchedulerService(ApplianceRepository applianceRepo,
                             GridService gridService,
                             SlidingWindowService slidingWindow) {
        this.applianceRepo = applianceRepo;
        this.gridService   = gridService;
        this.slidingWindow = slidingWindow;
    }

    /**
     * Calculate the optimal start time for a given appliance and location.
     *
     * @param applianceName case-insensitive appliance name (e.g. "Dryer")
     * @param zipCode       user's ZIP code used to locate the grid region
     * @return OptimalWindow with recommendation text and structured metadata
     * @throws IllegalArgumentException if the appliance is not found in the DB
     */
    public OptimalWindow calculate(String applianceName, String zipCode) {

        // ── Step 1: Find the appliance ────────────────────────────────────────
        Appliance appliance = applianceRepo.findByNameIgnoreCase(applianceName)
            .orElseThrow(() -> new IllegalArgumentException(
                "Appliance not found: '" + applianceName + "'. " +
                "Call GET /appliances to see available options."));

        // ── Step 2: Fetch grid forecast ───────────────────────────────────────
        List<GridDataPoint> forecast = gridService.getForecast(zipCode);

        // ── Step 3: Run the sliding window algorithm ──────────────────────────
        return slidingWindow.findOptimalWindow(
            forecast,
            appliance.getDurationHours(),
            appliance.getName()
        );
    }
}
