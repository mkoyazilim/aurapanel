# AuraPanel Marketing Site

This folder contains the standalone public website for `aurapanel.info` / `www.aurapanel.info`.

## Community Intake Backend

Community form submissions are handled by:

- `api/community_join.php`

Required server-side environment file:

- `/etc/aurapanel/community-site.env`

Reference template:

- `api/community-site.env.example`

## Database

SQL schema:

- `sql/community_signups.sql`

The endpoint auto-creates `community_signups` table if the DB user has required privileges.
