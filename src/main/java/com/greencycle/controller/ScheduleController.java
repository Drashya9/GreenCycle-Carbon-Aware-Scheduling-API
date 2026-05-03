package com.greencycle.controller;

import com.greencycle.model.Appliance;
import com.greencycle.model.OptimalWindow;
import com.greencycle.repository.ApplianceRepository;
import com.greencycle.service.SchedulerService;
import jakarta.validation.constraints.NotBlank;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

/**
 * ScheduleController — exposes the REST API.
 *
 * @RestController = @Controller + @ResponseBody on every method
 *   Every return value is serialized to JSON automatically.
 *
 * @RequestMapping("/") groups all routes under a base path.
 * @Validated enables Bean Validation on @RequestParam fields.
 *
 * ─── Endpoints ────────────────────────────────────────────────────────────────
 *
 *  GET /calculate?appliance=Dryer&zip=85281
 *      → runs the full pipeline, returns OptimalWindow JSON + plain-text recommendation
 *
 *  GET /appliances
 *      → lists all appliances stored in H2 (useful for Postman exploration)
 *
 *  POST /appliances
 *      → adds a new appliance at runtime (body: {"name":"…","durationHours":…})
 *
 *  GET /health  (also available at /actuator/health)
 *      → simple liveness check
 */
@RestController
@Validated
@CrossOrigin(origins = "*")   // allow calls from any origin during development
public class ScheduleController {

    private final SchedulerService     schedulerService;
    private final ApplianceRepository  applianceRepo;

    public ScheduleController(SchedulerService schedulerService,
                               ApplianceRepository applianceRepo) {
        this.schedulerService = schedulerService;
        this.applianceRepo    = applianceRepo;
    }

    // ── Primary endpoint ──────────────────────────────────────────────────────

    /**
     * GET /calculate?appliance=Dryer&zip=85281
     *
     * Returns the full OptimalWindow object (JSON) so a mobile app or web UI
     * can display both the structured data and the human-readable recommendation.
     *
     * Example response:
     * {
     *   "startTime": "2025-06-01T02:00:00-07:00",
     *   "endTime":   "2025-06-01T03:00:00-07:00",
     *   "averageCarbonIntensity": 138.4,
     *   "applianceName": "Dryer",
     *   "recommendation": "Run your Dryer at 2:00 AM — avg. 138 gCO₂/kWh (done by 3:00 AM)."
     * }
     */
    @GetMapping("/calculate")
    public ResponseEntity<OptimalWindow> calculate(
            @RequestParam @NotBlank(message = "appliance name is required") String appliance,
            @RequestParam @NotBlank(message = "zip code is required")        String zip) {

        OptimalWindow result = schedulerService.calculate(appliance.trim(), zip.trim());
        return ResponseEntity.ok(result);
    }

    // ── Appliance CRUD ────────────────────────────────────────────────────────

    /** GET /appliances — list all appliances seeded (or added) in H2. */
    @GetMapping("/appliances")
    public ResponseEntity<List<Appliance>> listAppliances() {
        return ResponseEntity.ok(applianceRepo.findAll());
    }

    /**
     * POST /appliances — add a custom appliance at runtime.
     *
     * Body (JSON):
     * {
     *   "name": "Hot Tub",
     *   "durationHours": 2.0
     * }
     */
    @PostMapping("/appliances")
    public ResponseEntity<Appliance> addAppliance(@RequestBody Appliance appliance) {
        Appliance saved = applianceRepo.save(appliance);
        return ResponseEntity.status(201).body(saved);
    }

    /** DELETE /appliances/{id} — remove an appliance by its database ID. */
    @DeleteMapping("/appliances/{id}")
    public ResponseEntity<Void> deleteAppliance(@PathVariable Long id) {
        if (!applianceRepo.existsById(id)) {
            return ResponseEntity.notFound().build();
        }
        applianceRepo.deleteById(id);
        return ResponseEntity.noContent().build();
    }

    // ── Utility ───────────────────────────────────────────────────────────────

    /** GET /health — quick sanity check without using Actuator. */
    @GetMapping("/health")
    public ResponseEntity<Map<String, String>> health() {
        return ResponseEntity.ok(Map.of("status", "UP", "service", "GreenCycle"));
    }
}
