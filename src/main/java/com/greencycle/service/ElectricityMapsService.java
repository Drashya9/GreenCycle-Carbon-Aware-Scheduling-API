package com.greencycle.service;

import com.greencycle.model.GridDataPoint;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;
import org.springframework.http.*;

import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.util.*;

/**
 * ElectricityMapsService — fetches live carbon forecasts from
 * https://api.electricitymap.org/v3/carbon-intensity/forecast
 *
 * Activated when:  grid.provider=electricitymaps
 *
 * ─── How to get an API key ───────────────────────────────────────────────────
 * 1. Go to https://www.electricitymaps.com/
 * 2. Sign up for a free or commercial plan.
 * 3. Copy your key and paste it into application.properties:
 *      grid.electricitymaps.api-key=YOUR_KEY_HERE
 *      grid.provider=electricitymaps
 * ─────────────────────────────────────────────────────────────────────────────
 *
 * ZIP-to-zone mapping:
 *   Electricity Maps uses grid zone codes (e.g. "US-CAL-CISO" for California).
 *   In a production app you'd call a geocoding API to convert ZIP → lat/lon → zone.
 *   Here we use a small static lookup table covering the most populous US states.
 *   Extend zipToZone() for additional states as needed.
 */
@Service
@ConditionalOnProperty(name = "grid.provider", havingValue = "electricitymaps")
public class ElectricityMapsService implements GridService {

    @Value("${grid.electricitymaps.api-key}")
    private String apiKey;

    @Value("${grid.electricitymaps.base-url:https://api.electricitymap.org/v3}")
    private String baseUrl;

    private final RestTemplate restTemplate = new RestTemplate();

    // ── Minimal ZIP-prefix → Electricity Maps zone mapping ──────────────────
    // Key: first 3 digits of ZIP code.  Extend this map as needed.
    private static final Map<String, String> ZIP_TO_ZONE = new HashMap<>();
    static {
        // California
        ZIP_TO_ZONE.put("900", "US-CAL-CISO"); ZIP_TO_ZONE.put("901", "US-CAL-CISO");
        ZIP_TO_ZONE.put("902", "US-CAL-CISO"); ZIP_TO_ZONE.put("940", "US-CAL-CISO");
        ZIP_TO_ZONE.put("941", "US-CAL-CISO"); ZIP_TO_ZONE.put("945", "US-CAL-CISO");
        // Arizona / Southwest
        ZIP_TO_ZONE.put("852", "US-SW-PNM");   ZIP_TO_ZONE.put("853", "US-SW-PNM");
        // Texas
        ZIP_TO_ZONE.put("750", "US-TEX-ERCOT"); ZIP_TO_ZONE.put("770", "US-TEX-ERCOT");
        // New York
        ZIP_TO_ZONE.put("100", "US-NY-NYIS");   ZIP_TO_ZONE.put("110", "US-NY-NYIS");
        // Florida
        ZIP_TO_ZONE.put("331", "US-FLA-FPL");   ZIP_TO_ZONE.put("332", "US-FLA-FPL");
        // Illinois
        ZIP_TO_ZONE.put("606", "US-MIDW-AMMO"); ZIP_TO_ZONE.put("607", "US-MIDW-AMMO");
        // Default fallback
        ZIP_TO_ZONE.put("000", "US-CAL-CISO");
    }

    @Override
    @SuppressWarnings("unchecked")
    public List<GridDataPoint> getForecast(String zipCode) {
        String zone = zipToZone(zipCode);
        String url  = baseUrl + "/carbon-intensity/forecast?zone=" + zone;

        HttpHeaders headers = new HttpHeaders();
        headers.set("auth-token", apiKey);
        headers.setAccept(List.of(MediaType.APPLICATION_JSON));

        ResponseEntity<Map> response = restTemplate.exchange(
                url, HttpMethod.GET, new HttpEntity<>(headers), Map.class);

        List<Map<String, Object>> rawForecast =
                (List<Map<String, Object>>) response.getBody().get("forecast");

        List<GridDataPoint> result = new ArrayList<>();
        // API returns 24 hourly slots; we take up to 24
        for (int i = 0; i < Math.min(24, rawForecast.size()); i++) {
            Map<String, Object> slot = rawForecast.get(i);
            ZonedDateTime ts = ZonedDateTime.parse(
                    (String) slot.get("datetime"), DateTimeFormatter.ISO_OFFSET_DATE_TIME);
            double ci = ((Number) slot.get("carbonIntensity")).doubleValue();
            result.add(new GridDataPoint(ts, ci));
        }
        return result;
    }

    /** Convert a ZIP code to the nearest Electricity Maps grid zone. */
    private String zipToZone(String zip) {
        if (zip == null || zip.length() < 3) return "US-CAL-CISO";
        String prefix = zip.substring(0, 3);
        return ZIP_TO_ZONE.getOrDefault(prefix, "US-CAL-CISO");
    }
}
