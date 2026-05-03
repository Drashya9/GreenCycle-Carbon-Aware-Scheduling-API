package com.greencycle;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * GreenCycleApplication — entry point for the Spring Boot process.
 *
 * @SpringBootApplication is a meta-annotation that combines:
 *   @Configuration      — this class can declare @Bean methods
 *   @EnableAutoConfiguration — let Spring Boot guess and wire dependencies
 *   @ComponentScan      — scan this package (and sub-packages) for components
 */
@SpringBootApplication
public class GreenCycleApplication {

    public static void main(String[] args) {
        SpringApplication.run(GreenCycleApplication.class, args);
    }
}
