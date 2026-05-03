package com.greencycle.service;

import com.greencycle.model.GridDataPoint;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;

import java.time.ZonedDateTime;
import java.time.ZoneId;
import java.util.ArrayList;
import java.util.List;

/**
 * MockGridService — generates a realistic-looking 24-hour carbon forecast
 * without needing any API key.
 *
 * @ConditionalOnProperty activates this bean ONLY when:
 *     grid.provider=mock   (set in application.properties)
 *
 * The simulated pattern mimics a real US grid day:
 *   • Night  (0–5 AM)  : low intensity  — low demand, steady wind
 *   • Morning(6–9 AM)  : rising          — demand spikes before solar peaks
 *   • Midday(10–14)    : dips            — solar generation at maximum
 *   • Evening(15–21)   : peaks           — solar fades, gas plants ramp up
 *   • Late night(22–23): falling         — demand drops off
 *
 * Random jitter (±30) is added so each call returns a slightly different
 * forecast, making development testing more realistic.
 */
@Service
@ConditionalOnProperty(name = "grid.provider", havingValue = "mock", matchIfMissing = true)
public class MockGridService implements GridService {

    // Base carbon intensities (gCO₂eq/kWh) for each hour of the day (index 0 = midnight)
    private static final double[] HOURLY_BASE = {
        180, 160, 145, 135, 130, 140,   // 0–5 AM   (night / early morning)
        180, 220, 260, 280, 300, 310,   // 6–11 AM  (morning ramp)
        280, 240, 210, 230, 270, 320,   // 12–17 PM (solar dip then peak demand)
        380, 400, 370, 320, 260, 210    // 18–23 PM (evening peak then decline)
    };

    @Override
    public List<GridDataPoint> getForecast(String zipCode) {
        List<GridDataPoint> forecast = new ArrayList<>();
        ZonedDateTime now = ZonedDateTime.now(ZoneId.systemDefault())
                .withMinute(0)
                .withSecond(0)
                .withNano(0);

        for (int i = 0; i < 24; i++) {
            ZonedDateTime slotTime = now.plusHours(i);
            int hourOfDay = slotTime.getHour();

            // Apply jitter to make each forecast unique
            double jitter = (Math.random() - 0.5) * 60;
            double intensity = Math.max(50, HOURLY_BASE[hourOfDay] + jitter);

            forecast.add(new GridDataPoint(slotTime, Math.round(intensity * 10.0) / 10.0));
        }

        return forecast;
    }
}
