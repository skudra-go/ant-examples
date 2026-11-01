-- Repeatable: this runs after the versioned migrations, and again whenever this
-- file changes. A view can be recreated at will, so edit it in place, and its
-- history stays in git, not in a trail of migrations.
DROP VIEW IF EXISTS post_stats;

CREATE VIEW post_stats AS
SELECT t.name AS tag,
       count(DISTINCT p.id) AS posts,
       count(c.id) AS comments
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id = t.id
LEFT JOIN posts p ON p.id = pt.post_id AND p.deleted_at IS NULL AND p.published_at IS NOT NULL
LEFT JOIN comments c ON c.post_id = p.id
GROUP BY t.name;
