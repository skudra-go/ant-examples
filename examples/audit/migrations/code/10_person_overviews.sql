-- Repeatable: this reruns whenever the file changes. One row per person, across
-- every month that still exists.
DROP VIEW IF EXISTS person_overviews;

CREATE VIEW person_overviews AS
SELECT p.ref,
       count(*) AS events,
       count(DISTINCT e.actor_id) AS actors,
       min(p.occurred_at) AS first_seen,
       max(p.occurred_at) AS last_seen
FROM event_persons p
JOIN events e ON e.id = p.event_id AND e.occurred_at = p.occurred_at
GROUP BY p.ref;
