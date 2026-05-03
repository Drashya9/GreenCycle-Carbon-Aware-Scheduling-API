package com.greencycle;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.web.servlet.MockMvc;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;
import static org.hamcrest.Matchers.*;

/**
 * ScheduleControllerIntegrationTest — boots the full Spring context and
 * exercises the REST endpoints through MockMvc (no real HTTP port needed).
 *
 * Spring Boot wires:
 *   - H2 in-memory database (populated by data.sql)
 *   - MockGridService (grid.provider=mock)
 *   - All controllers, services, and repositories
 *
 * @SpringBootTest(webEnvironment = MOCK) starts the app but intercepts HTTP
 *   calls in-process rather than binding a real TCP port.
 * @AutoConfigureMockMvc injects MockMvc automatically.
 */
@SpringBootTest
@AutoConfigureMockMvc
class ScheduleControllerIntegrationTest {

    @Autowired
    private MockMvc mockMvc;

    // ── /calculate ────────────────────────────────────────────────────────────

    @Test
    @DisplayName("GET /calculate with valid params returns 200 + recommendation")
    void calculateReturnsOk() throws Exception {
        mockMvc.perform(get("/calculate")
                .param("appliance", "Dishwasher")
                .param("zip",       "85281"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.recommendation").exists())
            .andExpect(jsonPath("$.applianceName").value("Dishwasher"))
            .andExpect(jsonPath("$.averageCarbonIntensity").isNumber())
            .andExpect(jsonPath("$.startTime").exists());
    }

    @Test
    @DisplayName("GET /calculate is case-insensitive for appliance name")
    void calculateCaseInsensitive() throws Exception {
        mockMvc.perform(get("/calculate")
                .param("appliance", "dryer")
                .param("zip",       "90210"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.applianceName").value("Dryer"));
    }

    @Test
    @DisplayName("GET /calculate with unknown appliance returns 400")
    void calculateUnknownApplianceReturns400() throws Exception {
        mockMvc.perform(get("/calculate")
                .param("appliance", "Time Machine")
                .param("zip",       "85281"))
            .andExpect(status().isBadRequest())
            .andExpect(jsonPath("$.message").value(containsString("not found")));
    }

    @Test
    @DisplayName("GET /calculate missing zip returns 400")
    void calculateMissingZipReturns400() throws Exception {
        mockMvc.perform(get("/calculate")
                .param("appliance", "Dryer"))
            .andExpect(status().isBadRequest());
    }

    // ── /appliances ───────────────────────────────────────────────────────────

    @Test
    @DisplayName("GET /appliances returns seeded list")
    void listAppliancesReturnsSeededData() throws Exception {
        mockMvc.perform(get("/appliances"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$", hasSize(greaterThanOrEqualTo(6))))
            .andExpect(jsonPath("$[*].name", hasItem("Dishwasher")));
    }

    @Test
    @DisplayName("POST /appliances creates a new appliance")
    void addApplianceCreates201() throws Exception {
        String body = "{\"name\":\"Hot Tub\",\"durationHours\":3.0}";

        mockMvc.perform(post("/appliances")
                .contentType("application/json")
                .content(body))
            .andExpect(status().isCreated())
            .andExpect(jsonPath("$.name").value("Hot Tub"))
            .andExpect(jsonPath("$.durationHours").value(3.0))
            .andExpect(jsonPath("$.id").isNumber());
    }

    // ── /health ───────────────────────────────────────────────────────────────

    @Test
    @DisplayName("GET /health returns UP")
    void healthReturnsUp() throws Exception {
        mockMvc.perform(get("/health"))
            .andExpect(status().isOk())
            .andExpect(jsonPath("$.status").value("UP"));
    }
}
