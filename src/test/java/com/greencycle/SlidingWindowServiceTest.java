package com.greencycle;

import com.greencycle.model.GridDataPoint;
import com.greencycle.model.OptimalWindow;
import com.greencycle.service.SlidingWindowService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.time.ZonedDateTime;
import java.util.ArrayList;
import java.util.List;

import static org.assertj.core.api.Assertions.*;

/**
 * SlidingWindowServiceTest — unit tests for the core algorithm.
 *
 * These tests run in milliseconds because they don't start a Spring context,
 * hit a database, or call any external API.
 *
 * Test cases:
 *   1. Cheapest window is at the start of the forecast
 *   2. Cheapest window is in the middle
 *   3. Cheapest window is at the end
 *   4. All slots are equal → window 0 wins (first encountered)
 *   5. Window larger than forecast → IllegalArgumentException
 *   6. Fractional duration (1.5 h) → rounds up to 2 slots
 */
class SlidingWindowServiceTest {

    private SlidingWindowService service;

    @BeforeEach
    void setUp() {
        service = new SlidingWindowService();
    }

    /** Build a minimal 24-slot forecast from a varargs array of intensities. */
    private List<GridDataPoint> buildForecast(double... intensities) {
        List<GridDataPoint> forecast = new ArrayList<>();
        ZonedDateTime base = ZonedDateTime.now().withMinute(0).withSecond(0).withNano(0);
        for (int i = 0; i < intensities.length; i++) {
            forecast.add(new GridDataPoint(base.plusHours(i), intensities[i]));
        }
        return forecast;
    }

    // ─────────────────────────────────────────────────────────────────────────

    @Test
    @DisplayName("Optimal window at start of forecast")
    void testOptimalAtStart() {
        // Slots: [50, 60, 300, 300, 300, …]  window=2 → best is [50,60]
        List<GridDataPoint> forecast = buildForecast(
            50, 60, 300, 300, 300, 300, 300, 300,
            300, 300, 300, 300, 300, 300, 300, 300,
            300, 300, 300, 300, 300, 300, 300, 300
        );
        OptimalWindow result = service.findOptimalWindow(forecast, 2.0, "Washer");

        assertThat(result.getStartTime()).isEqualTo(forecast.get(0).getTimestamp());
        assertThat(result.getAverageCarbonIntensity()).isEqualTo(55.0);
    }

    @Test
    @DisplayName("Optimal window in the middle of forecast")
    void testOptimalInMiddle() {
        // Slot 10-12 is cheapest
        double[] intensities = new double[24];
        for (int i = 0; i < 24; i++) intensities[i] = 300;
        intensities[10] = 80;
        intensities[11] = 70;
        intensities[12] = 90;

        List<GridDataPoint> forecast = buildForecast(intensities);
        OptimalWindow result = service.findOptimalWindow(forecast, 3.0, "Dryer");

        assertThat(result.getStartTime()).isEqualTo(forecast.get(10).getTimestamp());
    }

    @Test
    @DisplayName("Optimal window at end of forecast")
    void testOptimalAtEnd() {
        double[] intensities = new double[24];
        for (int i = 0; i < 22; i++) intensities[i] = 400;
        intensities[22] = 100;
        intensities[23] = 110;

        List<GridDataPoint> forecast = buildForecast(intensities);
        OptimalWindow result = service.findOptimalWindow(forecast, 2.0, "Dishwasher");

        assertThat(result.getStartTime()).isEqualTo(forecast.get(22).getTimestamp());
    }

    @Test
    @DisplayName("All equal intensities — window 0 is returned")
    void testAllEqual() {
        double[] intensities = new double[24];
        for (int i = 0; i < 24; i++) intensities[i] = 200;

        List<GridDataPoint> forecast = buildForecast(intensities);
        OptimalWindow result = service.findOptimalWindow(forecast, 3.0, "Pool Pump");

        // First window wins when all are equal
        assertThat(result.getStartTime()).isEqualTo(forecast.get(0).getTimestamp());
        assertThat(result.getAverageCarbonIntensity()).isEqualTo(200.0);
    }

    @Test
    @DisplayName("Window larger than forecast throws IllegalArgumentException")
    void testWindowTooLarge() {
        List<GridDataPoint> forecast = buildForecast(100, 200, 300); // only 3 slots

        assertThatThrownBy(() -> service.findOptimalWindow(forecast, 5.0, "EV Charger"))
            .isInstanceOf(IllegalArgumentException.class)
            .hasMessageContaining("needs");
    }

    @Test
    @DisplayName("Fractional duration 1.5h rounds up to 2 slots")
    void testFractionalDuration() {
        double[] intensities = new double[24];
        for (int i = 0; i < 24; i++) intensities[i] = 300;
        intensities[5] = 50;
        intensities[6] = 60;

        List<GridDataPoint> forecast = buildForecast(intensities);
        // 1.5 hours → ceil → 2 slots; cheapest 2-slot window is [50,60] at idx 5
        OptimalWindow result = service.findOptimalWindow(forecast, 1.5, "Water Heater");

        assertThat(result.getStartTime()).isEqualTo(forecast.get(5).getTimestamp());
        assertThat(result.getAverageCarbonIntensity()).isEqualTo(55.0);
    }

    @Test
    @DisplayName("Recommendation string contains appliance name")
    void testRecommendationContainsName() {
        List<GridDataPoint> forecast = buildForecast(
            100, 200, 300, 100, 200, 300, 100, 200,
            300, 100, 200, 300, 100, 200, 300, 100,
            200, 300, 100, 200, 300, 100, 200, 300
        );
        OptimalWindow result = service.findOptimalWindow(forecast, 1.0, "Fancy Toaster");

        assertThat(result.getRecommendation()).contains("Fancy Toaster");
    }
}
