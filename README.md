# 🌿 GreenCycle — Smart Appliance Scheduler

Run your dishwasher when the grid is greenest.

GreenCycle sits between your user and the power grid. It fetches a 24-hour
carbon intensity forecast for your region, runs a sliding-window algorithm
over that data, and tells you the single best hour to start your appliance.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Java | 17 or 21 | https://adoptium.net |
| Maven | 3.9+ | https://maven.apache.org/download.cgi or bundled via `./mvnw` |
| Postman | any | https://www.postman.com/downloads/ |

Check your Java version:
```bash
java -version
# should print: openjdk version "17.x.x" or "21.x.x"
```

---

## Project Structure

```
green-cycle/
├── pom.xml                                         ← Maven build file (all dependencies)
├── README.md
└── src/
    ├── main/
    │   ├── java/com/greencycle/
    │   │   ├── GreenCycleApplication.java          ← Entry point (@SpringBootApplication)
    │   │   ├── model/
    │   │   │   ├── Appliance.java                  ← JPA entity (maps to H2 table)
    │   │   │   ├── GridDataPoint.java              ← One hourly carbon reading (POJO)
    │   │   │   └── OptimalWindow.java              ← Algorithm result returned to user
    │   │   ├── repository/
    │   │   │   └── ApplianceRepository.java        ← Spring Data JPA — zero SQL needed
    │   │   ├── service/
    │   │   │   ├── GridService.java                ← Interface (mock OR real API)
    │   │   │   ├── MockGridService.java            ← Synthetic forecast, no API key
    │   │   │   ├── ElectricityMapsService.java     ← Real data (opt-in via properties)
    │   │   │   ├── SlidingWindowService.java       ← The O(n) algorithm
    │   │   │   └── SchedulerService.java           ← Orchestrates the three steps
    │   │   ├── controller/
    │   │   │   └── ScheduleController.java         ← REST endpoints
    │   │   └── config/
    │   │       └── GlobalExceptionHandler.java     ← Converts exceptions → JSON errors
    │   └── resources/
    │       ├── application.properties              ← All configuration lives here
    │       └── data.sql                            ← Seeds H2 with 6 appliances on start
    └── test/
        └── java/com/greencycle/
            ├── SlidingWindowServiceTest.java       ← Pure unit tests (fast, no Spring)
            └── ScheduleControllerIntegrationTest.java ← Full-stack tests via MockMvc
```

---

## Running the Application

### Option A — Maven Wrapper (recommended, no Maven install needed)

```bash
cd green-cycle
./mvnw spring-boot:run          # macOS / Linux
mvnw.cmd spring-boot:run        # Windows
```

### Option B — Standard Maven

```bash
cd green-cycle
mvn spring-boot:run
```

### Option C — Build a JAR and run it

```bash
mvn clean package -DskipTests
java -jar target/green-cycle-0.0.1-SNAPSHOT.jar
```

You should see output ending with:
```
Started GreenCycleApplication in 2.3 seconds (JVM running for 2.8)
```

---

## Postman Testing Guide

Import the collection file `GreenCycle.postman_collection.json` into Postman,
or create requests manually using the table below.

### Endpoints

#### 1. Calculate Optimal Time (the main feature)

```
GET http://localhost:8080/calculate?appliance=Dishwasher&zip=85281
```

**Success response (200 OK):**
```json
{
  "startTime": "2025-06-01T02:00:00-07:00",
  "endTime":   "2025-06-01T03:30:00-07:00",
  "averageCarbonIntensity": 138.4,
  "applianceName": "Dishwasher",
  "recommendation": "Run your Dishwasher at 2:00 AM — avg. 138 gCO₂/kWh (done by 3:30 AM)."
}
```

Try these appliance names (all pre-seeded):
- `Dishwasher`
- `Washing Machine`
- `Dryer`
- `EV Charger`
- `Pool Pump`
- `Water Heater`

**Error response (400 Bad Request):**
```json
{
  "timestamp": "2025-06-01T10:00:00Z",
  "status": 400,
  "error": "Bad Request",
  "message": "Appliance not found: 'Toaster'. Call GET /appliances to see available options."
}
```

---

#### 2. List All Appliances

```
GET http://localhost:8080/appliances
```

```json
[
  {"id": 1, "name": "Dishwasher",     "durationHours": 1.5},
  {"id": 2, "name": "Washing Machine","durationHours": 1.0},
  {"id": 3, "name": "Dryer",          "durationHours": 1.0},
  {"id": 4, "name": "EV Charger",     "durationHours": 8.0},
  {"id": 5, "name": "Pool Pump",      "durationHours": 4.0},
  {"id": 6, "name": "Water Heater",   "durationHours": 2.0}
]
```

---

#### 3. Add a Custom Appliance

```
POST http://localhost:8080/appliances
Content-Type: application/json

{
  "name": "Hot Tub",
  "durationHours": 3.0
}
```

Returns `201 Created` with the saved appliance including its generated `id`.

---

#### 4. Delete an Appliance

```
DELETE http://localhost:8080/appliances/{id}
```

Returns `204 No Content` on success.

---

#### 5. Health Check

```
GET http://localhost:8080/health
```

```json
{ "status": "UP", "service": "GreenCycle" }
```

---

#### 6. H2 Database Browser Console

Navigate to **http://localhost:8080/h2-console** while the app is running.

| Field        | Value                    |
|--------------|--------------------------|
| JDBC URL     | `jdbc:h2:mem:greencycle` |
| User Name    | `sa`                     |
| Password     | *(leave blank)*          |

Then run SQL directly:
```sql
SELECT * FROM APPLIANCE;
```

---

## Switching to a Real Grid API

### Option 1: Electricity Maps

1. Sign up at https://www.electricitymaps.com/
2. Get your API key
3. Edit `src/main/resources/application.properties`:

```properties
grid.provider=electricitymaps
grid.electricitymaps.api-key=YOUR_KEY_HERE
```

### Option 2: WattTime

1. Register at https://www.watttime.org/
2. Get your username + password
3. Edit `application.properties`:

```properties
grid.provider=watttime
grid.watttime.username=YOUR_USERNAME
grid.watttime.password=YOUR_PASSWORD
```

---

## Running Tests

```bash
# Run all tests (unit + integration)
mvn test

# Run only unit tests (faster, no Spring context)
mvn test -Dtest=SlidingWindowServiceTest

# Run only integration tests
mvn test -Dtest=ScheduleControllerIntegrationTest
```

Expected output:
```
Tests run: 13, Failures: 0, Errors: 0, Skipped: 0
BUILD SUCCESS
```

---

## How the Algorithm Works

```
Forecast (24 hours):  [180, 160, 145, 135, 130, 140, 180, 220, ...]
Appliance duration:   3 hours  → window size = 3 slots

Iteration 1:  sum([180, 160, 145]) = 485   ← new minimum
Iteration 2:  sum([160, 145, 135]) = 440   ← new minimum  (slide right)
Iteration 3:  sum([145, 135, 130]) = 410   ← new minimum
Iteration 4:  sum([135, 130, 140]) = 405   ← new minimum
Iteration 5:  sum([130, 140, 180]) = 450   ← not better, discard
...

Winner: index 3, starting at Hour 4 (11 PM previous night)
Average: 405 / 3 = 135 gCO₂/kWh
```

Time complexity: **O(n)** — one pass through the forecast.
Each slot is visited exactly twice (once entering, once leaving the window).

---

## Technology Decisions

| Choice | Why |
|--------|-----|
| Spring Boot | Zero-config embedded server, auto-wires everything |
| H2 In-Memory | No installation, wiped on restart — perfect for dev |
| `data.sql` | Declarative seed data — easy to extend |
| `GridService` interface | Swap mock → real API by changing one property |
| `@ConditionalOnProperty` | Only one GridService bean is created at a time |
| Constructor injection | Testable without Spring, no `@Autowired` magic |
| `@RestControllerAdvice` | Consistent JSON errors instead of HTML pages |
