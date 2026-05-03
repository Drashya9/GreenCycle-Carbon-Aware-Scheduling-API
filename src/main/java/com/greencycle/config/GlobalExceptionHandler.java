package com.greencycle.config;

import jakarta.validation.ConstraintViolationException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import java.time.ZonedDateTime;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * GlobalExceptionHandler — converts exceptions into consistent JSON error bodies.
 *
 * Without this, Spring returns a generic HTML "Whitelabel Error Page" which is
 * useless when testing with Postman.  With this, every error looks like:
 *
 * {
 *   "timestamp": "2025-06-01T10:30:00Z",
 *   "status": 400,
 *   "error": "Bad Request",
 *   "message": "Appliance not found: 'Toaster'. Call GET /appliances …"
 * }
 *
 * @RestControllerAdvice intercepts exceptions thrown from any @RestController.
 */
@RestControllerAdvice
public class GlobalExceptionHandler {

    /** Handles bad appliance names, window-too-large, etc. */
    @ExceptionHandler(IllegalArgumentException.class)
    public ResponseEntity<Map<String, Object>> handleIllegalArgument(IllegalArgumentException ex) {
        return errorBody(HttpStatus.BAD_REQUEST, ex.getMessage());
    }

    /** Handles @NotBlank violations on @RequestParam fields. */
    @ExceptionHandler(ConstraintViolationException.class)
    public ResponseEntity<Map<String, Object>> handleValidation(ConstraintViolationException ex) {
        return errorBody(HttpStatus.BAD_REQUEST, ex.getMessage());
    }

    /** Catch-all for unexpected errors. */
    @ExceptionHandler(Exception.class)
    public ResponseEntity<Map<String, Object>> handleGeneric(Exception ex) {
        return errorBody(HttpStatus.INTERNAL_SERVER_ERROR,
            "Unexpected error: " + ex.getMessage());
    }

    // ── Helper ────────────────────────────────────────────────────────────────

    private ResponseEntity<Map<String, Object>> errorBody(HttpStatus status, String message) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("timestamp", ZonedDateTime.now().toString());
        body.put("status",    status.value());
        body.put("error",     status.getReasonPhrase());
        body.put("message",   message);
        return ResponseEntity.status(status).body(body);
    }
}
