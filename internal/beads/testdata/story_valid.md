# Story 1.2: Database Schema

Status: ready-for-dev

## Story

As a developer,
I want a well-defined database schema,
so that data is stored consistently.

## Acceptance Criteria

1. **Given** a new installation
   **When** migrations run
   **Then** all tables are created

2. **Given** an existing user record
   **When** queried by email
   **Then** the correct user is returned

## Tasks / Subtasks

- [ ] Task 1: Create migration files
- [ ] Task 2: Add seed data

## Dev Notes

Use PostgreSQL with standard naming conventions.
